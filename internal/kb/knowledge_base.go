package kb

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"knowledge/internal/db"
	"knowledge/internal/llm"

	"github.com/ledongthuc/pdf"
	"github.com/lu4p/cat"
	"github.com/xuri/excelize/v2"
)

// 向量缓存 - 优化：减少重复内容的向量生成
var embeddingCache = make(map[string][]byte)
var cacheMutex sync.RWMutex

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
}

func NewKnowledgeBase() *KnowledgeBase {
	return &KnowledgeBase{}
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
	processedCount := 0

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
	progressChan := make(chan db.KnowledgeBaseFile, len(pendingFiles))

	// 控制并发数量
	concurrencyLimit := 5
	semaphore := make(chan struct{}, concurrencyLimit)

	for _, f := range pendingFiles {
		wg.Add(1)
		go func(file db.KnowledgeBaseFile) {
			defer wg.Done()

			// 申请信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 更新当前处理的文件
			kb.UpdateSyncProgress(SyncProgress{
				TotalFiles:     totalFiles,
				ProcessedFiles: processedCount,
				CurrentFile:    file.Path,
				Status:         "processing",
				Progress:       float64(processedCount) / float64(totalFiles) * 100,
			})

			if err := kb.processFile(file); err != nil {
				fmt.Printf("Error processing file %s: %v\n", file.Path, err)
				_ = db.UpdateKBFileStatus(file.ID, "error")
				errChan <- err
				return
			}

			_ = db.UpdateKBFileStatus(file.ID, "processed")
			progressChan <- file
		}(f)
	}

	// 处理进度更新
	go func() {
		for range pendingFiles {
			<-progressChan
			processedCount++
			progress := float64(processedCount) / float64(totalFiles) * 100
			kb.UpdateSyncProgress(SyncProgress{
				TotalFiles:     totalFiles,
				ProcessedFiles: processedCount,
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
	cacheMutex.RLock()
	if vector, ok := embeddingCache[key]; ok {
		cacheMutex.RUnlock()
		return vector, nil
	}
	cacheMutex.RUnlock()

	if llm.CurrentEngine == nil {
		return nil, fmt.Errorf("LLM engine not initialized")
	}

	embedding, err := llm.CurrentEngine.GetEmbedding(text)
	if err != nil {
		return nil, err
	}

	vector := Float32SliceToBytes(embedding)

	// 存入缓存
	cacheMutex.Lock()
	embeddingCache[key] = vector
	cacheMutex.Unlock()

	return vector, nil
}

func (kb *KnowledgeBase) processFile(f db.KnowledgeBaseFile) error {
	ext := strings.ToLower(filepath.Ext(f.Path))
	var content string
	var err error

	switch ext {
	case ".pdf":
		content, err = extractTextFromPDF(f.Path)
	case ".docx":
		content, err = extractTextFromDocx(f.Path)
	case ".xlsx", ".xls":
		content, err = extractTextFromXlsx(f.Path)
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

	// 先删除旧切片
	if err := db.DeleteKBChunks(f.ID); err != nil {
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
		chunks = splitText(content, 2000, 300) // XLSX 和 XLS 文件使用更大的分片，减少分片数量
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

	// 更新同步进度，添加分片进度信息
	kb.progressMu.Lock()
	chunkProgress := ChunkProgress{
		FileName:        fileName,
		TotalChunks:     totalChunks,
		ProcessedChunks: 0,
		Progress:        0,
	}
	kb.progress.ChunkProgress = []ChunkProgress{chunkProgress}
	kb.progressMu.Unlock()

	// 并发处理切片的向量生成和存储
	var wg sync.WaitGroup
	errChan := make(chan error, len(validChunks))
	progressChan := make(chan struct{}, len(validChunks))

	// 控制并发数量 - 优化：进一步增加并发数
	chunkConcurrencyLimit := 30 // 增加到 30 以提高 XLSX 文件处理速度
	chunkSemaphore := make(chan struct{}, chunkConcurrencyLimit)

	for _, chunk := range validChunks {
		wg.Add(1)
		go func(chunkContent string) {
			defer wg.Done()

			// 申请信号量
			chunkSemaphore <- struct{}{}
			defer func() { <-chunkSemaphore }()

			// 生成向量
			vector, err := getEmbedding(chunkContent)
			if err != nil {
				// 如果生成向量失败，继续处理，不返回错误
				fmt.Printf("Error getting embedding: %v\n", err)
				vector = nil
			}

			if err := db.SaveKBChunk(f.ID, chunkContent, vector); err != nil {
				errChan <- err
				return
			}

			// 处理完成一个分片，发送进度更新
			progressChan <- struct{}{}
		}(chunk)
	}

	// 处理分片进度更新
	go func() {
		for range validChunks {
			<-progressChan
			processedChunks++
			progress := float64(processedChunks) / float64(totalChunks) * 100

			// 更新分片进度
			kb.progressMu.Lock()
			kb.progress.ChunkProgress = []ChunkProgress{{
				FileName:        fileName,
				TotalChunks:     totalChunks,
				ProcessedChunks: processedChunks,
				Progress:        progress,
			}}
			kb.progressMu.Unlock()
		}
	}()

	// 等待所有处理完成
	wg.Wait()
	close(errChan)
	close(progressChan)

	// 检查是否有错误
	for err := range errChan {
		if err != nil {
			// 只返回第一个错误
			return err
		}
	}

	return nil
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

		// 计算最大处理行数：文件大小每增加1MB，增加1000行，最大不超过40000行
		maxProcessRows := int(fileSize/(1024*1024)) * 1000
		if maxProcessRows < 1000 {
			maxProcessRows = 1000
		}
		if maxProcessRows > 40000 {
			maxProcessRows = 40000
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

	// 计算最大处理行数：文件大小每增加1MB，增加1000行，最大不超过40000行
	maxProcessRows := int(fileSize/(1024*1024)) * 1000
	if maxProcessRows < 1000 {
		maxProcessRows = 1000
	}
	if maxProcessRows > 40000 {
		maxProcessRows = 40000
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
