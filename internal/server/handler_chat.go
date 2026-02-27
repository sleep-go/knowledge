package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"knowledge/internal/db"
	"knowledge/internal/llm"

	"github.com/gin-gonic/gin"
)

type ChatRequest struct {
	Message string `json:"message" binding:"required"`
}

type ChatResponse struct {
	Response string `json:"response"`
}

type OAIChatCompletionRequest struct {
	Model       string            `json:"model"`
	Messages    []llm.ChatMessage `json:"messages"`
	Stream      bool              `json:"stream"`
	Temperature *float32          `json:"temperature"`
	TopP        *float32          `json:"top_p"`
	MaxTokens   *int              `json:"max_tokens"`
	Stop        json.RawMessage   `json:"stop"`
}

func (s *Server) ListModels(c *gin.Context) {
	// 获取可用模型列表
	var models []string
	currentPath := ""
	err := s.withEngineLocked(func() error {
		var e error
		models, e = s.engine.ListModels()
		if e != nil {
			return e
		}
		currentPath = s.engine.GetModelPath()
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 提取文件名
	currentModel := ""
	if currentPath != "" {
		currentModel = filepath.Base(currentPath)
	}

	c.JSON(http.StatusOK, gin.H{
		"current_model": currentModel,
		"models":        models,
	})
}

func (s *Server) SelectModel(c *gin.Context) {
	var req struct {
		Model string `json:"model" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var switchErr error
	_ = s.withEngineLocked(func() error {
		currentPath := s.engine.GetModelPath()
		dir := "models"
		if currentPath != "" {
			dir = filepath.Dir(currentPath)
		}
		newPath := filepath.Join(dir, req.Model)
		switchErr = s.engine.SwitchModel(newPath)
		return switchErr
	})
	if switchErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": switchErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "model": req.Model})
}

func (s *Server) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	defaultConv, err := db.GetOrCreateDefaultConversation()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := db.SaveMessage(defaultConv.ID, "user", req.Message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	dbMessages, err := db.GetHistory(defaultConv.ID, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var history []llm.ChatMessage

	start := 0
	if len(dbMessages) > 20 {
		start = len(dbMessages) - 20
	}

	for i := start; i < len(dbMessages); i++ {
		history = append(history, llm.ChatMessage{
			Role:    dbMessages[i].Role,
			Content: dbMessages[i].Content,
		})
	}

	var response string
	err = s.withEngineLocked(func() error {
		var e error
		response, e = s.engine.Chat(history)
		return e
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := db.SaveMessage(defaultConv.ID, "assistant", response); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ChatResponse{Response: response})
}

func (s *Server) ChatStream(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	defaultConv, err := db.GetOrCreateDefaultConversation()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := db.SaveMessage(defaultConv.ID, "user", req.Message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	dbMessages, err := db.GetHistory(defaultConv.ID, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var response string
	err = s.withEngineLocked(func() error {
		history := BuildHistoryWithKB(s.kbase, dbMessages, 10, req.Message)
		var e error
		response, e = WritePlainTokens(c, func(yield func(string) bool) error {
			return s.engine.ChatStream(history, yield)
		}, StreamOptions{})
		return e
	})

	if err != nil {
		return
	}

	_ = db.SaveMessage(defaultConv.ID, "assistant", response)
}

func (s *Server) ChatWithConversation(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	convID := uint(id)
	if err := db.SaveMessage(convID, "user", req.Message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ensureFallbackTitle(convID, req.Message)

	dbMessages, err := db.GetHistory(convID, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	start := 0
	if len(dbMessages) > 20 {
		start = len(dbMessages) - 20
	}

	history := make([]llm.ChatMessage, 0, len(dbMessages)-start)
	for i := start; i < len(dbMessages); i++ {
		history = append(history, llm.ChatMessage{Role: dbMessages[i].Role, Content: dbMessages[i].Content})
	}

	var response string
	err = s.withEngineLocked(func() error {
		history = augmentHistoryWithKB(s.kbase, history, req.Message)
		var e error
		response, e = s.engine.Chat(history)
		return e
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := db.SaveMessage(convID, "assistant", response); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = s.withEngineLocked(func() error {
		tryGenerateSmartTitle(convID, s.engine)
		return nil
	})

	c.JSON(http.StatusOK, ChatResponse{Response: response})
}

func (s *Server) ChatStreamWithConversation(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	convID := uint(id)
	if err := db.SaveMessage(convID, "user", req.Message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ensureFallbackTitle(convID, req.Message)

	dbMessages, err := db.GetHistory(convID, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var response string
	err = s.withEngineLocked(func() error {
		history := BuildHistoryWithKB(s.kbase, dbMessages, 10, req.Message)
		var e error
		response, e = WritePlainTokens(c, func(yield func(string) bool) error {
			return s.engine.ChatStream(history, yield)
		}, StreamOptions{})
		return e
	})

	if err != nil {
		return
	}

	_ = db.SaveMessage(convID, "assistant", response)
	_ = s.withEngineLocked(func() error {
		tryGenerateSmartTitle(convID, s.engine)
		return nil
	})
}

func (s *Server) RetryStream(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}
	convID := uint(id)

	lastUser, err := db.GetLastUserMessage(convID)
	if err != nil || lastUser == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no user message"})
		return
	}
	if err := db.DeleteMessagesAfter(convID, lastUser.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	dbMessages, err := db.GetHistory(convID, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var response string
	err = s.withEngineLocked(func() error {
		history := BuildRetryHistoryWithKB(s.kbase, dbMessages, 5)
		fmt.Printf("[Retry] History length: %d\n", len(history))
		for i, msg := range history {
			fmt.Printf("[Retry] Msg %d (%s): %s\n", i, msg.Role, truncateRunes(msg.Content, 50))
		}
		var e error
		response, e = WritePlainTokens(c, func(yield func(string) bool) error {
			return s.engine.ChatStream(history, yield)
		}, StreamOptions{})
		return e
	})

	if err != nil {
		fmt.Printf("[Retry] Stream error: %v\n", err)
		return
	}
	if response == "" {
		fmt.Println("[Retry] Warning: Empty response from model")
	}

	_ = db.SaveMessage(convID, "assistant", response)
	_ = s.withEngineLocked(func() error {
		tryGenerateSmartTitle(convID, s.engine)
		return nil
	})
}

func (s *Server) OAIChatCompletion(c *gin.Context) {
	var req OAIChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "messages is required"})
		return
	}

	modelName := req.Model
	if modelName == "" {
		modelName = "local-llama.cpp"
	}

	var stops []string
	if len(req.Stop) > 0 {
		var s string
		if err := json.Unmarshal(req.Stop, &s); err == nil {
			stops = append(stops, s)
		} else {
			var ss []string
			if err := json.Unmarshal(req.Stop, &ss); err == nil {
				stops = append(stops, ss...)
			}
		}
	}

	opts := llm.ChatOptions{
		MaxTokens:     512,
		Temperature:   0.7,
		TopP:          0.95,
		TopK:          40,
		RepeatPenalty: 1.1,
		Stop:          stops,
	}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		opts.MaxTokens = *req.MaxTokens
	}
	if req.Temperature != nil {
		opts.Temperature = *req.Temperature
	}
	if req.TopP != nil {
		opts.TopP = *req.TopP
	}

	id := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	created := time.Now().Unix()

	if req.Stream {
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Status(http.StatusOK)

		first := true
		var streamErr error
		_ = s.withEngineLocked(func() error {
			if e, ok := s.engine.(llm.EngineWithOptions); ok {
				streamErr = e.ChatStreamWithOptions(req.Messages, opts, func(token string) bool {
					select {
					case <-c.Request.Context().Done():
						return false
					default:
					}

					if first {
						first = false
						chunk := gin.H{
							"id":      id,
							"object":  "chat.completion.chunk",
							"created": created,
							"model":   modelName,
							"choices": []gin.H{
								{"index": 0, "delta": gin.H{"role": "assistant"}, "finish_reason": nil},
							},
						}
						b, _ := json.Marshal(chunk)
						_, _ = c.Writer.WriteString("data: " + string(b) + "\n\n")
						flusher.Flush()
					}

					if token == "" {
						return true
					}
					chunk := gin.H{
						"id":      id,
						"object":  "chat.completion.chunk",
						"created": created,
						"model":   modelName,
						"choices": []gin.H{
							{"index": 0, "delta": gin.H{"content": token}, "finish_reason": nil},
						},
					}
					b, _ := json.Marshal(chunk)
					_, _ = c.Writer.WriteString("data: " + string(b) + "\n\n")
					flusher.Flush()
					return true
				})
				return streamErr
			}

			streamErr = s.engine.ChatStream(req.Messages, func(token string) bool {
				select {
				case <-c.Request.Context().Done():
					return false
				default:
				}

				if first {
					first = false
					chunk := gin.H{
						"id":      id,
						"object":  "chat.completion.chunk",
						"created": created,
						"model":   modelName,
						"choices": []gin.H{
							{"index": 0, "delta": gin.H{"role": "assistant"}, "finish_reason": nil},
						},
					}
					b, _ := json.Marshal(chunk)
					_, _ = c.Writer.WriteString("data: " + string(b) + "\n\n")
					flusher.Flush()
				}

				if token == "" {
					return true
				}
				chunk := gin.H{
					"id":      id,
					"object":  "chat.completion.chunk",
					"created": created,
					"model":   modelName,
					"choices": []gin.H{
						{"index": 0, "delta": gin.H{"content": token}, "finish_reason": nil},
					},
				}
				b, _ := json.Marshal(chunk)
				_, _ = c.Writer.WriteString("data: " + string(b) + "\n\n")
				flusher.Flush()
				return true
			})
			return streamErr
		})

		finalChunk := gin.H{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   modelName,
			"choices": []gin.H{
				{"index": 0, "delta": gin.H{}, "finish_reason": "stop"},
			},
		}
		b, _ := json.Marshal(finalChunk)
		_, _ = c.Writer.WriteString("data: " + string(b) + "\n\n")
		_, _ = c.Writer.WriteString("data: [DONE]\n\n")
		flusher.Flush()
		_ = streamErr
		return
	}

	var respText string
	var err error
	err = s.withEngineLocked(func() error {
		if e, ok := s.engine.(llm.EngineWithOptions); ok {
			var e2 error
			respText, e2 = e.ChatWithOptions(req.Messages, opts)
			return e2
		}
		var e2 error
		respText, e2 = s.engine.Chat(req.Messages)
		return e2
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"object":  "chat.completion",
		"created": created,
		"model":   modelName,
		"choices": []gin.H{
			{
				"index": 0,
				"message": gin.H{
					"role":    "assistant",
					"content": respText,
				},
				"finish_reason": "stop",
			},
		},
	})
}
