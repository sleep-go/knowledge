package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"knowledge/internal/db"
	"knowledge/internal/kb"
	"knowledge/internal/llm"
	"knowledge/internal/server"
	"knowledge/web"
)

func main() {
	port := flag.String("port", "8081", "Server port") // Use 8081 for web, as 8080 is for llama-server
	modelPath := flag.String("model", "models/llama-2-7b-chat.gguf", "Path to GGUF model")
	dbPath := flag.String("db", "data/knowledge.db", "Path to SQLite database")
	// useMock := flag.Bool("mock", false, "Force use of Mock engine") // Removed
	flag.Parse()

	// Find the model file
	finalModelPath := *modelPath
	if _, err := os.Stat(finalModelPath); os.IsNotExist(err) {
		// If default or specified path doesn't exist, try to find any .gguf in models/
		files, _ := filepath.Glob("models/*.gguf")
		if len(files) > 0 {
			finalModelPath = files[0]
			fmt.Printf("Auto-detected model: %s\n", finalModelPath)
		} else {
			fmt.Printf("Warning: Model not found at %s and no .gguf files in models/ directory.\n", finalModelPath)
		}
	}

	// Initialize Database
	db.InitDB(*dbPath)

	// Initialize LLM Engine (Use Native CGO Engine)
	var engine llm.Engine = llm.NewEngine()

	// Initialize Knowledge Base
	kbase := kb.NewKnowledgeBase()

	// Try to initialize. If it fails (e.g. model path wrong, or binding error),
	// we should log it clearly.
	if err := engine.Init(finalModelPath); err != nil {
		log.Printf("Error initializing LlamaEngine with model '%s': %v", finalModelPath, err)
		// If failed, we just exit or panic because we removed Mock fallback
		log.Fatal("Failed to initialize LLM engine")
	} else {
		log.Printf("Successfully initialized LlamaEngine with model: %s", finalModelPath)
	}

	// Ensure cleanup on exit
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\nShutting down...")
		if closer, ok := engine.(interface{ Close() }); ok {
			closer.Close()
		}
		os.Exit(0)
	}()

	// Setup Router
	r := server.SetupRouter(web.StaticFiles, engine, kbase)

	// Open Browser automatically
	go func() {
		time.Sleep(2 * time.Second) // Wait for server to start
		url := fmt.Sprintf("http://localhost:%s", *port)
		openBrowser(url)
	}()

	// Start Server
	fmt.Printf("Starting web server on port %s...\n", *port)
	if err := r.Run(":" + *port); err != nil {
		log.Fatal(err)
	}
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Printf("Failed to open browser: %v", err)
	}
}
