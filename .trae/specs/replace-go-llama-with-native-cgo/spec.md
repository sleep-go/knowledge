# Replace go-llama.cpp with Native CGO Spec

## Why
目前项目依赖 `github.com/go-skynet/go-llama.cpp`，该库可能更新滞后或维护困难。通过直接集成 `llama.cpp` 并自行编写 CGO 绑定，可以获得更高的灵活性、最新的功能支持（如 Qwen2 架构），以及更好的构建控制。

## What Changes
- **移除** `go-llama.cpp` 依赖。
- **集成** `llama.cpp` 官方代码（作为子目录或 submodule）。
- **新增** `internal/binding` 包，包含自定义的 CGO 绑定代码，直接调用 `llama.cpp` 的 C API。
- **修改** `internal/llm/llama.go`，使其从使用 `go-skynet` 切换到使用本地的 `internal/binding`。
- **更新** 构建流程（Makefile 或构建脚本），确保在编译 Go 项目前先编译 `llama.cpp` 的静态库。

## Impact
- Affected specs: 无
- Affected code: 
  - `go.mod` (移除依赖)
  - `internal/llm/llama.go` (重构)
  - `internal/binding/` (新增)
  - `llama.cpp/` (新增/更新)

## ADDED Requirements
### Requirement: 自定义 CGO 绑定
系统必须包含一个 Go 包，能够：
1. 加载 GGUF 模型。
2. 创建 Context。
3. 执行推理 (Completion/Chat)。
4. 处理 Tokenizer (编码/解码)。

#### Scenario: 模型加载
- **WHEN** 调用 `binding.NewLlama`
- **THEN** 成功加载模型文件并返回句柄。

### Requirement: 支持新架构
通过更新 `llama.cpp` 源码，确保支持 Qwen2 等新模型架构（解决之前 `unknown model architecture: 'qwen2'` 的问题）。

## MODIFIED Requirements
### Requirement: LlamaEngine 实现
`LlamaEngine` 不再依赖外部库，而是调用内部的绑定实现。

## REMOVED Requirements
### Requirement: go-llama.cpp 依赖
移除 `go.mod` 中的相关依赖。
