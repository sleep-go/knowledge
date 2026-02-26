package kb

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"knowledge/internal/db"
	"knowledge/internal/llm"

	"github.com/ledongthuc/pdf"
	"github.com/lu4p/cat"
	"github.com/xuri/excelize/v2"
)

type KnowledgeBase struct {
	mu sync.Mutex
}

func NewKnowledgeBase() *KnowledgeBase {
	return &KnowledgeBase{}
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
	folder, err := db.GetKBFolder()
	if err != nil || folder == "" {
		return fmt.Errorf("knowledge base folder not set")
	}

	return filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
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

		// 存入数据库
		_, err = db.SaveKBFile(path, info.Size(), checksum)
		return err
	})
}

// ProcessFiles 处理待处理的文件
func (kb *KnowledgeBase) ProcessFiles() error {
	files, err := db.ListKBFiles()
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.Status != "pending" {
			continue
		}

		if err := kb.processFile(f); err != nil {
			fmt.Printf("Error processing file %s: %v\n", f.Path, err)
			_ = db.UpdateKBFileStatus(f.ID, "error")
			continue
		}

		_ = db.UpdateKBFileStatus(f.ID, "processed")
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
	case ".xlsx":
		return extractTextFromXlsx(path)
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
	if llm.CurrentEngine == nil {
		return nil, fmt.Errorf("LLM engine not initialized")
	}

	embedding, err := llm.CurrentEngine.GetEmbedding(text)
	if err != nil {
		return nil, err
	}

	return Float32SliceToBytes(embedding), nil
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
	case ".xlsx":
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

	// 简单切片：按行或者按固定长度
	chunks := splitText(content, 500, 100) // 每 500 字一个切片，100 字重叠，减少分片大小以适应上下文限制
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			continue
		}

		// 生成向量
		vector, err := getEmbedding(chunk)
		if err != nil {
			// 如果生成向量失败，继续处理，不返回错误
			fmt.Printf("Error getting embedding: %v\n", err)
			vector = nil
		}

		if err := db.SaveKBChunk(f.ID, chunk, vector); err != nil {
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
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		for _, row := range rows {
			for _, colCell := range row {
				fullText.WriteString(colCell)
				fullText.WriteString("\t")
			}
			fullText.WriteString("\n")
		}
	}
	return fullText.String(), nil
}

func isSupportedExt(ext string) bool {
	supported := []string{
		".txt", ".md", ".pdf", ".docx", ".xlsx",
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
