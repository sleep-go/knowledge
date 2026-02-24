package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"knowledge/internal/db"
	"knowledge/internal/kb"
	"knowledge/internal/llm"

	"github.com/gin-gonic/gin"
)

type ChatRequest struct {
	Message string `json:"message" binding:"required"`
}

type ChatResponse struct {
	Response string `json:"response"`
}

type CreateConversationRequest struct {
	Title string `json:"title"`
}

type UpdateMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

type UpdateSettingRequest struct {
	Value string `json:"value" binding:"required"`
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

func truncateRunes(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	s = strings.Join(strings.Fields(s), " ")
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func ensureFallbackTitle(conversationID uint, userText string) {
	c, err := db.GetConversation(conversationID)
	if err != nil || c == nil {
		return
	}
	if c.Title == "Default" {
		return
	}
	if strings.TrimSpace(c.Title) != "" && c.Title != "New chat" {
		return
	}
	title := truncateRunes(userText, 20)
	if title == "" {
		title = "New chat"
	}
	_ = db.UpdateConversationTitle(conversationID, title)
}

func sanitizeTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.ReplaceAll(title, "**", "")
	title = strings.ReplaceAll(title, "*", "")
	title = strings.ReplaceAll(title, "`", "")
	title = strings.Trim(title, "\"'“”‘’「」")
	if i := strings.LastIndex(title, "标题"); i >= 0 {
		sub := strings.TrimSpace(title[i:])
		sub = strings.TrimSpace(strings.TrimPrefix(sub, "标题："))
		sub = strings.TrimSpace(strings.TrimPrefix(sub, "标题:"))
		title = sub
	}
	title = strings.TrimSpace(strings.TrimPrefix(title, "标题："))
	title = strings.TrimSpace(strings.TrimPrefix(title, "标题:"))
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.Join(strings.Fields(title), " ")
	title = truncateRunes(title, 20)
	return title
}

func heuristicTitleFromUser(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "，。！？、,.!? ")
	for _, p := range []string{"请你", "请", "帮我", "给我", "麻烦", "能不能", "能否", "如何", "怎么"} {
		s = strings.TrimSpace(strings.TrimPrefix(s, p))
	}
	for _, p := range []string{"写一个", "写", "总结一下", "总结", "解释一下", "解释", "介绍一下", "介绍", "给出", "提供"} {
		s = strings.TrimSpace(strings.TrimPrefix(s, p))
	}
	s = strings.TrimLeft(s, "，。！？、,.!? ")
	return truncateRunes(s, 20)
}

func augmentHistoryWithKB(history []llm.ChatMessage, lastUserMsg string) []llm.ChatMessage {
	chunks, err := db.SearchKBChunks(lastUserMsg, 5) // 增加到 5 个分片
	if err != nil || len(chunks) == 0 {
		return history
	}

	fmt.Printf("[KB] Found %d chunks for query: %s\n", len(chunks), lastUserMsg)

	contextText := "\n\n【本地知识库参考资料】\n"
	for i, chunk := range chunks {
		contextText += fmt.Sprintf("[%d] %s\n", i+1, chunk.Content)
	}
	contextText += "\n请根据以上提供的【参考资料】来回答用户的问题。如果参考资料中没有相关信息，请明确告知用户并结合你的通用知识回答。"

	// 在最后一个用户消息后面附加知识库内容
	if len(history) > 0 {
		lastIdx := len(history) - 1
		if history[lastIdx].Role == "user" {
			// 为了防止 prompt 过长，这里可以做一点清理或截断
			history[lastIdx].Content += contextText
		}
	}
	return history
}

func isBadTitle(title string, fallback string) bool {
	title = strings.TrimSpace(title)
	if title == "" || title == fallback {
		return true
	}
	if len([]rune(title)) < 4 {
		return true
	}
	for _, p := range []string{"好的", "明白", "请", "您好", "你好"} {
		if strings.HasPrefix(title, p) {
			return true
		}
	}
	for _, kw := range []string{"提出", "问题", "我会", "尽力", "帮助"} {
		if strings.Contains(title, kw) {
			return true
		}
	}
	return false
}

