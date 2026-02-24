# ChatGPT 风格聊天增强 Spec

## Why
当前聊天体验缺少会话侧边栏、对话标题、Markdown 渲染与“编辑/重试”等关键交互，导致可用性与可读性偏弱，不利于长期对话与知识沉淀。

## What Changes
- 新增左侧会话边栏：展示历史会话列表、支持切换会话与新建会话。
- 自动生成会话标题：创建会话后自动填充标题，并在需要时生成更贴近语义的标题。
- Markdown 渲染：assistant 消息按 Markdown 渲染（含代码块），并保证安全（不执行任意 HTML/脚本）。
- 消息编辑与重试：
  - 支持编辑最近一条 user 消息并重新生成 assistant 回复。
  - 支持对最近一条 user 消息执行“重试”，重新生成 assistant 回复（不修改 user 文本）。
- 后端提供对上述能力的 API（保持现有 API 可继续工作，新增接口优先用于新 UI）。

## Impact
- Affected specs: 会话管理、消息展示、消息生成、流式输出、对外 OpenAI 兼容接口（不要求变更，但需兼容并存）。
- Affected code:
  - 前端：web/index.html、web/static/script.js、web/static/style.css
  - 服务端：internal/server/router.go
  - 数据库：internal/db/sqlite.go
  - 推理：internal/llm（用于标题生成调用）

## ADDED Requirements
### Requirement: 会话边栏
系统 SHALL 提供左侧会话边栏，用于展示与切换会话。

#### Scenario: 列出会话
- **WHEN** 用户打开页面
- **THEN** 页面左侧展示会话列表（按更新时间倒序）
- **AND** 每个会话展示标题与最近更新时间（或相对时间）

#### Scenario: 切换会话
- **WHEN** 用户点击某个会话
- **THEN** 主聊天区域加载该会话消息并滚动到底部
- **AND** 新消息发送到当前会话

#### Scenario: 新建会话
- **WHEN** 用户点击“新建对话”
- **THEN** 系统创建一个新会话并切换到该会话
- **AND** 聊天区域为空（或仅包含欢迎提示）

### Requirement: 自动生成会话标题
系统 SHALL 为每个会话自动生成标题，并在生成失败时提供可用的回退方案。

#### Scenario: 标题回退（无需模型）
- **WHEN** 会话创建后尚无标题或标题为空
- **THEN** 使用第一条 user 消息的前 N 个字符作为标题（N 默认 20，去除换行/多余空格）

#### Scenario: 智能标题（使用模型）
- **WHEN** 会话产生第一条 assistant 回复（或达到可生成标题的条件）
- **THEN** 系统调用本地模型生成短标题（≤ 20 字符，单行，无引号）
- **AND** 将标题写入会话记录

### Requirement: Markdown 渲染
系统 SHALL 将 assistant 消息以 Markdown 方式渲染，并避免 XSS 风险。

#### Scenario: 渲染常见 Markdown
- **WHEN** assistant 回复包含段落、列表、代码块、行内代码、链接
- **THEN** 前端以 Markdown 形式渲染
- **AND** 代码块保持原样并等宽显示

#### Scenario: 安全性
- **WHEN** assistant 回复包含 HTML 标签或脚本片段
- **THEN** 前端不得执行任意脚本或注入 HTML（应进行转义或安全过滤）

### Requirement: 编辑最近一条 user 消息
系统 SHALL 支持编辑最近一条 user 消息并重新生成对应的 assistant 回复。

#### Scenario: 编辑成功
- **GIVEN** 当前会话至少包含一条 user 消息
- **WHEN** 用户点击最近一条 user 消息的“编辑”并保存
- **THEN** 系统更新该 user 消息内容
- **AND** 删除/替换其后紧邻的最后一条 assistant 回复（或将其标记为过期）
- **AND** 触发重新生成并以流式方式显示新的 assistant 回复

### Requirement: 重试最近一次生成
系统 SHALL 支持对最近一条 user 消息执行“重试”，重新生成 assistant 回复。

#### Scenario: 重试成功
- **GIVEN** 最近一条消息为 user，且已有至少一条 assistant 回复
- **WHEN** 用户点击“重试”
- **THEN** 系统在不修改 user 消息的前提下重新生成 assistant 回复
- **AND** 替换最后一条 assistant 回复（或追加并在 UI 中标记为重试结果）

## MODIFIED Requirements
### Requirement: 现有聊天接口兼容
系统 SHALL 保持现有 `/api/chat`、`/api/chat/stream`、`/api/history` 等接口可用（用于兼容旧前端或调试），同时新 UI 优先使用会话化接口。

## REMOVED Requirements
无

