package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
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
	// 获取可执行文件路径，用于处理相对路径
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal("获取可执行文件路径失败:", err)
	}
	exeDir := filepath.Dir(exePath)

	// 路径解析函数：如果路径不是绝对路径，则尝试在不同位置查找
	// 在 macOS .app 包中，资源通常位于 ../Resources 目录
	// 标准结构：
	// App.app/Contents/MacOS/binary
	// App.app/Contents/Resources/models
	resolvePath := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}

		// 1. 检查相对于当前工作目录的路径（开发模式）
		if _, err := os.Stat(p); err == nil {
			return p
		}

		// 2. 检查相对于可执行文件目录的路径（标准二进制部署）
		pathExe := filepath.Join(exeDir, p)
		if _, err := os.Stat(pathExe); err == nil {
			return pathExe
		}

		// 3. 检查相对于 ../Resources 的路径（macOS App Bundle）
		// exeDir 是 Contents/MacOS，所以 ../Resources 是 Contents/Resources
		pathResources := filepath.Join(exeDir, "..", "Resources", p)
		if _, err := os.Stat(pathResources); err == nil {
			return pathResources
		}

		// 如果在所有位置都找不到，返回原始路径（稍后自然失败或创建新文件）
		return p
	}

	// 默认路径（相对路径）
	defaultModelPath := "models/LiquidAI_LFM2.5-1.2B-Instruct-GGUF_LFM2.5-1.2B-Instruct-Q4_K_M.gguf"
	defaultDbPath := "data/knowledge.db"

	// 命令行参数
	port := flag.String("port", "8081", "服务器端口")
	modelPath := flag.String("model", defaultModelPath, "GGUF 模型路径")
	dbPath := flag.String("db", defaultDbPath, "SQLite 数据库路径")
	flag.Parse()

	// 解析路径
	finalModelPath := resolvePath(*modelPath)
	finalDbPath := resolvePath(*dbPath)

	fmt.Printf("可执行文件: %s\n", exePath)
	fmt.Printf("解析后的模型路径: %s\n", finalModelPath)
	fmt.Printf("解析后的数据库路径: %s\n", finalDbPath)

	// 确保数据库文件所在目录存在
	if _, err := os.Stat(finalDbPath); os.IsNotExist(err) {
		// 如果解析后的路径仍然是相对路径（未找到），使用当前工作目录
		// 这样可以确保即使在使用 go run 时，数据库文件也能在正确位置创建
		if !filepath.IsAbs(finalDbPath) {
			// 开发模式下使用当前工作目录
			finalDbPath = filepath.Join("data", "knowledge.db")
			fmt.Printf("数据库未找到，默认创建位置: %s\n", finalDbPath)
		}
	}

	// 确保数据库目录存在
	dbDir := filepath.Dir(finalDbPath)
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			log.Fatal("创建数据库目录失败:", err)
		}
		fmt.Printf("创建数据库目录: %s\n", dbDir)
	}

	// 查找模型文件
	if _, err := os.Stat(finalModelPath); os.IsNotExist(err) {
		// 如果默认或指定路径不存在，尝试在可能的位置查找 .gguf 文件

		// 搜索位置
		searchDirs := []string{
			"models",
			filepath.Join(exeDir, "models"),
			filepath.Join(exeDir, "..", "Resources", "models"),
		}

		found := false
		// 直接搜索所有模型
		for _, dir := range searchDirs {
			pattern := filepath.Join(dir, "*.gguf")
			files, _ := filepath.Glob(pattern)
			if len(files) > 0 {
				finalModelPath = files[0]
				fmt.Printf("自动检测到模型: %s\n", finalModelPath)
				found = true
				break
			}
		}

		if !found {
			fmt.Printf("警告: 在 %s 未找到模型，且在搜索路径中未找到 .gguf 文件。\n", finalModelPath)
		}
	}

	// 初始化数据库
	db.InitDB(finalDbPath)

	// 禁用Metal后端，使用CPU后端
	os.Setenv("GGML_METAL", "0")
	os.Setenv("GGML_METAL_PATH", "")

	// 初始化LLM引擎（使用原生CGO引擎）
	var engine llm.Engine = llm.NewEngine()

	// 初始化知识库
	kbase := kb.NewKnowledgeBase()

	// 尝试初始化引擎。如果失败（例如模型路径错误或绑定错误），清晰记录日志
	if err := engine.Init(finalModelPath); err != nil {
		log.Printf("初始化LlamaEngine失败，模型 '%s': %v", finalModelPath, err)
		// 如果失败，直接退出，因为我们移除了Mock fallback
		log.Fatal("初始化LLM引擎失败")
	} else {
		log.Printf("成功初始化LlamaEngine，模型: %s", finalModelPath)
	}

	// 将初始化后的引擎赋值给全局变量，供知识库使用
	llm.CurrentEngine = engine

	// 处理退出信号：优雅关闭 HTTP + 取消 KB 任务 + 释放引擎资源
	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 设置路由
	r := server.SetupRouter(web.StaticFiles, engine, kbase)

	// 自动打开浏览器
	go func() {
		// 立即打开浏览器，不等待服务器启动
		// 这样可以避免因为服务器启动缓慢而导致浏览器无法打开
		url := fmt.Sprintf("http://localhost:%s", *port)
		openBrowser(url)
	}()

	// 启动服务器
	fmt.Printf("正在端口 %s 启动网络服务器...\n", *port)
	srv := &http.Server{
		Addr:    ":" + *port,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-appCtx.Done()
	fmt.Println("\n正在关闭...")

	// 先取消 KB 后台任务
	kbase.Close()

	// 关闭 HTTP（给正在进行的请求一个收尾窗口）
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)

	// 最后关闭模型引擎
	if closer, ok := engine.(interface{ Close() }); ok {
		closer.Close()
	}
}

// openBrowser 打开浏览器函数
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
		err = fmt.Errorf("不支持的平台")
	}
	if err != nil {
		log.Printf("打开浏览器失败: %v", err)
	}
}
