package server

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"knowledge/internal/db"
	"knowledge/internal/kb"
	"knowledge/internal/llm"
)

func truncateTextKeepNewlines(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
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
	// Remove common markdown or quote characters first to avoid them being treated as part of the title if they wrap it
	title = strings.Trim(title, "\"'“”‘’「」`")

	// Remove any <...> tags
	for {
		start := strings.Index(title, "<")
		if start == -1 {
			break
		}
		end := strings.Index(title[start:], ">")
		if end == -1 {
			break
		}
		end += start
		title = title[:start] + title[end+1:]
	}

	// Handle "Title:" prefix commonly output by LLMs
	if i := strings.LastIndex(title, "标题"); i >= 0 {
		sub := strings.TrimSpace(title[i:])
		sub = strings.TrimSpace(strings.TrimPrefix(sub, "标题："))
		sub = strings.TrimSpace(strings.TrimPrefix(sub, "标题:"))
		title = sub
	}
	title = strings.TrimSpace(strings.TrimPrefix(title, "标题："))
	title = strings.TrimSpace(strings.TrimPrefix(title, "标题:"))

	// Strict filter: allow only alphanumeric (including unicode letters), spaces, hyphens, underscores
	var sb strings.Builder
	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) || r == '-' || r == '_' {
			sb.WriteRune(r)
		}
	}
	title = sb.String()

	// Normalize spaces
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

func augmentHistoryWithKB(kbase *kb.KnowledgeBase, history []llm.ChatMessage, lastUserMsg string) []llm.ChatMessage {
	var fileContext string

	// 1. 尝试直接读取附件内容
	// 匹配前端生成的: [已上传文件: [filename](/api/kb/download?file=...)]
	// 这里的正则表达式有两层含义：
	// 1. `\[([^\[\]]+)\]`：匹配 Markdown 链接的 Label 部分，且不允许包含嵌套的 `[` 或 `]`，这避免了匹配外层的 `[已上传文件: ...]` 结构。
	// 2. `\(/api/kb/download\?file=([^)]+)\)`：匹配 URL 部分，并捕获 `file` 参数的值（URL 编码后的文件名）。
	re := regexp.MustCompile(`\[([^\[\]]+)\]\(/api/kb/download\?file=([^)]+)\)`)
	matches := re.FindStringSubmatch(lastUserMsg)
	if len(matches) > 2 && kbase != nil {
		// 优先使用 Label 中的文件名（通常是原始文件名）
		filename := strings.TrimSpace(matches[1])
		// 也可以尝试使用 URL 参数中的文件名（URL 编码过的）
		encodedFilename := matches[2]
		
		fmt.Printf("[KB] Detected file in message. Label: %s, Encoded URL param: %s\n", filename, encodedFilename)

		// 如果 Label 为空或者看起来不正常，尝试解码 URL 参数
		if filename == "" || filename == "已上传文件" { // 简单的防御性检查
			decoded, err := url.QueryUnescape(encodedFilename)
			if err == nil && decoded != "" {
				filename = decoded
				fmt.Printf("[KB] Using decoded filename from URL: %s\n", filename)
			}
		}

		// 优先从 DB 中查找文件路径，比单纯拼接更可靠
		var fullPath string
		files, err := db.ListKBFiles()
		if err == nil {
			for _, f := range files {
				if filepath.Base(f.Path) == filename {
					fullPath = f.Path
					break
				}
			}
		}

		// 如果 DB 没找到，尝试回退到拼接路径
		if fullPath == "" {
			folder, err := db.GetKBFolder()
			if err == nil && folder != "" {
				fullPath = filepath.Join(folder, filename)
			}
		}

		if fullPath != "" {
			content, err := kbase.GetFileContent(fullPath)
			if err == nil {
				const maxFileChars = 10000
				truncated := truncateTextKeepNewlines(content, maxFileChars)

				fileContext += fmt.Sprintf("\n\n<current_file name=\"%s\">\n%s\n</current_file>\n", filename, truncated)
				if len(content) > maxFileChars {
					fileContext += fmt.Sprintf("\n(File content truncated, showing first %d chars)\n", maxFileChars)
				}
				fileContext += "请基于上述 <current_file> 标签中的文件内容进行回答。\n"
				fmt.Printf("[KB] Direct file read: %s (path=%s, len=%d)\n", filename, fullPath, len(truncated))
			} else {
				fmt.Printf("[KB] Failed to read file content %s (path=%s): %v\n", filename, fullPath, err)
			}
		} else {
			fmt.Printf("[KB] File not found in DB or folder: %s\n", filename)
		}
	}

	// 2. 知识库检索 (RAG)
	// 减少分片数量以防止 Prompt 过长
	chunks, err := db.SearchKBChunks(lastUserMsg, 5)

	ragContext := ""
	if err == nil && len(chunks) > 0 {
		fmt.Printf("[KB] Found %d chunks for query: %s\n", len(chunks), lastUserMsg)

		// 限制 KB 总字符数
		const maxKBChars = 3000
		totalLen := 0
		var validChunks []db.KnowledgeBaseChunk

		for i, chunk := range chunks {
			if totalLen+len(chunk.Content) > maxKBChars {
				// 如果单个分片就超过限制，但还没有添加任何分片，尝试截断添加
				if len(validChunks) == 0 {
					truncated := truncateRunes(chunk.Content, maxKBChars)
					fmt.Printf("[KB] Truncating chunk %d (original size %d) to fit limit %d\n", i+1, len(chunk.Content), maxKBChars)
					chunk.Content = truncated
					validChunks = append(validChunks, chunk)
					totalLen += len(truncated)
					break // 添加完这个截断的分片后就停止，避免超限
				}

				fmt.Printf("[KB] Skipping chunk %d (size %d) due to size limit. Current total: %d\n", i+1, len(chunk.Content), totalLen)
				continue
			}
			fmt.Printf("[KB] Chunk %d: %s...\n", i+1, truncateRunes(chunk.Content, 50))
			totalLen += len(chunk.Content)
			validChunks = append(validChunks, chunk)
		}
		fmt.Printf("[KB] Total context length (chars): %d\n", totalLen)

		if len(validChunks) > 0 {
			ragContext += "\n\n<knowledge_base>\n"
			for i, chunk := range validChunks {
				ragContext += fmt.Sprintf("<document index=\"%d\">\n%s\n</document>\n", i+1, chunk.Content)
			}
			ragContext += "</knowledge_base>\n\n"
			ragContext += "请参考以上 <knowledge_base> 标签内提供的背景知识来回答用户的问题。\n"
			ragContext += "如果在背景知识中找到了相关信息，请优先使用背景知识回答。\n"
			ragContext += "如果背景知识中没有相关信息，请忽略它们，并结合你的通用知识回答。\n"
		}
	}

	finalContext := fileContext + ragContext
	if finalContext == "" {
		return history
	}

	// 在最后一个用户消息后面附加知识库内容
	if len(history) > 0 {
		lastIdx := len(history) - 1
		if history[lastIdx].Role == "user" {
			// 为了防止 prompt 过长，这里可以做一点清理或截断
			history[lastIdx].Content += finalContext
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
