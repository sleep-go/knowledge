# Knowledge 应用改进 - 实现计划

## [x] Task 1: 数据库打包优化
- **Priority**: P0
- **Depends On**: None
- **Description**:
  - 修改 build_app.sh 脚本，确保打包时只包含表结构，不包含数据
  - 在打包前创建一个新的空数据库文件，只包含表结构
  - 确保应用启动时能正确初始化数据库
- **Acceptance Criteria Addressed**: AC-1
- **Test Requirements**:
  - `programmatic` TR-1.1: 打包脚本执行后，生成的数据库文件只包含表结构
  - `programmatic` TR-1.2: 应用启动时能正确初始化数据库
- **Notes**: 可以通过在打包前创建一个新的空数据库文件来实现

## [x] Task 2: 系统提示配置
- **Priority**: P0
- **Depends On**: None
- **Description**:
  - 在系统设置界面添加 system_prompt 配置选项
  - 修改后端代码，支持读取和保存 system_prompt 配置
  - 确保应用启动时能正确加载用户配置的 system_prompt
- **Acceptance Criteria Addressed**: AC-2
- **Test Requirements**:
  - `human-judgment` TR-2.1: 系统设置界面有 system_prompt 配置选项
  - `programmatic` TR-2.2: 应用能正确读取和保存 system_prompt 配置
  - `human-judgment` TR-2.3: 聊天测试验证 system_prompt 生效
- **Notes**: 需要修改前端和后端代码，添加相应的 API 接口

## [x] Task 3: 启动流程优化
- **Priority**: P0
- **Depends On**: None
- **Description**:
  - 分析应用启动阻塞的原因
  - 优化启动流程，避免阻塞导致的跳转问题
  - 确保应用启动时间不超过 3 秒
- **Acceptance Criteria Addressed**: AC-3
- **Test Requirements**:
  - `human-judgment` TR-3.1: 应用启动无阻塞或跳转问题
  - `programmatic` TR-3.2: 应用启动时间不超过 3 秒
- **Notes**: 需要分析启动流程，找出阻塞的具体原因