# Knowledge - macOS 客户端应用实现计划

## 项目概述
实现一个与现有 web 页面功能相同的 macOS 客户端应用，基于现有的 Go 和 llama.cpp 架构。

## 任务分解与优先级

### [x] 任务 1: 分析现有 Web 功能
- **Priority**: P0
- **Depends On**: None
- **Description**:
  - 详细分析现有 web 页面的所有功能
  - 确保 macOS 应用包含所有相同的功能点
- **Success Criteria**:
  - 完整的功能清单，包括所有 web 页面的功能
- **Test Requirements**:
  - `programmatic` TR-1.1: 功能清单完整覆盖 web 页面的所有功能
  - `human-judgement` TR-1.2: 功能分析准确，无遗漏
- **Notes**: 需要详细分析 index.html 和 script.js 文件

### [x] 任务 2: 完善打包脚本
- **Priority**: P0
- **Depends On**: Task 1
- **Description**:
  - 完善现有的 build_app.sh 脚本
  - 确保所有必要的资源文件（web 静态文件、模型、数据）都被正确打包
  - 优化应用包结构
- **Success Criteria**:
  - 打包脚本能够正确创建完整的 macOS 应用包
  - 所有必要的资源文件都被包含
- **Test Requirements**:
  - `programmatic` TR-2.1: 脚本执行无错误
  - `programmatic` TR-2.2: 生成的应用包结构完整
  - `human-judgement` TR-2.3: 脚本代码清晰可维护
- **Notes**: 需要确保 web 静态文件被正确复制到应用包中

### [x] 任务 3: 实现资源路径处理
- **Priority**: P0
- **Depends On**: Task 2
- **Description**:
  - 修改应用代码，使其能够正确处理 macOS 应用包中的资源路径
  - 确保应用能够找到并加载 web 静态文件、模型文件等
- **Success Criteria**:
  - 应用能够正确加载所有资源文件
  - 路径处理逻辑在 macOS 环境下正常工作
- **Test Requirements**:
  - `programmatic` TR-3.1: 应用能够启动并加载 web 界面
  - `programmatic` TR-3.2: 所有资源文件都能被正确加载
- **Notes**: 需要考虑 macOS 应用包的特殊路径结构

### [x] 任务 4: 测试功能完整性
- **Priority**: P1
- **Depends On**: Task 3
- **Description**:
  - 测试 macOS 应用的所有功能
  - 确保与 web 页面功能完全一致
  - 测试文件上传、知识库管理、聊天功能等
- **Success Criteria**:
  - 所有功能都能正常工作
  - 与 web 页面功能一致
- **Test Requirements**:
  - `programmatic` TR-4.1: 应用能够正常启动和运行
  - `programmatic` TR-4.2: 所有功能测试通过
  - `human-judgement` TR-4.3: 用户体验与 web 页面一致
- **Notes**: 需要测试各种边缘情况

### [x] 任务 5: 优化应用体验
- **Priority**: P2
- **Depends On**: Task 4
- **Description**:
  - 优化 macOS 应用的用户体验
  - 添加应用图标
  - 确保应用在 macOS 下的外观和行为符合系统规范
- **Success Criteria**:
  - 应用具有良好的 macOS 原生体验
  - 图标和界面美观
- **Test Requirements**:
  - `human-judgement` TR-5.1: 应用外观美观，符合 macOS 设计规范
  - `human-judgement` TR-5.2: 用户体验流畅
- **Notes**: 可以添加自定义应用图标

### [x] 任务 6: 更新文档
- **Priority**: P2
- **Depends On**: Task 5
- **Description**:
  - 更新 README.md 文件，添加 macOS 应用的使用说明
  - 提供详细的打包和使用指南
- **Success Criteria**:
  - 文档完整，包含所有必要的信息
  - 指南清晰易懂
- **Test Requirements**:
  - `human-judgement` TR-6.1: 文档内容完整
  - `human-judgement` TR-6.2: 指南清晰易懂
- **Notes**: 需要包含打包步骤和应用使用说明

## 技术实现要点

1. **资源打包**:
   - 确保 web 静态文件（html、css、js）被正确复制到应用包中
   - 确保模型文件和数据文件的路径处理正确

2. **路径处理**:
   - 在 macOS 应用中，需要使用 `NSBundle` 来获取资源路径
   - 确保应用能够正确找到 web 静态文件和其他资源

3. **功能一致性**:
   - 确保 macOS 应用包含 web 页面的所有功能
   - 测试所有功能点，确保与 web 页面行为一致

4. **用户体验**:
   - 确保应用在 macOS 下的外观和行为符合系统规范
   - 优化启动速度和响应性能

## 预期成果

- 一个功能完整的 macOS 客户端应用
- 与现有 web 页面功能完全一致
- 良好的用户体验和美观的界面
- 详细的文档和使用指南