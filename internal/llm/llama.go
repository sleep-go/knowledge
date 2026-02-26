package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"knowledge/internal/binding"
	"knowledge/internal/db"
)

type LlamaEngine struct {
	modelPath string
	model     *binding.Llama
	mu        sync.Mutex
}

type oaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func getSystemPrompt() string {
	s, err := db.GetSystemPrompt()
	if err == nil {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return db.DefaultSystemPrompt
}

func buildMessagesWithSystemPrompt(history []ChatMessage, systemPrompt string) ([]byte, error) {
	msgs := make([]oaMsg, 0, len(history)+1)
	if len(history) == 0 || history[0].Role != "system" {
		msgs = append(msgs, oaMsg{Role: "system", Content: systemPrompt})
	}
	for _, m := range history {
		msgs = append(msgs, oaMsg{Role: m.Role, Content: m.Content})
	}
	return json.Marshal(msgs)
}

func (l *LlamaEngine) Init(modelPath string) error {
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model not found at %s", modelPath)
	}
	l.modelPath = modelPath

	var err error
	// 初始化模型时，设置较大的上下文窗口大小，以适应多轮对话和知识库内容
	// 4096 是大多数现代 Llama 模型的默认上下文窗口大小
	// 如果需要支持更长的上下文，可以进一步增加这个值
	l.model, err = binding.NewLlama(modelPath, 4096, 4, 0)
	if err != nil {
		return err
	}

	fmt.Printf("[LlamaEngine] Initialized with model: %s (Native CGO)\n", modelPath)
	return nil
}

func (l *LlamaEngine) Chat(history []ChatMessage) (string, error) {
	if e, ok := interface{}(l).(EngineWithOptions); ok {
		return e.ChatWithOptions(history, ChatOptions{
			MaxTokens:     512,
			Temperature:   0.7,
			TopP:          0.95,
			TopK:          40,
			RepeatPenalty: 1.1,
		})
	}

	b, err := buildMessagesWithSystemPrompt(history, getSystemPrompt())
	if err != nil {
		return "", err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.model.Chat(string(b), nil, 512, 0.7, 0.95, 40, 1.1)
}

func (l *LlamaEngine) ChatStream(history []ChatMessage, onToken func(token string) bool) error {
	b, err := buildMessagesWithSystemPrompt(history, getSystemPrompt())
	if err != nil {
		return err
	}

	if onToken == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 增加 MaxTokens 限制到 2048，进一步防止输出被截断
	// 如果需要支持更长的回复，可以继续增加此值，但要注意不要超过总上下文窗口大小
	maxTokens := 2048
	fmt.Printf("[LlamaEngine] Starting stream with MaxTokens: %d\n", maxTokens)
	
	return l.model.ChatStream(string(b), nil, maxTokens, 0.7, 0.95, 40, 1.1, func(piece string) bool {
		if piece == "" {
			return true
		}
		return onToken(piece)
	})
}

func (l *LlamaEngine) ChatWithOptions(history []ChatMessage, opts ChatOptions) (string, error) {
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 512
	}
	if opts.Temperature == 0 {
		opts.Temperature = 0.7
	}
	if opts.TopP == 0 {
		opts.TopP = 0.95
	}
	if opts.TopK == 0 {
		opts.TopK = 40
	}
	if opts.RepeatPenalty == 0 {
		opts.RepeatPenalty = 1.1
	}

	b, err := buildMessagesWithSystemPrompt(history, getSystemPrompt())
	if err != nil {
		return "", err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.model.Chat(string(b), opts.Stop, opts.MaxTokens, opts.Temperature, opts.TopP, opts.TopK, opts.RepeatPenalty)
}

func (l *LlamaEngine) ChatStreamWithOptions(history []ChatMessage, opts ChatOptions, onToken func(token string) bool) error {
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 512
	}
	if opts.Temperature == 0 {
		opts.Temperature = 0.7
	}
	if opts.TopP == 0 {
		opts.TopP = 0.95
	}
	if opts.TopK == 0 {
		opts.TopK = 40
	}
	if opts.RepeatPenalty == 0 {
		opts.RepeatPenalty = 1.1
	}

	b, err := buildMessagesWithSystemPrompt(history, getSystemPrompt())
	if err != nil {
		return err
	}

	if onToken == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.model.ChatStream(string(b), opts.Stop, opts.MaxTokens, opts.Temperature, opts.TopP, opts.TopK, opts.RepeatPenalty, func(piece string) bool {
		if piece == "" {
			return true
		}
		return onToken(piece)
	})
}

func (l *LlamaEngine) Close() {
	if l.model != nil {
		l.model.Close()
	}
}

// SwitchModel 切换模型
func (l *LlamaEngine) SwitchModel(modelPath string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 检查文件是否存在
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model not found at %s", modelPath)
	}

	// 关闭当前模型
	if l.model != nil {
		l.model.Close()
		l.model = nil
	}

	var err error
	// 使用与 Init 相同的参数重新加载模型
	l.model, err = binding.NewLlama(modelPath, 4096, 4, 0)
	if err != nil {
		return fmt.Errorf("failed to load model %s: %v", modelPath, err)
	}

	l.modelPath = modelPath
	fmt.Printf("[LlamaEngine] Switched to model: %s\n", modelPath)
	return nil
}

// ListModels 列出可用模型
func (l *LlamaEngine) ListModels() ([]string, error) {
	// 确定搜索目录：优先使用当前模型所在目录，默认为 "models"
	dir := "models"
	if l.modelPath != "" {
		dir = filepath.Dir(l.modelPath)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		// 如果目录不存在，返回空列表而不是错误，或者视情况而定
		// 这里返回错误以便调试
		return nil, fmt.Errorf("failed to list models in %s: %v", dir, err)
	}

	var models []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".gguf") {
			models = append(models, entry.Name())
		}
	}
	return models, nil
}

// GetModelPath 获取当前模型路径
func (l *LlamaEngine) GetModelPath() string {
	return l.modelPath
}

// GetEmbedding 获取文本的向量表示
func (l *LlamaEngine) GetEmbedding(text string) ([]float32, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.model == nil {
		return nil, fmt.Errorf("model not initialized")
	}

	return l.model.GetEmbedding(text)
}

func NewEngine() Engine {
	return &LlamaEngine{}
}

func streamText(text string, onToken func(token string) bool) error {
	if onToken == nil || text == "" {
		return nil
	}

	const maxChunkBytes = 24
	for i := 0; i < len(text); {
		j := i
		for j < len(text) && j-i < maxChunkBytes {
			_, size := utf8.DecodeRuneInString(text[j:])
			if size <= 0 {
				break
			}
			if j-i+size > maxChunkBytes {
				break
			}
			j += size
		}
		if j == i {
			j = i + 1
		}
		if !onToken(text[i:j]) {
			return nil
		}
		i = j
	}
	return nil
}