func tryGenerateSmartTitle(conversationID uint, engine llm.Engine) {
	c, err := db.GetConversation(conversationID)
	if err != nil || c == nil {
		return
	}
	if c.Title == "Default" {
		return
	}

	firstUser, err := db.GetFirstUserMessage(conversationID)
	if err != nil || firstUser == nil {
		return
	}
	fallback := truncateRunes(firstUser.Content, 20)
	if c.Title != "New chat" && c.Title != fallback {
		return
	}

	titlePrompt := []llm.ChatMessage{
		{Role: "user", Content: "只输出一个不超过20字的中文标题，不要任何多余文字。标题：" + firstUser.Content},
	}

	if e, ok := engine.(llm.EngineWithOptions); ok {
		out, err := e.ChatWithOptions(titlePrompt, llm.ChatOptions{
			MaxTokens:     64,
			Temperature:   0.7,
			TopP:          0.9,
			TopK:          40,
			RepeatPenalty: 1.1,
			Stop:          nil,
		})
		if err == nil {
			title := sanitizeTitle(out)
			if !isBadTitle(title, fallback) {
				_ = db.UpdateConversationTitle(conversationID, title)
				return
			}
		}
	}

	ht := heuristicTitleFromUser(firstUser.Content)
	if ht != "" && ht != fallback {
		_ = db.UpdateConversationTitle(conversationID, ht)
	}
}

