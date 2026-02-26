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

	"knowledge/internal/db"
	"knowledge/internal/kb"
	"knowledge/internal/llm"
	"knowledge/internal/server"
	"knowledge/web"
)

func main() {
	// Determine executable directory for relative paths
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal("Failed to get executable path:", err)
	}
	exeDir := filepath.Dir(exePath)

	// Helper to resolve paths relative to executable if they are not absolute
	// In macOS .app bundle, resources are usually in ../Resources relative to the binary in Contents/MacOS
	// But standard structure is:
	// App.app/Contents/MacOS/binary
	// App.app/Contents/Resources/models
	// So we might need to check parent directory too.
	resolvePath := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}

		// 1. Check relative to CWD (development mode)
		if _, err := os.Stat(p); err == nil {
			return p
		}

		// 2. Check relative to executable directory (standard binary deployment)
		pathExe := filepath.Join(exeDir, p)
		if _, err := os.Stat(pathExe); err == nil {
			return pathExe
		}

		// 3. Check relative to ../Resources (macOS App Bundle)
		// exeDir is Contents/MacOS, so ../Resources is Contents/Resources
		pathResources := filepath.Join(exeDir, "..", "Resources", p)
		if _, err := os.Stat(pathResources); err == nil {
			return pathResources
		}

		// Default to relative path if not found anywhere (let it fail naturally later or create new)
		return p
	}

	// Default paths (relative)
	defaultModelPath := "models/LiquidAI_LFM2.5-1.2B-Instruct-GGUF_LFM2.5-1.2B-Instruct-Q4_K_M.gguf"
	defaultDbPath := "data/knowledge.db"

	port := flag.String("port", "8081", "Server port")
	modelPath := flag.String("model", defaultModelPath, "Path to GGUF model")
	dbPath := flag.String("db", defaultDbPath, "Path to SQLite database")
	flag.Parse()

	// Resolve paths
	finalModelPath := resolvePath(*modelPath)
	finalDbPath := resolvePath(*dbPath)

	fmt.Printf("Executable: %s\n", exePath)
	fmt.Printf("Resolved Model Path: %s\n", finalModelPath)
	fmt.Printf("Resolved DB Path: %s\n", finalDbPath)

	// Ensure data directory exists for DB if we are creating it
	if _, err := os.Stat(finalDbPath); os.IsNotExist(err) {
		// If resolved path is still the relative one (not found), make it relative to exeDir or Resources?
		// If it's "data/knowledge.db" and not found, resolvePath returned "data/knowledge.db".
		// We should probably default to creating it next to binary or in Resources if packaged.

		// Let's refine logic: if resolvePath returned relative path, it means it wasn't found.
		// We should construct an absolute path for creation.
		if !filepath.IsAbs(finalDbPath) {
			// Check if we are in a bundle
			resourcesDir := filepath.Join(exeDir, "..", "Resources")
			if _, err := os.Stat(resourcesDir); err == nil {
				// We are likely in a bundle, try to use Resources (though writing there is discouraged, user asked for it)
				finalDbPath = filepath.Join(resourcesDir, *dbPath)
			} else {
				// Standard binary
				finalDbPath = filepath.Join(exeDir, *dbPath)
			}
			fmt.Printf("DB not found, defaulting to create at: %s\n", finalDbPath)
		}
	}

	// Find the model file
	if _, err := os.Stat(finalModelPath); os.IsNotExist(err) {
		// If default or specified path doesn't exist, try to find any .gguf in models/ relative to likely locations

		// Search locations
		searchDirs := []string{
			"models",
			filepath.Join(exeDir, "models"),
			filepath.Join(exeDir, "..", "Resources", "models"),
		}

		found := false
		for _, dir := range searchDirs {
			pattern := filepath.Join(dir, "*.gguf")
			files, _ := filepath.Glob(pattern)
			if len(files) > 0 {
				finalModelPath = files[0]
				fmt.Printf("Auto-detected model: %s\n", finalModelPath)
				found = true
				break
			}
		}

		if !found {
			fmt.Printf("Warning: Model not found at %s and no .gguf files found in search paths.\n", finalModelPath)
		}
	}

	// Initialize Database
	db.InitDB(finalDbPath)

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
		// 立即打开浏览器，不等待服务器启动
		// 这样可以避免因为服务器启动缓慢而导致浏览器无法打开
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
