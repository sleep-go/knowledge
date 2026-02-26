package server

import (
	"knowledge/internal/kb"
	"knowledge/internal/llm"
)

type Server struct {
	engine llm.Engine
	kbase  *kb.KnowledgeBase
}

func NewServer(engine llm.Engine, kbase *kb.KnowledgeBase) *Server {
	return &Server{
		engine: engine,
		kbase:  kbase,
	}
}
