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

func (s *Server) SelectKBFolder(c *gin.Context) {
	// 仅在 macOS 上工作
	// 直接获取 POSIX 路径
	script := `POSIX path of (choose folder with prompt "请选择知识库文件夹" default location (path to home folder))`
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 用户取消或出错
		c.JSON(http.StatusOK, gin.H{"path": ""})
		return
	}

	// output 已经是 POSIX 路径，只需 trim
	pathStr := strings.TrimSpace(string(output))
	c.JSON(http.StatusOK, gin.H{"path": pathStr})
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
