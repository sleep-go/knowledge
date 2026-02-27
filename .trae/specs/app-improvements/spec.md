# Knowledge 应用改进 - 产品需求文档

## Overview
- **Summary**: 对 Knowledge macOS 应用进行改进，包括数据库打包优化、系统提示配置和启动流程优化。
- **Purpose**: 解决当前应用存在的问题，提升用户体验和灵活性。
- **Target Users**: 使用 Knowledge macOS 应用的用户。

## Goals
- 优化数据库打包过程，确保只打包表结构而不包含数据
- 将 system_prompt 配置移到系统设置中，允许用户自定义
- 修复应用启动时的阻塞问题，避免应用一直跳转

## Non-Goals (Out of Scope)
- 更改应用的核心功能逻辑
- 重构现有的代码架构
- 添加新的功能模块

## Background & Context
- 当前应用在打包时会包含完整的数据库文件，包括数据
- system_prompt 目前是硬编码的，用户无法自定义
- 应用启动时服务会阻塞，导致应用一直跳转

## Functional Requirements
- **FR-1**: 数据库打包优化 - 打包时只包含表结构，不包含数据
- **FR-2**: 系统提示配置 - 将 system_prompt 移到系统设置中，允许用户自定义
- **FR-3**: 启动流程优化 - 修复应用启动时的阻塞问题

## Non-Functional Requirements
- **NFR-1**: 性能 - 应用启动时间不应超过 3 秒
- **NFR-2**: 可用性 - 系统设置界面应简洁易用
- **NFR-3**: 稳定性 - 应用应能稳定运行，无崩溃或异常行为

## Constraints
- **Technical**: 基于现有的 Go 和 llama.cpp 架构
- **Business**: 保持与现有 web 页面功能的一致性
- **Dependencies**: 依赖现有的数据库结构和文件系统

## Assumptions
- 用户需要能够自定义 system_prompt 以适应不同的使用场景
- 打包的应用应该是干净的，不包含任何用户数据
- 应用启动应该流畅，无阻塞或跳转问题

## Acceptance Criteria

### AC-1: 数据库打包优化
- **Given**: 用户运行打包脚本
- **When**: 脚本执行完成
- **Then**: 生成的应用包中的数据库只包含表结构，不包含任何数据
- **Verification**: `programmatic`
- **Notes**: 可以通过查看生成的数据库文件大小或内容来验证

### AC-2: 系统提示配置
- **Given**: 用户打开系统设置
- **When**: 用户修改 system_prompt 配置
- **Then**: 应用应该使用用户配置的 system_prompt
- **Verification**: `human-judgment`
- **Notes**: 可以通过聊天测试来验证 system_prompt 是否生效

### AC-3: 启动流程优化
- **Given**: 用户双击打开应用
- **When**: 应用启动
- **Then**: 应用应该正常启动，无阻塞或跳转问题
- **Verification**: `human-judgment`
- **Notes**: 可以通过观察应用启动过程来验证

## Open Questions
- [ ] 数据库表结构的初始化方式是否需要修改？
- [ ] system_prompt 的默认值应该是什么？
- [ ] 启动阻塞问题的具体原因是什么？