func SetupRouter(staticFS embed.FS, engine llm.Engine, kbase *kb.KnowledgeBase) *gin.Engine {
	r := gin.Default()

	// Serve static files from embed.FS
	staticFiles, _ := fs.Sub(staticFS, "static")
	r.StaticFS("/static", http.FS(staticFiles))

	// Serve index.html
	r.GET("/", func(c *gin.Context) {
		indexData, _ := staticFS.ReadFile("index.html")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
	})

	defaultConv, err := db.GetOrCreateDefaultConversation()
	if err != nil {
		panic(err)
	}

	api := r.Group("/api")
	{
		api.GET("/conversations", func(c *gin.Context) {
			cs, err := db.ListConversations(50)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, cs)
		})

		api.DELETE("/conversations/:id", func(c *gin.Context) {
			id, err := strconv.ParseUint(c.Param("id"), 10, 64)
			if err != nil || id == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
				return
			}
			convID := uint(id)
			if err := db.DeleteConversation(convID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})

		api.POST("/conversations", func(c *gin.Context) {
			var req CreateConversationRequest
			_ = c.ShouldBindJSON(&req)
			conv, err := db.CreateConversation(req.Title)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, conv)
		})

		api.GET("/conversations/:id/messages", func(c *gin.Context) {
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
		})

		api.GET("/history", func(c *gin.Context) {
			messages, err := db.GetHistory(defaultConv.ID, 200)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, messages)
		})

		api.POST("/chat", func(c *gin.Context) {
			var req ChatRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

			response, err := engine.Chat(history)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			if err := db.SaveMessage(defaultConv.ID, "assistant", response); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, ChatResponse{Response: response})
		})

		api.POST("/chat/stream", func(c *gin.Context) {
			var req ChatRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

			start := 0
			if len(dbMessages) > 20 {
				start = len(dbMessages) - 20
			}
			history := make([]llm.ChatMessage, 0, len(dbMessages)-start)
			for i := start; i < len(dbMessages); i++ {
				history = append(history, llm.ChatMessage{Role: dbMessages[i].Role, Content: dbMessages[i].Content})
			}

			// 增加知识库上下文
			history = augmentHistoryWithKB(history, req.Message)

			flusher, ok := c.Writer.(http.Flusher)
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
				return
			}

			c.Header("Content-Type", "text/plain; charset=utf-8")
			c.Header("Cache-Control", "no-cache")
			c.Status(http.StatusOK)

			var out strings.Builder
			err = engine.ChatStream(history, func(token string) bool {
				select {
				case <-c.Request.Context().Done():
					return false
				default:
				}
				if token == "" {
					return true
				}
				out.WriteString(token)
				_, _ = c.Writer.WriteString(token)
				flusher.Flush()
				return true
			})
			if err != nil {
				if out.Len() == 0 {
					_, _ = c.Writer.WriteString("ERROR: " + err.Error())
					flusher.Flush()
				}
				return
			}
			_ = db.SaveMessage(defaultConv.ID, "assistant", out.String())
		})

		api.POST("/conversations/:id/chat", func(c *gin.Context) {
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

			// 增加知识库上下文
			history = augmentHistoryWithKB(history, req.Message)

			response, err := engine.Chat(history)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			if err := db.SaveMessage(convID, "assistant", response); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			tryGenerateSmartTitle(convID, engine)

			c.JSON(http.StatusOK, ChatResponse{Response: response})
		})

		api.POST("/conversations/:id/chat/stream", func(c *gin.Context) {
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

			// 增加知识库上下文
			history = augmentHistoryWithKB(history, req.Message)

			flusher, ok := c.Writer.(http.Flusher)
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
				return
			}

			c.Header("Content-Type", "text/plain; charset=utf-8")
			c.Header("Cache-Control", "no-cache")
			c.Status(http.StatusOK)

			var out strings.Builder
			err = engine.ChatStream(history, func(token string) bool {
				select {
				case <-c.Request.Context().Done():
					return false
				default:
				}
				if token == "" {
					return true
				}
				out.WriteString(token)
				_, _ = c.Writer.WriteString(token)
				flusher.Flush()
				return true
			})
			if err != nil {
				if out.Len() == 0 {
					_, _ = c.Writer.WriteString("ERROR: " + err.Error())
					flusher.Flush()
				}
				return
			}
			_ = db.SaveMessage(convID, "assistant", out.String())
			tryGenerateSmartTitle(convID, engine)
		})

		api.PATCH("/conversations/:id/messages/:mid", func(c *gin.Context) {
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
			if err != nil || lastUser == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "no user message"})
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
		})

		api.POST("/conversations/:id/retry/stream", func(c *gin.Context) {
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

			start := 0
			if len(dbMessages) > 20 {
				start = len(dbMessages) - 20
			}
			history := make([]llm.ChatMessage, 0, len(dbMessages)-start)
			for i := start; i < len(dbMessages); i++ {
				history = append(history, llm.ChatMessage{Role: dbMessages[i].Role, Content: dbMessages[i].Content})
			}

			// 增加知识库上下文
			if len(history) > 0 && history[len(history)-1].Role == "user" {
				history = augmentHistoryWithKB(history, history[len(history)-1].Content)
			}

			flusher, ok := c.Writer.(http.Flusher)
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
				return
			}

			c.Header("Content-Type", "text/plain; charset=utf-8")
			c.Header("Cache-Control", "no-cache")
			c.Status(http.StatusOK)

			var out strings.Builder
			err = engine.ChatStream(history, func(token string) bool {
				select {
				case <-c.Request.Context().Done():
					return false
				default:
				}
				if token == "" {
					return true
				}
				out.WriteString(token)
				_, _ = c.Writer.WriteString(token)
				flusher.Flush()
				return true
			})
			if err != nil {
				if out.Len() == 0 {
					_, _ = c.Writer.WriteString("ERROR: " + err.Error())
					flusher.Flush()
				}
				return
			}
			_ = db.SaveMessage(convID, "assistant", out.String())
			tryGenerateSmartTitle(convID, engine)
		})

		// 知识库设置相关接口
		api.GET("/settings/kb-folder", func(c *gin.Context) {
			folder, _ := db.GetKBFolder()
			c.JSON(http.StatusOK, gin.H{"folder": folder})
		})

		api.POST("/settings/kb-folder", func(c *gin.Context) {
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
		})

		api.POST("/settings/select-folder", func(c *gin.Context) {
			// 仅在 macOS 上工作
			script := `choose folder with prompt "请选择知识库文件夹" default location (path to home folder)`
			cmd := exec.Command("osascript", "-e", script)
			output, err := cmd.CombinedOutput()
			if err != nil {
				// 用户取消或出错
				c.JSON(http.StatusOK, gin.H{"path": ""})
				return
			}

			// osascript 返回格式类似 "alias Macintosh HD:Users:name:folder:"
			pathStr := strings.TrimSpace(string(output))
			if strings.HasPrefix(pathStr, "alias ") {
				pathStr = strings.TrimPrefix(pathStr, "alias ")
			}

			// 转换为 POSIX 路径
			posixScript := fmt.Sprintf(`POSIX path of %s`, string(output))
			posixCmd := exec.Command("osascript", "-e", posixScript)
			posixOutput, posixErr := posixCmd.CombinedOutput()
			if posixErr == nil {
				pathStr = strings.TrimSpace(string(posixOutput))
			}

			c.JSON(http.StatusOK, gin.H{"path": pathStr})
		})

		api.GET("/kb/files", func(c *gin.Context) {
			files, err := db.ListKBFiles()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, files)
		})

		api.POST("/kb/sync", func(c *gin.Context) {
			if err := kbase.ScanFolder(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// 异步处理文件
			go func() {
				if err := kbase.ProcessFiles(); err != nil {
					fmt.Printf("Error processing KB files: %v\n", err)
				}
			}()
			c.JSON(http.StatusOK, gin.H{"ok": true, "message": "Knowledge base sync started"})
		})
	}

	v1 := r.Group("/v1")
	{
		v1.POST("/chat/completions", func(c *gin.Context) {
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
				if e, ok := engine.(llm.EngineWithOptions); ok {
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
				} else {
					streamErr = engine.ChatStream(req.Messages, func(token string) bool {
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
				}

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
			if e, ok := engine.(llm.EngineWithOptions); ok {
				respText, err = e.ChatWithOptions(req.Messages, opts)
			} else {
				respText, err = engine.Chat(req.Messages)
			}
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
		})
	}

	return r
}
