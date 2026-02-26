# 任务列表

- [ ] 任务 1：后端实现
  - [ ] 子任务 1.1：在 `internal/db/sqlite.go` 中添加 `ResetKnowledgeBase` 函数，用于删除 `KnowledgeBaseFile` 和 `KnowledgeBaseChunk` 表中的所有行。
  - [ ] 子任务 1.2：在 `internal/server/router.go` 中添加 `/api/kb/reset` 接口，调用 `ResetKnowledgeBase`。

- [ ] 任务 2：前端实现
  - [ ] 子任务 2.1：在 `web/static/index.html` 的设置模态框中添加“重置”按钮。
  - [ ] 子任务 2.2：在 `web/static/script.js` 中为重置按钮添加点击事件监听器，调用重置 API 并刷新文件列表。

# 任务依赖
- 任务 2 依赖于 任务 1。
