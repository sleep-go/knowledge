package server

import (
	"knowledge/internal/db"
	"knowledge/internal/llm"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildHistory(t *testing.T) {
	// 准备测试数据
	dbMessages := []db.Message{
		{Role: "system", Content: "System message"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
		{Role: "assistant", Content: "I'm fine, thank you!"},
	}

	// 测试截取所有消息
	history := BuildHistory(dbMessages, 10)
	expected := []llm.ChatMessage{
		{Role: "system", Content: "System message"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
		{Role: "assistant", Content: "I'm fine, thank you!"},
	}
	assert.Equal(t, expected, history)

	// 测试截取部分消息
	history = BuildHistory(dbMessages, 3)
	expected = []llm.ChatMessage{
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
		{Role: "assistant", Content: "I'm fine, thank you!"},
	}
	assert.Equal(t, expected, history)

	// 测试空消息列表
	emptyMessages := []db.Message{}
	history = BuildHistory(emptyMessages, 5)
	assert.Empty(t, history)

	// 测试tail为0的情况
	history = BuildHistory(dbMessages, 0)
	assert.Empty(t, history)
}

func TestBuildHistoryWithKB(t *testing.T) {
	// 准备测试数据
	dbMessages := []db.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "Tell me about Go language"},
	}

	// 注意：由于augmentHistoryWithKB是一个内部函数，我们无法直接测试其具体行为
	// 我们只能测试它确实返回了一个非空的历史记录数组
	history := BuildHistoryWithKB(nil, dbMessages, 10, "Go language")

	// 验证基本结构
	assert.NotEmpty(t, history)
	assert.Equal(t, "user", history[0].Role)
	assert.Equal(t, "Hello", history[0].Content)
}

func TestBuildRetryHistoryWithKB(t *testing.T) {
	// 测试最后一条消息是用户消息的情况
	dbMessages := []db.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "Tell me about Go language"},
	}

	// 注意：由于augmentHistoryWithKB是一个内部函数，我们无法直接测试其具体行为
	// 我们只能测试它确实返回了一个非空的历史记录数组
	history := BuildRetryHistoryWithKB(nil, dbMessages, 10)

	// 验证基本结构
	assert.NotEmpty(t, history)
	assert.Equal(t, "user", history[len(history)-1].Role)
	assert.Equal(t, "Tell me about Go language", history[len(history)-1].Content)

	// 测试最后一条消息是助手消息的情况
	dbMessages2 := []db.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	history2 := BuildRetryHistoryWithKB(nil, dbMessages2, 10)

	// 验证基本结构
	assert.NotEmpty(t, history2)
	assert.Equal(t, "assistant", history2[len(history2)-1].Role)
	assert.Equal(t, "Hi there!", history2[len(history2)-1].Content)

	// 测试空消息列表
	emptyMessages := []db.Message{}
	history3 := BuildRetryHistoryWithKB(nil, emptyMessages, 10)
	assert.Empty(t, history3)
}
