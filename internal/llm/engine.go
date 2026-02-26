package llm

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Engine interface {
	Init(modelPath string) error
	Chat(history []ChatMessage) (string, error)
	ChatStream(history []ChatMessage, onToken func(token string) bool) error
	SwitchModel(modelPath string) error
	ListModels() ([]string, error)
	GetModelPath() string
	GetEmbedding(text string) ([]float32, error)
}

type ChatOptions struct {
	MaxTokens     int
	Temperature   float32
	TopP          float32
	TopK          int
	RepeatPenalty float32
	Stop          []string
}

type EngineWithOptions interface {
	ChatWithOptions(history []ChatMessage, opts ChatOptions) (string, error)
	ChatStreamWithOptions(history []ChatMessage, opts ChatOptions, onToken func(token string) bool) error
}

// Global instance
var CurrentEngine Engine
