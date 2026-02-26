# 任务列表

- [x] 任务 1: 实现严格的标题净化
  - [x] 子任务 1.1: 修改 `internal/server/utils.go` 中的 `sanitizeTitle`，使用正则或严格过滤来移除标点和特殊符号。

- [x] 任务 2: 实现自动创建会话
  - [x] 子任务 2.1: 修改 `web/static/script.js` 中的 `sendMessage`，检测 `currentConversationId` 是否为空。
  - [x] 子任务 2.2: 如果为空，先调用 `createConversation`（并等待其完成），然后再发送消息。

- [x] 任务 3: 改进知识库智能度
  - [x] 子任务 3.1: 修改 `internal/kb/knowledge_base.go` 中的 `splitText` 以支持分片重叠（例如大小 500，重叠 100）。
  - [x] 子任务 3.2: 更新 `internal/server/utils.go` 中的 `augmentHistoryWithKB`，增加 `maxKBChars`（至 3000）和获取数量（至 5）。

- [x] 任务 4: 实现文件上传
  - [x] 子任务 4.1: 在 `internal/server/handler_kb.go` 中添加 `UploadKBFile` 处理程序，处理 multipart 文件上传，保存到 KB 文件夹，并触发处理。
  - [x] 子任务 4.2: 在 `internal/server/router.go` 中注册 `POST /api/kb/upload`。
  - [x] 子任务 4.3: 在 `web/index.html`（设置模态框或 KB 区域内）添加文件输入框和“上传”按钮。
  - [x] 子任务 4.4: 在 `web/static/script.js` 中添加 JavaScript 逻辑，处理文件选择并通过 API 上传。

# 任务依赖
- 任务 4 依赖 任务 3（部分依赖，因为上传的文件应使用新的分片策略）。
- 任务 1、2 和 3 基本独立。
