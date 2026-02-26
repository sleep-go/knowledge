# 任务列表

- [x] 任务 1: UI 验证与清理
  - [x] 子任务 1.1: 验证 `add-chatgpt-ui-features/tasks.md` 中的任务并标记完成。
  - [x] 子任务 1.2: 检查其他 Spec 中是否有未完成的 UI 任务。

- [x] 任务 2: 实现可折叠的思考块
  - [x] 子任务 2.1: 更新 `web/static/extra.css` 添加折叠/展开样式。
  - [x] 子任务 2.2: 更新 `web/static/script.js` 渲染 `<think>` 块，支持标题点击和内容显示切换。
  - [x] 子任务 2.3: 确保切换逻辑（点击处理）正常工作。

- [x] 任务 3: 渐进式 Markdown 渲染
  - [x] 子任务 3.1: 修改 `web/static/script.js` 在流式接收循环中实时调用 `renderMarkdown`。
  - [x] 子任务 3.2: 处理潜在的闪烁或性能问题（例如：频繁渲染大量文本）。

# 任务依赖
- 任务 1 是独立的。
- 任务 2 和 3 可以并行，但可能涉及同一文件 (`script.js`)。
