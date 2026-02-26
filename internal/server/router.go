package server

import (
	"embed"
	"io/fs"
	"net/http"

	"knowledge/internal/db"
	"knowledge/internal/kb"
	"knowledge/internal/llm"

	"github.com/gin-gonic/gin"
)

func SetupRouter(staticFS embed.FS, engine llm.Engine, kbase *kb.KnowledgeBase) *gin.Engine {
	r := gin.Default()
	s := NewServer(engine, kbase)

	// Serve static files from embed.FS
	staticFiles, _ := fs.Sub(staticFS, "static")
	r.StaticFS("/static", http.FS(staticFiles))

	// Serve index.html
	r.GET("/", func(c *gin.Context) {
		indexData, _ := staticFS.ReadFile("index.html")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
	})

	_, err := db.GetOrCreateDefaultConversation()
	if err != nil {
		panic(err)
	}

	api := r.Group("/api")
	{
		api.GET("/conversations", s.ListConversations)
		api.DELETE("/conversations/:id", s.DeleteConversation)
		api.POST("/conversations/batch-delete", s.BatchDeleteConversations)
		api.POST("/conversations", s.CreateConversation)
		api.GET("/conversations/:id/messages", s.GetConversationMessages)
		
		// /history is an alias for getting default conversation messages
		api.GET("/history", func(c *gin.Context) {
			defaultConv, _ := db.GetOrCreateDefaultConversation()
			msgs, err := db.GetHistory(defaultConv.ID, 200)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, msgs)
		})

		api.GET("/models", s.ListModels)
		api.POST("/models/select", s.SelectModel)

		api.POST("/chat", s.Chat)
		api.POST("/chat/stream", s.ChatStream)

		api.POST("/conversations/:id/chat", s.ChatWithConversation)
		api.POST("/conversations/:id/chat/stream", s.ChatStreamWithConversation)

		api.PATCH("/conversations/:id/messages/:mid", s.UpdateMessage)
		api.POST("/conversations/:id/retry/stream", s.RetryStream)

		// 知识库设置相关接口
		api.GET("/settings/kb-folder", s.GetKBFolder)
		api.POST("/settings/kb-folder", s.UpdateKBFolder)
		api.POST("/settings/select-folder", s.SelectKBFolder)

		api.GET("/kb/files", s.ListKBFiles)
		api.GET("/kb/download", s.DownloadKBFile)
		api.DELETE("/kb/files/:id", s.DeleteKBFile)
		api.POST("/kb/files/batch-delete", s.BatchDeleteKBFiles)
		api.POST("/kb/sync", s.SyncKB)
		api.POST("/kb/upload", s.UploadKBFile)
		api.POST("/kb/reset", s.ResetKB)
	}

	v1 := r.Group("/v1")
	{
		v1.POST("/chat/completions", s.OAIChatCompletion)
	}

	return r
}
