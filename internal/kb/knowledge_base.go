package kb

import (
	"bytes"
	"container/list"
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"knowledge/internal/db"
	"knowledge/internal/llm"

	"github.com/ledongthuc/pdf"
	"github.com/lu4p/cat"
	"github.com/xuri/excelize/v2"
)

type embeddingCacheEntry struct {
	key string
	val []byte
	exp time.Time
}

type embeddingLRUCache struct {
	mu  sync.Mutex
	cap int
	ttl time.Duration
	ll  *list.List
	m   map[string]*list.Element
}

func newEmbeddingLRUCache(capacity int, ttl time.Duration) *embeddingLRUCache {
	if capacity <= 0 {
		capacity = 2048
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &embeddingLRUCache{
		cap: capacity,
		ttl: ttl,
		ll:  list.New(),
		m:   make(map[string]*list.Element, capacity),
	}
}

func (c *embeddingLRUCache) Get(key string) ([]byte, bool) {
	if key == "" {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.m[key]; ok {
		ent := el.Value.(*embeddingCacheEntry)
		if !ent.exp.IsZero() && time.Now().After(ent.exp) {
			c.ll.Remove(el)
			delete(c.m, key)
			return nil, false
		}
		c.ll.MoveToFront(el)
		return ent.val, true
	}
	return nil, false
}

func (c *embeddingLRUCache) Set(key string, val []byte) {
	if key == "" || len(val) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.m[key]; ok {
		ent := el.Value.(*embeddingCacheEntry)
		ent.val = val
		ent.exp = time.Now().Add(c.ttl)
		c.ll.MoveToFront(el)
		return
	}

	ent := &embeddingCacheEntry{key: key, val: val, exp: time.Now().Add(c.ttl)}
	el := c.ll.PushFront(ent)
	c.m[key] = el

	for c.ll.Len() > c.cap {
		back := c.ll.Back()
		if back == nil {
			break
		}
		be := back.Value.(*embeddingCacheEntry)
		delete(c.m, be.key)
		c.ll.Remove(back)
	}
}

// 向量缓存 - 优化：减少重复内容的向量生成（有界 + TTL）
var embeddingCache = newEmbeddingLRUCache(2048, 10*time.Minute)

// ChunkProgress 文件分片进度信息
type ChunkProgress struct {
	FileName        string  `json:"file_name"`
	TotalChunks     int     `json:"total_chunks"`
	ProcessedChunks int     `json:"processed_chunks"`
	Progress        float64 `json:"progress"`
}

// SyncProgress 同步进度信息
type SyncProgress struct {
	TotalFiles     int             `json:"total_files"`
	ProcessedFiles int             `json:"processed_files"`
	CurrentFile    string          `json:"current_file"`
	Status         string          `json:"status"`
	Progress       float64         `json:"progress"`
	ChunkProgress  []ChunkProgress `json:"chunk_progress"`
}

type KnowledgeBase struct {
	mu         sync.Mutex
	progress   SyncProgress
	progressMu sync.Mutex
	isSyncing  bool
	syncMu     sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc

	paused atomic.Bool
}

func NewKnowledgeBase() *KnowledgeBase {
	ctx, cancel := context.WithCancel(context.Background())
	return &KnowledgeBase{ctx: ctx, cancel: cancel}
}

func (kb *KnowledgeBase) Close() {
	if kb.cancel != nil {
		kb.cancel()
	}
}

// GetSyncProgress 获取当前同步进度
func (kb *KnowledgeBase) GetSyncProgress() SyncProgress {
	kb.progressMu.Lock()
	defer kb.progressMu.Unlock()
	return kb.progress
}

// UpdateSyncProgress 更新同步进度
func (kb *KnowledgeBase) UpdateSyncProgress(progress SyncProgress) {
	kb.progressMu.Lock()
	defer kb.progressMu.Unlock()
	kb.progress = progress
}

// ResetSyncProgress 重置同步进度
func (kb *KnowledgeBase) ResetSyncProgress() {
	kb.progressMu.Lock()
	defer kb.progressMu.Unlock()
	kb.progress = SyncProgress{
		TotalFiles:     0,
		ProcessedFiles: 0,
		CurrentFile:    "",
		Status:         "idle",
		Progress:       0,
		ChunkProgress:  []ChunkProgress{},
	}
}

// PauseSync 暂停当前同步/处理
func (kb *KnowledgeBase) PauseSync() {
	kb.paused.Store(true)
}

// ResumeSync 恢复当前同步/处理
func (kb *KnowledgeBase) ResumeSync() {
	kb.paused.Store(false)
}

// CancelSync 停止当前同步/处理，并重置上下文和进度
func (kb *KnowledgeBase) CancelSync() {
	kb.syncMu.Lock()
	defer kb.syncMu.Unlock()

	if kb.cancel != nil {
		kb.cancel()
	}

	// 为后续新的同步流程创建新的上下文
	kb.ctx, kb.cancel = context.WithCancel(context.Background())
	kb.isSyncing = false
	kb.ResetSyncProgress()
	kb.paused.Store(false)
}

// waitIfPaused 在分片/扫描循环中调用，支持暂停与停止
func (kb *KnowledgeBase) waitIfPaused() error {
	for {
		if kb.ctx != nil {
			select {
			case <-kb.ctx.Done():
				return kb.ctx.Err()
			default:
			}
		}
		if !kb.paused.Load() {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// updateChunkProgress 更新（或追加）某个文件的分片进度，用于前端展示列表
func (kb *KnowledgeBase) updateChunkProgress(p ChunkProgress) {
	kb.progressMu.Lock()
	defer kb.progressMu.Unlock()

	found := false
	for i := range kb.progress.ChunkProgress {
		if kb.progress.ChunkProgress[i].FileName == p.FileName {
			kb.progress.ChunkProgress[i] = p
			found = true
			break
		}
	}
	if !found {
		kb.progress.ChunkProgress = append(kb.progress.ChunkProgress, p)
	}
}

// AddFile 添加单个文件到知识库并立即处理
func (kb *KnowledgeBase) AddFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory")
	}

	// 过滤以 .~ 开头的临时文件
	filename := filepath.Base(path)
	if strings.HasPrefix(filename, ".~") {
		return fmt.Errorf("temporary file not supported: %s", filename)
	}

	// 只处理文本相关文件
	ext := strings.ToLower(filepath.Ext(path))
	if !isSupportedExt(ext) {
		return fmt.Errorf("unsupported file extension: %s", ext)
	}

	checksum, err := calculateMD5(path)
	if err != nil {
		return err
	}

	// 存入数据库
	kbFile, err := db.SaveKBFile(path, info.Size(), checksum)
	if err != nil {
		return err
	}

	// 立即处理文件
	if err := kb.processFile(*kbFile); err != nil {
		_ = db.UpdateKBFileStatus(kbFile.ID, "error")
		return err
	}

	return db.UpdateKBFileStatus(kbFile.ID, "processed")
}

// ScanFolder 扫描文件夹并同步到数据库
func (kb *KnowledgeBase) ScanFolder() error {
	if kb.ctx != nil {
		select {
		case <-kb.ctx.Done():
			return kb.ctx.Err()
		default:
		}
	}
	// 检查是否正在同步中
	kb.syncMu.Lock()
	if kb.isSyncing {
		kb.syncMu.Unlock()
		return fmt.Errorf("sync already in progress")
	}
	kb.isSyncing = true
	kb.syncMu.Unlock()

	// 函数结束时重置同步状态
	defer func() {
		kb.syncMu.Lock()
		kb.isSyncing = false
		kb.syncMu.Unlock()
	}()

	folder, err := db.GetKBFolder()
	if err != nil || folder == "" {
		return fmt.Errorf("knowledge base folder not set")
	}

	// 重置进度
	kb.ResetSyncProgress()

	// 收集所有文件信息
	var files []struct {
		path     string
		info     os.FileInfo
		checksum string
	}

	// 第一遍：收集文件信息
	kb.UpdateSyncProgress(SyncProgress{
		TotalFiles:     0,
		ProcessedFiles: 0,
		CurrentFile:    "",
		Status:         "scanning",
		Progress:       0,
	})

	err = filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if kb.ctx != nil {
			select {
			case <-kb.ctx.Done():
				return kb.ctx.Err()
			default:
			}
		}
		if err := kb.waitIfPaused(); err != nil {
			return err
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// 过滤以 .~ 开头的临时文件
		filename := filepath.Base(path)
		if strings.HasPrefix(filename, ".~") {
			return nil
		}

		// 只处理文本相关文件
		ext := strings.ToLower(filepath.Ext(path))
		if !isSupportedExt(ext) {
			return nil
		}

		checksum, err := calculateMD5(path)
		if err != nil {
			return err
		}

		files = append(files, struct {
			path     string
			info     os.FileInfo
			checksum string
		}{path, info, checksum})

		// 更新进度
		kb.UpdateSyncProgress(SyncProgress{
			TotalFiles:     len(files),
			ProcessedFiles: 0,
			CurrentFile:    path,
			Status:         "scanning",
			Progress:       0,
		})

		return nil
	})

	if err != nil {
		return err
	}

	// 第二遍：批量处理数据库操作
	totalFiles := len(files)
	kb.UpdateSyncProgress(SyncProgress{
		TotalFiles:     totalFiles,
		ProcessedFiles: 0,
		CurrentFile:    "",
		Status:         "syncing",
		Progress:       0,
	})

	for i, file := range files {
		_, err = db.SaveKBFile(file.path, file.info.Size(), file.checksum)
		if err != nil {
			return err
		}

		// 更新进度
		progress := float64(i+1) / float64(totalFiles) * 100
		kb.UpdateSyncProgress(SyncProgress{
			TotalFiles:     totalFiles,
			ProcessedFiles: i + 1,
			CurrentFile:    file.path,
			Status:         "syncing",
			Progress:       progress,
		})
	}

	kb.UpdateSyncProgress(SyncProgress{
		TotalFiles:     totalFiles,
		ProcessedFiles: totalFiles,
		CurrentFile:    "",
		Status:         "scanned",
		Progress:       100,
	})

	return nil
}

// ProcessFiles 处理待处理的文件
func (kb *KnowledgeBase) ProcessFiles() error {
	if kb.ctx != nil {
		select {
		case <-kb.ctx.Done():
			return kb.ctx.Err()
		default:
		}
	}
	// 检查是否正在同步中
	kb.syncMu.Lock()
	if kb.isSyncing {
		kb.syncMu.Unlock()
		return fmt.Errorf("sync already in progress")
	}
	kb.isSyncing = true
	kb.syncMu.Unlock()

	// 函数结束时重置同步状态
	defer func() {
		kb.syncMu.Lock()
		kb.isSyncing = false
		kb.syncMu.Unlock()
	}()

	files, err := db.ListKBFiles()
	if err != nil {
		return err
	}

	// 过滤出待处理的文件
	var pendingFiles []db.KnowledgeBaseFile
	for _, f := range files {
		if f.Status == "pending" {
			pendingFiles = append(pendingFiles, f)
		}
	}

	if len(pendingFiles) == 0 {
		kb.UpdateSyncProgress(SyncProgress{
			TotalFiles:     0,
			ProcessedFiles: 0,
			CurrentFile:    "",
			Status:         "idle",
			Progress:       0,
		})
		return nil
	}

	totalFiles := len(pendingFiles)
	var processedCount atomic.Int64

	// 更新进度为处理开始
	kb.UpdateSyncProgress(SyncProgress{
		TotalFiles:     totalFiles,
		ProcessedFiles: 0,
		CurrentFile:    "",
		Status:         "processing",
		Progress:       0,
	})

	// 并发处理文件
	var wg sync.WaitGroup
	errChan := make(chan error, len(pendingFiles))
	progressChan := make(chan struct{}, len(pendingFiles))
	progressDone := make(chan struct{})

	// 控制并发数量：根据 CPU 核心数自适应，避免在多核机器上过于保守
	concurrencyLimit := runtime.NumCPU() * 2
	if concurrencyLimit < 4 {
		concurrencyLimit = 4
	}
	if concurrencyLimit > 16 {
		concurrencyLimit = 16
	}
	semaphore := make(chan struct{}, concurrencyLimit)

	for _, f := range pendingFiles {
		wg.Add(1)
		go func(file db.KnowledgeBaseFile) {
			defer wg.Done()

			if err := kb.waitIfPaused(); err != nil {
				errChan <- err
				return
			}

			if kb.ctx != nil {
				select {
				case <-kb.ctx.Done():
					errChan <- kb.ctx.Err()
					return
				default:
				}
			}

			// 申请信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 更新当前处理的文件
			cur := processedCount.Load()
			kb.UpdateSyncProgress(SyncProgress{
				TotalFiles:     totalFiles,
				ProcessedFiles: int(cur),
				CurrentFile:    file.Path,
				Status:         "processing",
				Progress:       float64(cur) / float64(totalFiles) * 100,
			})

			if err := kb.processFile(file); err != nil {
				fmt.Printf("Error processing file %s: %v\n", file.Path, err)
				_ = db.UpdateKBFileStatus(file.ID, "error")
				errChan <- err
				return
			}

			_ = db.UpdateKBFileStatus(file.ID, "processed")
			progressChan <- struct{}{}
		}(f)
	}

	// 处理进度更新
	go func() {
		defer close(progressDone)
		for range pendingFiles {
			<-progressChan
			cur := processedCount.Add(1)
			progress := float64(cur) / float64(totalFiles) * 100
			kb.UpdateSyncProgress(SyncProgress{
				TotalFiles:     totalFiles,
				ProcessedFiles: int(cur),
				CurrentFile:    "",
				Status:         "processing",
				Progress:       progress,
			})
		}
	}()

	// 等待所有处理完成
	wg.Wait()
	close(errChan)
	close(progressChan)
	<-progressDone

	// 更新进度为处理完成
	kb.UpdateSyncProgress(SyncProgress{
		TotalFiles:     totalFiles,
		ProcessedFiles: totalFiles,
		CurrentFile:    "",
		Status:         "completed",
		Progress:       100,
	})

	// 检查是否有错误
	for err := range errChan {
		if err != nil {
			// 只返回第一个错误
			return err
		}
	}

	return nil
}

// GetFileContent 获取文件内容
func (kb *KnowledgeBase) GetFileContent(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return extractTextFromPDF(path)
	case ".docx":
		return extractTextFromDocx(path)
	case ".xlsx", ".xls":
		return extractTextFromXlsx(path)
	case ".csv":
		return extractTextFromCsv(path)
	default:
		// 默认处理文本文件
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

// Float32SliceToBytes 将float32切片转换为字节数组
func Float32SliceToBytes(s []float32) []byte {
	b := make([]byte, len(s)*4)
	for i, v := range s {
		binary.LittleEndian.PutUint32(b[i*4:], uint32(math.Float32bits(v)))
	}
	return b
}

// BytesToFloat32Slice 将字节数组转换为float32切片
func BytesToFloat32Slice(b []byte) []float32 {
	s := make([]float32, len(b)/4)
	for i := range s {
		s[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return s
}

// CosineSimilarity 计算两个向量的余弦相似度
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct float32
	var normA float32
	var normB float32

	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// getEmbedding 获取文本的向量表示
func getEmbedding(text string) ([]byte, error) {
	// 计算文本哈希作为缓存键
	hash := md5.Sum([]byte(text))
	key := fmt.Sprintf("%x", hash)

	// 检查缓存
	if vector, ok := embeddingCache.Get(key); ok {
		return vector, nil
	}

	if llm.CurrentEngine == nil {
		return nil, fmt.Errorf("LLM engine not initialized")
	}

	embedding, err := llm.CurrentEngine.GetEmbedding(text)
	if err != nil {
		return nil, err
	}

	vector := Float32SliceToBytes(embedding)

	// 存入缓存
	embeddingCache.Set(key, vector)

	return vector, nil
}

func (kb *KnowledgeBase) processFile(f db.KnowledgeBaseFile) error {
	// 确保“写入向量时使用的 embedding 模型”和“后续查询的 embedding 模型”一致
	if llm.CurrentEngine != nil {
		currentModel := strings.TrimSpace(llm.CurrentEngine.GetModelPath())
		if currentModel != "" {
			existingModel, _ := db.GetKBEmbeddingModel()
			existingModel = strings.TrimSpace(existingModel)
			if existingModel == "" {
				_ = db.SetKBEmbeddingModel(currentModel)
			} else if existingModel != currentModel {
				return fmt.Errorf("embedding model changed (kb=%s, current=%s). please reset/rebuild knowledge base", existingModel, currentModel)
			}
		}
	}

	ext := strings.ToLower(filepath.Ext(f.Path))
	var (
		content     string
		indexChunks []string
		err         error
	)

	switch ext {
	case ".pdf":
		content, err = extractTextFromPDF(f.Path)
	case ".docx":
		content, err = extractTextFromDocx(f.Path)
	case ".xlsx", ".xls":
		// 预览仍使用 markdown 表格；索引/检索使用行级语义编码（更适合按编号/成绩查询）
		indexChunks, err = extractIndexChunksFromXlsx(f.Path)
	default:
		// 默认处理文本文件
		var b []byte
		b, err = os.ReadFile(f.Path)
		if err == nil {
			content = string(b)
		}
	}

	if err != nil {
		return err
	}

	// 优化：根据文件类型调整分片大小
	var chunks []string
	switch ext {
	case ".pdf":
		chunks = splitText(content, 1500, 250) // PDF 内容更适合较大分片
	case ".docx":
		chunks = splitText(content, 1200, 200)
	case ".xlsx", ".xls":
		chunks = indexChunks
	default:
		chunks = splitText(content, 1000, 150) // 普通文本适当减小重叠
	}

	// 过滤空切片
	var validChunks []string
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk) != "" {
			validChunks = append(validChunks, chunk)
		}
	}

	if len(validChunks) == 0 {
		return nil
	}

	// 创建文件分片进度对象
	fileName := filepath.Base(f.Path)
	totalChunks := len(validChunks)
	processedChunks := 0
	lastProgressUpdate := time.Now()
	fileInfo, _ := os.Stat(f.Path)
	fileSize := int64(0)
	if fileInfo != nil {
		fileSize = fileInfo.Size()
	}

	// 大文件导入加速：对超大 Excel 可跳过向量生成（仍保存文本，依赖编号/关键词检索；必要时查询阶段再按需生成向量）
	skipEmbedding := false
	if ext == ".xlsx" || ext == ".xls" {
		// Excel 的“按编号/字段检索”主要依赖文本命中；大量向量生成会极慢。
		// 因此对“中等规模以上”的 Excel 就跳过 embedding，以显著提升导入速度。
		// （查询阶段仍可对少量候选按需生成向量做精排）
		if totalChunks >= 200 || fileSize >= 3*1024*1024 || totalChunks >= 1200 || fileSize >= 15*1024*1024 {
			skipEmbedding = true
		}
	}

	// 更新同步进度，添加分片进度信息
	kb.updateChunkProgress(ChunkProgress{
		FileName:        fileName,
		TotalChunks:     totalChunks,
		ProcessedChunks: 0,
		Progress:        0,
	})

	// 单事务 + 批量写入：减少 SQLite 写锁争用，并避免中途失败导致数据不一致
	tx := db.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()

	if err := tx.Where("file_id = ?", f.ID).Delete(&db.KnowledgeBaseChunk{}).Error; err != nil {
		_ = tx.Rollback()
		return err
	}

	batchSize := 200
	if totalChunks >= 2000 {
		batchSize = 500
	}
	batch := make([]db.KnowledgeBaseChunk, 0, batchSize)

	for _, chunkContent := range validChunks {
		if err := kb.waitIfPaused(); err != nil {
			_ = tx.Rollback()
			return err
		}
		if kb.ctx != nil {
			select {
			case <-kb.ctx.Done():
				_ = tx.Rollback()
				return kb.ctx.Err()
			default:
			}
		}
		// 生成向量（失败则降级为空向量，仍保留 content 供文本检索）
		var vector []byte
		if !skipEmbedding {
			v, err := getEmbedding(chunkContent)
			if err != nil {
				fmt.Printf("Error getting embedding: %v\n", err)
				v = nil
			}
			vector = v
		}

		batch = append(batch, db.KnowledgeBaseChunk{
			FileID:  f.ID,
			Content: chunkContent,
			Vector:  vector,
		})

		processedChunks++
		// 进度更新节流：大文件分片时避免每个 chunk 都加锁刷新
		if processedChunks == totalChunks || processedChunks%25 == 0 || time.Since(lastProgressUpdate) >= 250*time.Millisecond {
			progress := float64(processedChunks) / float64(totalChunks) * 100
			kb.updateChunkProgress(ChunkProgress{
				FileName:        fileName,
				TotalChunks:     totalChunks,
				ProcessedChunks: processedChunks,
				Progress:        progress,
			})
			lastProgressUpdate = time.Now()
		}

		if len(batch) >= batchSize {
			if err := tx.CreateInBatches(batch, batchSize).Error; err != nil {
				_ = tx.Rollback()
				return err
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := tx.CreateInBatches(batch, batchSize).Error; err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}

func extractIndexChunksFromXlsx(path string) ([]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()

	// 最大处理行数：文件大小每增加1MB，增加1000行，最大不超过200000行
	maxProcessRows := int(fileSize/(1024*1024)) * 1000
	if maxProcessRows < 1000 {
		maxProcessRows = 1000
	}
	if maxProcessRows > 200000 {
		maxProcessRows = 200000
	}

	// chunk 越小需要算的 embedding 越多，会显著拖慢大文件导入。
	// 对大文件提高 chunk 字符数上限，减少 embedding 次数，但仍保证“按行/按记录”可检索。
	targetChunkChars := 5000
	if maxProcessRows >= 20000 || fileSize >= 20*1024*1024 {
		targetChunkChars = 12000
	} else if maxProcessRows >= 10000 || fileSize >= 10*1024*1024 {
		targetChunkChars = 8000
	}
	var chunks []string

	sheets := f.GetSheetList()
	for _, sheet := range sheets {
		rows, err := f.Rows(sheet)
		if err != nil {
			continue
		}

		var (
			headers []string
			rowNum  = 0
			sb      strings.Builder
		)
		var line strings.Builder

		flush := func() {
			s := strings.TrimSpace(sb.String())
			if s != "" {
				chunks = append(chunks, s)
			}
			sb.Reset()
		}

		for rows.Next() {
			rowNum++
			if rowNum > maxProcessRows {
				break
			}
			cells, err := rows.Columns()
			if err != nil {
				continue
			}
			if rowNum == 1 {
				// 表头行
				headers = make([]string, len(cells))
				for i := range cells {
					headers[i] = strings.TrimSpace(cells[i])
					if headers[i] == "" {
						headers[i] = fmt.Sprintf("列%d", i+1)
					}
				}
				continue
			}

			// 跳过空行
			empty := true
			for _, c := range cells {
				if strings.TrimSpace(c) != "" {
					empty = false
					break
				}
			}
			if empty {
				continue
			}

			// 行级语义编码：工作表 + 行号 + 列名:值
			line.Reset()
			line.WriteString("工作表: ")
			line.WriteString(sheet)
			line.WriteString("；行: ")
			line.WriteString(strconv.Itoa(rowNum))
			line.WriteString("；")

			limit := len(cells)
			if len(headers) < limit {
				limit = len(headers)
			}
			for i := 0; i < limit; i++ {
				v := strings.TrimSpace(cells[i])
				if v == "" {
					continue
				}
				line.WriteString(headers[i])
				line.WriteString(": ")
				line.WriteString(v)
				line.WriteString("；")
			}

			record := strings.TrimSpace(line.String())
			if record == "" {
				continue
			}

			// 控制 chunk 大小，保证记录不被截断
			if sb.Len() > 0 && sb.Len()+len(record)+1 > targetChunkChars {
				flush()
			}
			if sb.Len() == 0 {
				sb.WriteString("数据来源: Excel；文件: ")
				sb.WriteString(filepath.Base(path))
				sb.WriteString("\n")
			}
			sb.WriteString(record)
			sb.WriteString("\n")
		}

		flush()
	}

	return chunks, nil
}

func extractTextFromPDF(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	_, err = buf.ReadFrom(b)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func extractTextFromDocx(path string) (string, error) {
	return cat.File(path)
}

func extractTextFromXlsx(path string) (string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Printf("关闭 Excel 文件出错: %v\n", err)
		}
	}()

	var fullText strings.Builder
	sheets := f.GetSheetList()
	for _, sheet := range sheets {
		// 添加工作表名称
		fullText.WriteString("工作表: ")
		fullText.WriteString(sheet)
		fullText.WriteString("\n")

		// 获取工作表的最大行数和列数
		dimension, err := f.GetSheetDimension(sheet)
		if err != nil {
			continue
		}

		// 解析维度字符串，获取最大行号
		maxRow := 0
		if dimension != "" {
			parts := strings.Split(dimension, ":")
			if len(parts) == 2 {
				endCell := parts[1]
				rowStr := ""
				for _, char := range endCell {
					if unicode.IsDigit(char) {
						rowStr += string(char)
					}
				}
				if rowStr != "" {
					maxRow, _ = strconv.Atoi(rowStr)
				}
			}
		}

		// 根据文件大小动态调整处理行数
		fileInfo, err := os.Stat(path)
		if err != nil {
			continue
		}
		fileSize := fileInfo.Size()

		// 计算最大处理行数：文件大小每增加1MB，增加1000行，最大不超过200000行
		maxProcessRows := int(fileSize/(1024*1024)) * 1000
		if maxProcessRows < 1000 {
			maxProcessRows = 1000
		}
		if maxProcessRows > 200000 {
			maxProcessRows = 200000
		}

		// 如果工作表行数小于最大处理行数，则处理所有行
		if maxRow < maxProcessRows {
			maxProcessRows = maxRow
		}

		// 获取所有行数据
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}

		// 获取表头（第一行）
		var headers []string
		if len(rows) > 0 {
			headerCells := rows[0]
			headers = make([]string, len(headerCells))
			copy(headers, headerCells)

			// 写入markdown表格格式的表头
			fullText.WriteString("| ")
			for _, header := range headers {
				fullText.WriteString(header)
				fullText.WriteString(" | ")
			}
			fullText.WriteString("\n")

			// 写入表头分隔线
			fullText.WriteString("| ")
			for _, header := range headers {
				fullText.WriteString(strings.Repeat("-", len(header)))
				fullText.WriteString(" | ")
			}
			fullText.WriteString("\n")
		}

		// 逐行处理，避免一次性加载所有行
		startRow := 1 // 从第二行开始处理数据（索引为1）
		if len(rows) < startRow {
			startRow = 0
		}

		for row := startRow; row < len(rows) && row < maxProcessRows; row++ {
			cells := rows[row]

			// 跳过空行
			isEmptyRow := true
			for _, cell := range cells {
				if cell != "" {
					isEmptyRow = false
					break
				}
			}
			if isEmptyRow {
				continue
			}

			// 写入markdown表格格式的数据行
			fullText.WriteString("| ")
			for _, cell := range cells {
				fullText.WriteString(cell)
				fullText.WriteString(" | ")
			}
			fullText.WriteString("\n")
		}

		// 添加工作表分隔符
		fullText.WriteString("\n" + strings.Repeat("-", 80) + "\n\n")
	}
	return fullText.String(), nil
}

func extractTextFromCsv(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var fullText strings.Builder
	reader := csv.NewReader(f)

	// 根据文件大小动态调整处理行数
	fileInfo, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	fileSize := fileInfo.Size()

	// 计算最大处理行数：文件大小每增加1MB，增加1000行，最大不超过200000行
	maxProcessRows := int(fileSize/(1024*1024)) * 1000
	if maxProcessRows < 1000 {
		maxProcessRows = 1000
	}
	if maxProcessRows > 200000 {
		maxProcessRows = 200000
	}

	// 添加文件名称
	fullText.WriteString("CSV文件: ")
	fullText.WriteString(filepath.Base(path))
	fullText.WriteString("\n")

	// 获取表头（第一行）
	var headers []string
	headerRow, err := reader.Read()
	if err == nil {
		headers = make([]string, len(headerRow))
		copy(headers, headerRow)

		// 写入markdown表格格式的表头
		fullText.WriteString("| ")
		for _, header := range headers {
			fullText.WriteString(header)
			fullText.WriteString(" | ")
		}
		fullText.WriteString("\n")

		// 写入表头分隔线
		fullText.WriteString("| ")
		for _, header := range headers {
			fullText.WriteString(strings.Repeat("-", len(header)))
			fullText.WriteString(" | ")
		}
		fullText.WriteString("\n")
	}

	// 逐行处理，避免一次性加载所有行
	rowCount := 2 // 从第二行开始处理数据
	for rowCount <= maxProcessRows {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		// 跳过空行
		isEmptyRow := true
		for _, cell := range row {
			if cell != "" {
				isEmptyRow = false
				break
			}
		}
		if isEmptyRow {
			rowCount++
			continue
		}

		// 写入markdown表格格式的数据行
		fullText.WriteString("| ")
		for _, cell := range row {
			fullText.WriteString(cell)
			fullText.WriteString(" | ")
		}
		fullText.WriteString("\n")

		rowCount++
	}

	// 添加文件分隔符
	fullText.WriteString("\n" + strings.Repeat("-", 80) + "\n\n")

	return fullText.String(), nil
}

func isSupportedExt(ext string) bool {
	supported := []string{
		".txt", ".md", ".pdf", ".docx", ".xlsx", ".xls", ".csv",
	}
	return slices.Contains(supported, ext)
}

func calculateMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func splitText(text string, chunkSize int, overlap int) []string {
	if chunkSize <= 0 {
		return []string{text}
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize - 1
	}

	var chunks []string
	runes := []rune(text)
	n := len(runes)

	if n == 0 {
		return chunks
	}

	step := chunkSize - overlap
	if step <= 0 {
		step = 1
	}

	for i := 0; i < n; i += step {
		end := min(i+chunkSize, n)
		chunks = append(chunks, string(runes[i:end]))
		if end == n {
			break
		}
	}
	return chunks
}
