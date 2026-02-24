//go:build cgo

package binding

/*
#cgo CXXFLAGS: -std=c++17 -I${SRCDIR}/../../llama.cpp/common -I${SRCDIR}/../../llama.cpp/include -I${SRCDIR}/../../llama.cpp/ggml/include -I${SRCDIR}/../../llama.cpp/vendor
#cgo LDFLAGS: -L${SRCDIR}/../../llama.cpp/build/common -lcommon -L${SRCDIR}/../../llama.cpp/build/src -lllama -L${SRCDIR}/../../llama.cpp/build/ggml/src -lggml -lggml-base -lggml-cpu -L${SRCDIR}/../../llama.cpp/build/ggml/src/ggml-blas -lggml-blas -lstdc++
#cgo darwin LDFLAGS: -L${SRCDIR}/../../llama.cpp/build/ggml/src/ggml-metal -lggml-metal -framework Accelerate -framework Foundation -framework Metal
#include <stdlib.h>
#include "binding.h"
*/
import "C"
import (
	"fmt"
	"runtime/cgo"
	"strings"
	"unsafe"
)

type Llama struct {
	ctx unsafe.Pointer
}

func NewLlama(modelPath string, nCtx int, nThreads int, nGpuLayers int) (*Llama, error) {
	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))

	ctx := C.llama_binding_load_model(cPath, C.int(nCtx), C.int(nThreads), C.int(nGpuLayers))
	if ctx == nil {
		return nil, fmt.Errorf("failed to load model: %s", modelPath)
	}

	return &Llama{ctx: ctx}, nil
}

func (l *Llama) Chat(messagesJSON string, stopTokens []string, nPredict int, temp float32, topP float32, topK int, repeatPenalty float32) (string, error) {
	cMessages := C.CString(messagesJSON)
	defer C.free(unsafe.Pointer(cMessages))

	var cStop *C.char
	if len(stopTokens) > 0 {
		joined := strings.Join(stopTokens, "\x1f")
		cStop = C.CString(joined)
		defer C.free(unsafe.Pointer(cStop))
	}

	result := C.llama_binding_chat(
		l.ctx,
		cMessages,
		cStop,
		C.int(nPredict),
		C.float(temp),
		C.float(topP),
		C.int(topK),
		C.float(repeatPenalty),
	)

	if result == nil {
		return "", fmt.Errorf("chat failed")
	}
	defer C.llama_binding_free_result(result)

	return C.GoString(result), nil
}

type TokenCallback func(token string) bool

func (l *Llama) ChatStream(messagesJSON string, stopTokens []string, nPredict int, temp float32, topP float32, topK int, repeatPenalty float32, cb TokenCallback) error {
	cMessages := C.CString(messagesJSON)
	defer C.free(unsafe.Pointer(cMessages))

	var cStop *C.char
	if len(stopTokens) > 0 {
		joined := strings.Join(stopTokens, "\x1f")
		cStop = C.CString(joined)
		defer C.free(unsafe.Pointer(cStop))
	}

	h := cgo.NewHandle(cb)
	defer h.Delete()

	rc := C.llama_binding_chat_stream(
		l.ctx,
		cMessages,
		cStop,
		C.int(nPredict),
		C.float(temp),
		C.float(topP),
		C.int(topK),
		C.float(repeatPenalty),
		C.uintptr_t(h),
	)
	if rc != 0 {
		return fmt.Errorf("chat stream failed")
	}
	return nil
}

func (l *Llama) Close() {
	if l.ctx != nil {
		C.llama_binding_free_model(l.ctx)
		l.ctx = nil
	}
}

//export llama_binding_go_on_token
func llama_binding_go_on_token(cbHandle C.uintptr_t, tokenPiece *C.char) C.int {
	h := cgo.Handle(cbHandle)
	v := h.Value()
	cb, ok := v.(TokenCallback)
	if !ok {
		return 0
	}
	if cb(C.GoString(tokenPiece)) {
		return 1
	}
	return 0
}
