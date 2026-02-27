package server

import (
	"knowledge/internal/kb"
	"knowledge/internal/llm"
	"sync"
)

type Server struct {
	engine   llm.Engine
	kbase    *kb.KnowledgeBase
	engineMu sync.Mutex
}

func NewServer(engine llm.Engine, kbase *kb.KnowledgeBase) *Server {
	return &Server{
		engine: engine,
		kbase:  kbase,
	}
}

func (s *Server) withEngineLocked(fn func() error) error {
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	return fn()
}
