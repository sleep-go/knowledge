package server

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"knowledge/internal/db"
	"knowledge/internal/llm"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

type UpdateSettingRequest struct {
	Value string `json:"value" binding:"required"`
}

func (s *Server) GetKBFolder(c *gin.Context) {
	folder, _ := db.GetKBFolder()
	c.JSON(http.StatusOK, gin.H{"folder": folder})
}

func (s *Server) UpdateKBFolder(c *gin.Context) {
	var req UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := db.SetSetting(db.KBFolderKey, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) DownloadKBFile(c *gin.Context) {
	fileName := c.Query("file")
	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File parameter is required"})
		return
	}

	// Sanitize filename
	cleanName := filepath.Base(fileName)

	// Get KB folder
	folder, err := db.GetKBFolder()
	if err != nil || folder == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Knowledge base folder not configured"})
		return
	}

	// Join path
	filePath := filepath.Join(folder, cleanName)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.File(filePath)
}

// GetKBFileContent 获取文件经过解析后的文本内容（用于预览）
func (s *Server) GetKBFileContent(c *gin.Context) {
	fileName := c.Query("file")
	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File parameter is required"})
		return
	}

	folder, err := db.GetKBFolder()
	if err != nil || folder == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Knowledge base folder not set"})
		return
	}

	// 简单防止路径遍历，只允许访问 KB 根目录下的文件
	// 如果支持子目录，需要更复杂的逻辑，但目前 KB 主要是扁平的或由 ScanFolder 决定
	// 这里假设前端传来的只是文件名或相对路径
	// 安全起见，我们先只取 Base
	cleanFileName := filepath.Base(fileName)
	filePath := filepath.Join(folder, cleanFileName)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	content, err := s.kbase.GetFileContent(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file content: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content": content,
	})
}

