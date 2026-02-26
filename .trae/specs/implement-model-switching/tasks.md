# 任务列表

- [x] 任务 1: 更新 LLM 引擎接口和实现
  - [x] 子任务 1.1: 在 `internal/llm/engine.go` 的 `Engine` 接口中添加 `SwitchModel(modelPath string) error` 和 `ListModels() ([]string, error)`。
  - [x] 子任务 1.2: 在 `LlamaEngine` (`internal/llm/llama.go`) 中实现 `SwitchModel`，包含互斥锁和清理逻辑。
  - [x] 子任务 1.3: 在 `LlamaEngine` (`internal/llm/llama.go`) 中实现 `ListModels` 函数，扫描 `models/` 目录下的 `.gguf` 文件。

- [x] 任务 2: 实现模型切换 API
  - [x] 子任务 2.1: 在 `internal/server/router.go` 中添加 `GET /api/models` 处理程序以列出模型。
  - [x] 子任务 2.2: 在 `internal/server/router.go` 中添加 `POST /api/models/select` 处理程序以调用 `SwitchModel`。

- [x] 任务 3: 验证功能
  - [x] 子任务 3.1: 验证 `ListModels` 返回正确的文件。
  - [x] 子任务 3.2: 验证 `SwitchModel` 成功切换模型。
  - [x] 子任务 3.3: 验证并发访问（切换期间聊天）被优雅处理（等待或报错）。

# 任务依赖
- 任务 2 依赖 任务 1。
- 任务 3 依赖 任务 2。
