# Tasks
- [x] Task 1: 准备 llama.cpp 源码
  - [x] SubTask 1.1: 移除现有的 `go-llama.cpp` 目录。
  - [x] SubTask 1.2: 拉取官方 `llama.cpp` 最新代码到 `llama.cpp` 目录。
  - [x] SubTask 1.3: 验证 `llama.cpp` 可以编译通过 (生成 libllama.a)。
- [x] Task 2: 创建 CGO 绑定 (`internal/binding`)
  - [x] SubTask 2.1: 创建 `binding.go`，定义 CGO 头文件引用 (`#cgo LDFLAGS` 等)。
  - [x] SubTask 2.2: 实现模型加载函数 (`LoadModel`)。
  - [x] SubTask 2.3: 实现推理函数 (`Predict` / `Eval`)。
  - [x] SubTask 2.4: 实现 Token 处理函数 (`Tokenize`, `Detokenize`)。
- [x] Task 3: 重构 LlamaEngine
  - [x] SubTask 3.1: 修改 `internal/llm/llama.go`，使用新的 `binding` 包替换 `go-skynet`。
  - [x] SubTask 3.2: 确保 `ServerEngine` (如果保留) 仍然可用，或者将 `LlamaEngine` 恢复为默认引擎。
- [x] Task 4: 更新构建系统
  - [x] SubTask 4.1: 更新 `Makefile` 或构建脚本，添加编译 `llama.cpp` 的步骤。
  - [x] SubTask 4.2: 清理 `go.mod`。

# Task Dependencies
- Task 2 依赖 Task 1
- Task 3 依赖 Task 2
