//go:build !cgo

package binding

import (
	"fmt"
	"unsafe"
)

type Llama struct {
	ctx unsafe.Pointer
}

func NewLlama(modelPath string, nCtx int, nThreads int, nGpuLayers int) (*Llama, error) {
	return nil, fmt.Errorf("CGO is disabled, Llama binding is unavailable on this platform/configuration")
}

func (l *Llama) Chat(messagesJSON string, stopTokens []string, nPredict int, temp float32, topP float32, topK int, repeatPenalty float32) (string, error) {
	return "", fmt.Errorf("CGO is disabled, Llama binding is unavailable")
}

type TokenCallback func(token string) bool

func (l *Llama) ChatStream(messagesJSON string, stopTokens []string, nPredict int, temp float32, topP float32, topK int, repeatPenalty float32, cb TokenCallback) error {
	return fmt.Errorf("CGO is disabled, Llama binding is unavailable")
}

func (l *Llama) Close() {
}
