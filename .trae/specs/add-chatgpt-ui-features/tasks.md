# Tasks
- [x] Task 1: 会话边栏 UI 改造
  - [x] SubTask 1.1: 页面布局调整为“左侧会话栏 + 右侧聊天区”
  - [x] SubTask 1.2: 会话列表渲染与选中态、更新时间展示
  - [x] SubTask 1.3: 新建会话按钮与切换会话逻辑
  - [x] SubTask 1.4: 移动端/窄屏降级（可折叠或顶部入口）

- [x] Task 2: Markdown 渲染与安全处理
  - [x] SubTask 2.1: assistant 消息改为 Markdown 渲染（含代码块）
  - [x] SubTask 2.2: 禁止执行任意 HTML/脚本（转义或白名单）
  - [x] SubTask 2.3: 代码块样式与复制体验（最小可用）

- [x] Task 3: 消息编辑与重试能力
  - [x] SubTask 3.1: 后端增加“编辑消息”接口（限定最近一条 user）
  - [x] SubTask 3.2: 后端增加“重试生成”接口（限定最近一次）
  - [x] SubTask 3.3: 前端增加编辑入口、编辑状态、保存后重新生成
  - [x] SubTask 3.4: 前端增加重试入口，并更新 UI（替换或标记）

- [x] Task 4: 自动生成会话标题
  - [x] SubTask 4.1: 后端标题生成流程（回退标题 + 可选模型标题）
  - [x] SubTask 4.2: 会话列表实时刷新标题与更新时间
  - [x] SubTask 4.3: 标题生成触发时机与幂等处理

- [x] Task 5: 验证与回归
  - [x] SubTask 5.1: 手工验证：切换/新建会话、流式输出、Markdown 渲染
  - [x] SubTask 5.2: 手工验证：编辑/重试行为符合 Spec
  - [x] SubTask 5.3: 兼容性验证：旧接口与旧页面不崩溃

# Task Dependencies
- Task 1 依赖现有会话 API（已存在），可直接开始
- Task 2 与 Task 1 可并行，但 UI 结构定型后更易落地
- Task 3 依赖后端接口（SubTask 3.1/3.2）先完成
- Task 4 可独立实现，但需要与 Task 1 的会话列表刷新配合
