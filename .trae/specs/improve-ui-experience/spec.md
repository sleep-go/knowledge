# 改进 UI 体验 Spec

## Why
用户反馈在流式生成过程中，思考过程的可见性和 Markdown 渲染的时机存在问题。此外，需要确保之前所有的 UI 功能都已充分验证并完成。

## What Changes
- **可折叠的思考块**：`<think>` 内容默认隐藏，提供展开/折叠切换功能。
- **渐进式渲染**：Markdown 内容将在流式传输过程中逐步渲染，而不是等待传输结束。
- **任务验证**：审查并标记之前 Spec 中待验证的任务。

## Impact
- **受影响的 Spec**: `add-chatgpt-ui-features`
- **受影响的代码**: `web/static/script.js`, `web/static/style.css`, `web/static/extra.css`

## ADDED Requirements

### Requirement: 可折叠的思考块
系统 SHALL 默认隐藏 `<think>` 标签的内容，并提供切换机制。

#### Scenario: 默认视图
- **WHEN** 消息包含 `<think>` 标签
- **THEN** 其中的内容被隐藏
- **AND** 显示“思考过程”标题/按钮

#### Scenario: 切换视图
- **WHEN** 用户点击“思考过程”标题
- **THEN** 内容展开或折叠

### Requirement: 渐进式 Markdown 渲染
系统 SHALL 在数据块到达时实时渲染 Markdown 内容。

#### Scenario: 流式传输
- **WHEN** 接收到流式 Token
- **THEN** UI 立即更新渲染已累积的文本的 HTML
- **AND** 优雅处理未完成的 Markdown 语法（例如未闭合的代码块）

## MODIFIED Requirements
### Requirement: UI 验证
验证并关闭 `add-chatgpt-ui-features` 中的待办任务。
