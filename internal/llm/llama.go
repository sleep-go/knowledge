package llm

import (
	"encoding/json"
	"fmt"
	"os"
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
	l.model, err = binding.NewLlama(modelPath, 2048, 4, 0)
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

	return l.model.ChatStream(string(b), nil, 512, 0.7, 0.95, 40, 1.1, func(piece string) bool {
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
