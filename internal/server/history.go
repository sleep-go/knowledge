package server

import (
	"knowledge/internal/db"
	"knowledge/internal/kb"
	"knowledge/internal/llm"
)

// BuildHistory 从数据库消息构建聊天历史
// 该函数会截取最后tail条消息并转换为llm.ChatMessage格式
// 注意：为了避免上下文过长，建议不要将tail设置得过大，推荐在10-20之间
func BuildHistory(dbMessages []db.Message, tail int) []llm.ChatMessage {
	start := 0
	if len(dbMessages) > tail {
		start = len(dbMessages) - tail
	}
	
	history := make([]llm.ChatMessage, 0, len(dbMessages)-start)
	for i := start; i < len(dbMessages); i++ {
		history = append(history, llm.ChatMessage{
			Role:    dbMessages[i].Role,
			Content: dbMessages[i].Content,
		})
	}
	
	return history
}

// BuildHistoryWithKB 从数据库消息构建带知识库上下文的聊天历史
// 该函数会截取最后tail条消息，转换为llm.ChatMessage格式，并添加知识库上下文
func BuildHistoryWithKB(kbase *kb.KnowledgeBase, dbMessages []db.Message, tail int, seed string) []llm.ChatMessage {
	history := BuildHistory(dbMessages, tail)
	return augmentHistoryWithKB(kbase, history, seed)
}

// BuildRetryHistoryWithKB 为重试操作构建带知识库上下文的聊天历史
// 该函数会截取最后tail条消息，转换为llm.ChatMessage格式，
// 如果最后一条是用户消息，则添加知识库上下文
func BuildRetryHistoryWithKB(kbase *kb.KnowledgeBase, dbMessages []db.Message, tail int) []llm.ChatMessage {
	history := BuildHistory(dbMessages, tail)
	
	// 如果历史记录不为空且最后一条是用户消息，则添加知识库上下文
	if len(history) > 0 && history[len(history)-1].Role == "user" {
		return augmentHistoryWithKB(kbase, history, history[len(history)-1].Content)
	}
	
	return history
}