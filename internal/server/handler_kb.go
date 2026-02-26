package server

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"knowledge/internal/db"

	"github.com/gin-gonic/gin"
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
