# 任务列表 (Tasks)

- [x] 任务 1: 添加多格式解析依赖库
  - [x] 子任务 1.1: 添加 PDF 解析库 `github.com/ledongthuc/pdf`
  - [x] 子任务 1.2: 添加 Word 解析库 `github.com/nguyenthenguyen/docx`
  - [x] 子任务 1.3: 添加 Excel 解析库 `github.com/xuri/excelize/v2`
  - [x] 子任务 1.4: 执行 `go mod tidy`

- [x] 任务 2: 扩展 `internal/kb` 解析能力
  - [x] 子任务 2.1: 在 `knowledge_base.go` 中更新 `isSupportedExt` 以支持新格式
  - [x] 子任务 2.2: 实现 `extractTextFromPDF` 函数
  - [x] 子任务 2.3: 实现 `extractTextFromDocx` 函数
  - [x] 子任务 2.4: 实现 `extractTextFromXlsx` 函数
  - [x] 子任务 2.5: 重构 `processFile` 函数，根据后缀分发到不同的解析器

- [x] 任务 3: 验证与测试
  - [x] 子任务 3.1: 准备包含 PDF, Docx, Xlsx 的测试文件夹
  - [x] 子任务 3.2: 运行服务并触发同步，检查控制台日志是否有报错
  - [x] 子任务 3.3: 检查 SQLite 数据库中的 `knowledge_base_chunks` 表，确认内容已成功导入
  - [x] 子任务 3.4: 在 UI 界面测试 AI 是否能根据这些文档内容回答问题

# 任务依赖 (Task Dependencies)
- [任务 2] 依赖于 [任务 1]
- [任务 3] 依赖于 [任务 2]
