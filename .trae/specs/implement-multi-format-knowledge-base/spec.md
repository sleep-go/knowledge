# 多格式本地知识库系统设计文档 (implement-multi-format-knowledge-base)

## 为什么 (Why)
目前的知识库系统仅支持纯文本格式（.txt, .md, .go 等）。为了让系统更具实用性，需要支持用户常见的办公文档格式，包括 PDF、Word (.docx) 和 Excel (.xlsx)。这使得用户可以将现有的文档资料直接导入知识库进行学习和检索。

## 变更内容 (What Changes)
- 引入多格式文档解析库。
- 扩展后端 `knowledge_base.go` 的解析逻辑，支持 PDF、Word 和 Excel。
- 更新数据库支持及前端文件列表显示（已在基础版中实现，需确保兼容性）。
- **BREAKING**: 无。

## 影响范围 (Impact)
- 受影响的组件：`internal/kb` (知识库处理引擎)。
- 受影响的代码：`internal/kb/knowledge_base.go`。
- 依赖项：需要添加处理 PDF、Docx 和 Excel 的 Go 依赖库。

## 新增需求 (ADDED Requirements)
### 需求：支持多格式文件解析
系统必须能够从指定的本地文件夹中识别并提取以下格式的内容：
- **PDF**: 提取文本内容。
- **Word (.docx)**: 提取文档正文。
- **Excel (.xlsx)**: 提取工作表中的文本数据。
- **TXT**: 保持原有的文本读取能力。

#### 场景：成功同步多格式文件
- **当** 用户在设置中指定包含 PDF/Word/Excel 的文件夹并点击“开始同步”。
- **那么** 系统应能正确解析这些文件，将其内容切片并存入 SQLite 数据库。
- **并且** 在对话时，AI 能检索到这些文件中的信息并据此回答。

## 修改需求 (MODIFIED Requirements)
### 需求：文件同步逻辑
- **修改前**：仅支持 `isSupportedExt` 定义的文本后缀。
- **修改后**：扩展 `isSupportedExt` 以包含 `.pdf`, `.docx`, `.xlsx`。根据文件后缀调用不同的解析器。

## 移除需求 (REMOVED Requirements)
无。