// PreviewKBExcel 以 HTML 表格预览 Excel（完整数据，流式输出）
func (s *Server) PreviewKBExcel(c *gin.Context) {
	fileName := c.Query("file")
	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File parameter is required"})
		return
	}

	folder, err := db.GetKBFolder()
	if err != nil || folder == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Knowledge base folder not set"})
		return
	}

	cleanFileName := filepath.Base(fileName)
	filePath := filepath.Join(folder, cleanFileName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	ext := strings.ToLower(filepath.Ext(cleanFileName))
	if ext != ".xlsx" && ext != ".xls" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not an excel file"})
		return
	}

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open excel: " + err.Error()})
		return
	}
	defer func() { _ = f.Close() }()

	c.Header("Content-Type", "text/html; charset=utf-8")

	// 简单样式：粘性表头 + 可滚动
	_, _ = c.Writer.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"/>")
	_, _ = c.Writer.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"/>")
	_, _ = c.Writer.WriteString("<title>" + html.EscapeString(cleanFileName) + "</title>")
	_, _ = c.Writer.WriteString(`<style>
	:root{color-scheme:dark;}
	body{margin:0;background:#0b1220;color:#e5e7eb;font-family:ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,Helvetica,Arial;}
	.top{position:sticky;top:0;z-index:10;background:rgba(11,18,32,.92);backdrop-filter:saturate(180%) blur(10px);border-bottom:1px solid rgba(255,255,255,.08);padding:10px 12px;}
	.fn{font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;}
	.hint{opacity:.7;font-size:12px;margin-top:2px;}
	.sheet{padding:10px 12px 0 12px;}
	.sheet h2{margin:10px 0 8px 0;font-size:14px;opacity:.9}
	.table-wrap{border:1px solid rgba(255,255,255,.08);border-radius:12px;overflow:auto;max-height:calc(100vh - 120px);background:rgba(255,255,255,.02);}
	table{border-collapse:separate;border-spacing:0;width:max-content;min-width:100%;}
	thead th{position:sticky;top:0;background:rgba(17,24,39,.98);z-index:5}
	th,td{padding:8px 10px;border-bottom:1px solid rgba(255,255,255,.06);border-right:1px solid rgba(255,255,255,.06);font-size:12px;white-space:nowrap;vertical-align:top;}
	tr:nth-child(even) td{background:rgba(255,255,255,.01);}
	th:last-child,td:last-child{border-right:none;}
	.empty{opacity:.7;padding:12px;}
	</style></head><body>`)
	_, _ = c.Writer.WriteString("<div class=\"top\"><div class=\"fn\">" + html.EscapeString(cleanFileName) + "</div><div class=\"hint\">Excel 预览（完整数据）</div></div>")

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		_, _ = c.Writer.WriteString("<div class=\"sheet\"><div class=\"empty\">无工作表</div></div></body></html>")
		return
	}

	for _, sheet := range sheets {
		rows, err := f.Rows(sheet)
		if err != nil {
			continue
		}

		_, _ = c.Writer.WriteString("<div class=\"sheet\"><h2>" + html.EscapeString(sheet) + "</h2>")
		_, _ = c.Writer.WriteString("<div class=\"table-wrap\"><table>")

		rowNum := 0
		var header []string
		for rows.Next() {
			rowNum++
			cells, err := rows.Columns()
			if err != nil {
				continue
			}

			// 第一行作为表头（若为空则自动补列名）
			if rowNum == 1 {
				header = make([]string, len(cells))
				for i := range cells {
					h := strings.TrimSpace(cells[i])
					if h == "" {
						h = fmt.Sprintf("列%d", i+1)
					}
					header[i] = h
				}
				_, _ = c.Writer.WriteString("<thead><tr>")
				for _, h := range header {
					_, _ = c.Writer.WriteString("<th>" + html.EscapeString(h) + "</th>")
				}
				_, _ = c.Writer.WriteString("</tr></thead><tbody>")
				continue
			}

			_, _ = c.Writer.WriteString("<tr>")
			limit := len(cells)
			if len(header) > 0 && limit < len(header) {
				// 末尾补空，保证列对齐
				for len(cells) < len(header) {
					cells = append(cells, "")
				}
				limit = len(header)
			} else if len(header) > 0 && limit > len(header) {
				limit = len(header)
			}
			for i := 0; i < limit; i++ {
				_, _ = c.Writer.WriteString("<td>" + html.EscapeString(cells[i]) + "</td>")
			}
			_, _ = c.Writer.WriteString("</tr>")
		}
		_ = rows.Close()

		if rowNum <= 1 {
			_, _ = c.Writer.WriteString("<tbody><tr><td class=\"empty\">无数据</td></tr></tbody>")
		} else {
			_, _ = c.Writer.WriteString("</tbody>")
		}
		_, _ = c.Writer.WriteString("</table></div></div>")
	}

	_, _ = c.Writer.WriteString("</body></html>")
}

func (s *Server) SelectKBFolder(c *gin.Context) {
	// 仅在 macOS 上工作
	// 直接获取 POSIX 路径，使用更简洁的输出
	script := `set posixPath to POSIX path of (choose folder with prompt "请选择知识库文件夹" default location (path to home folder))
return posixPath`
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 用户取消或出错
		c.JSON(http.StatusOK, gin.H{"path": ""})
		return
	}

	// 清理输出：只保留路径，过滤掉日志信息
	pathStr := string(output)

	// 找到最后一个换行符，然后取后面的内容
	lines := strings.Split(pathStr, "\n")
	var cleanPath string

	// 从后向前查找，找到第一个非空且看起来像路径的行
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && strings.HasPrefix(line, "/") {
			cleanPath = line
			break
		}
	}

	// 如果没找到，就用原来的方式处理
	if cleanPath == "" {
		cleanPath = strings.TrimSpace(pathStr)
	}

	c.JSON(http.StatusOK, gin.H{"path": cleanPath})
}

func (s *Server) ListKBFiles(c *gin.Context) {
	files, err := db.ListKBFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, files)
}

func (s *Server) SyncKB(c *gin.Context) {
	if err := s.kbase.ScanFolder(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 异步处理文件
	go func() {
		if err := s.kbase.ProcessFiles(); err != nil {
			fmt.Printf("Error processing KB files: %v\n", err)
		}
	}()
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "Knowledge base sync started"})
}

