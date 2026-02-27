package server

import (
	"net/http"
	"strconv"

	"knowledge/internal/db"

	"github.com/gin-gonic/gin"
)

type CreateConversationRequest struct {
	Title string `json:"title"`
}

type UpdateMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

func (s *Server) ListConversations(c *gin.Context) {
	cs, err := db.ListConversations(50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cs)
}

func (s *Server) DeleteConversation(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}
	if err := db.DeleteConversation(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (s *Server) BatchDeleteConversations(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := db.DeleteConversations(req.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "deleted": len(req.IDs)})
}

func (s *Server) CreateConversation(c *gin.Context) {
	var req CreateConversationRequest
	_ = c.ShouldBindJSON(&req)
	conv, err := db.CreateConversation(req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, conv)
}

func (s *Server) GetConversationMessages(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}
	msgs, err := db.GetHistory(uint(id), 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, msgs)
}

func (s *Server) UpdateMessage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}
	mid, err := strconv.ParseUint(c.Param("mid"), 10, 64)
	if err != nil || mid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
		return
	}
	var req UpdateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	convID := uint(id)
	lastUser, err := db.GetLastUserMessage(convID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get last user message: " + err.Error()})
		return
	}
	if lastUser == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no user message found"})
		return
	}

	if lastUser.ID != uint(mid) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only last user message can be edited"})
		return
	}

	if err := db.UpdateMessageContent(convID, uint(mid), req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := db.DeleteMessagesAfter(convID, uint(mid)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