func (s *Server) DebugKBSearch(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	currentModel := ""
	if llm.CurrentEngine != nil {
		currentModel = llm.CurrentEngine.GetModelPath()
	}
	kbModel, _ := db.GetKBEmbeddingModel()

	// 候选集来自文本检索（能确保命中包含编号/关键字的 chunk）
	candidates, err := db.SearchKBChunks(q, 800)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	queryVec, vecErr := func() ([]float32, error) {
		if llm.CurrentEngine == nil {
			return nil, fmt.Errorf("LLM engine not initialized")
		}
		if kbModel != "" && currentModel != "" && kbModel != currentModel {
			return nil, fmt.Errorf("embedding model mismatch (kb=%s, current=%s)", kbModel, currentModel)
		}
		return llm.CurrentEngine.GetEmbedding(q)
	}()

	type item struct {
		ID         uint    `json:"id"`
		FileID     uint    `json:"file_id"`
		Similarity float32 `json:"similarity"`
		HasVector  bool    `json:"has_vector"`
		Snippet    string  `json:"snippet"`
	}

	res := make([]item, 0, limit)
	if vecErr != nil || len(queryVec) == 0 {
		// 无向量时仅返回文本命中情况
		for i := 0; i < len(candidates) && i < limit; i++ {
			ch := candidates[i]
			res = append(res, item{
				ID:         ch.ID,
				FileID:     ch.FileID,
				Similarity: 0,
				HasVector:  len(ch.Vector) > 0,
				Snippet:    truncateRunes(ch.Content, 120),
			})
		}
		c.JSON(http.StatusOK, gin.H{
			"q":                  q,
			"current_model":      currentModel,
			"kb_embedding_model": kbModel,
			"vector_error":       vecErr.Error(),
			"candidates":         len(candidates),
			"results":            res,
		})
		return
	}

	type scored struct {
		ch  db.KnowledgeBaseChunk
		sim float32
	}
	scoredList := make([]scored, 0, len(candidates))
	for _, ch := range candidates {
		if len(ch.Vector) == 0 {
			continue
		}
		v := bytesToFloat32Slice(ch.Vector)
		if len(v) != len(queryVec) {
			continue
		}
		scoredList = append(scoredList, scored{ch: ch, sim: cosineSimilarity(queryVec, v)})
	}
	// 简单排序取前 N（调试端点可接受）
	sort.Slice(scoredList, func(i, j int) bool { return scoredList[i].sim > scoredList[j].sim })

	for i := 0; i < len(scoredList) && i < limit; i++ {
		ch := scoredList[i].ch
		res = append(res, item{
			ID:         ch.ID,
			FileID:     ch.FileID,
			Similarity: scoredList[i].sim,
			HasVector:  true,
			Snippet:    truncateRunes(ch.Content, 120),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"q":                  q,
		"current_model":      currentModel,
		"kb_embedding_model": kbModel,
		"vector_dim":         len(queryVec),
		"candidates":         len(candidates),
		"scored":             len(scoredList),
		"results":            res,
	})
}

func (s *Server) ResetKB(c *gin.Context) {
	if err := db.ResetKnowledgeBase(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "Knowledge base reset successfully"})
}

func (s *Server) UploadKBFile(c *gin.Context) {
	// 1. Get file from request
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	// 2. Get KB folder
	folder, err := db.GetKBFolder()
	if err != nil || folder == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Knowledge base folder not set"})
		return
	}

	// Ensure folder exists
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		if err := os.MkdirAll(folder, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create KB folder: " + err.Error()})
			return
		}
	}

	// 3. Save file to KB folder
	dst := filepath.Join(folder, filepath.Base(file.Filename))
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file: " + err.Error()})
		return
	}

	// 4. Add to KnowledgeBase
	if err := s.kbase.AddFile(dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process file: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "path": dst})
}

func (s *Server) DeleteKBFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}

	if err := db.DeleteKBFile(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) BatchDeleteKBFiles(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := db.DeleteKBFiles(req.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": len(req.IDs)})
}

// GetSyncProgress 获取知识库同步进度
func (s *Server) GetSyncProgress(c *gin.Context) {
	progress := s.kbase.GetSyncProgress()
	c.JSON(http.StatusOK, progress)
}

func (s *Server) GetSystemPrompt(c *gin.Context) {
	prompt, _ := db.GetSystemPrompt()
	c.JSON(http.StatusOK, gin.H{"prompt": prompt})
}

func (s *Server) UpdateSystemPrompt(c *gin.Context) {
	var req UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := db.SetSetting(db.SystemPromptKey, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
