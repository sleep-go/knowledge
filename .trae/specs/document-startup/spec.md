# 项目启动文档 Spec

## Why
用户需要清楚地知道如何启动项目，目前的启动方式包含多个参数（端口、模型路径等），需要文档化和简化。

## What Changes
- 更新 `README.md`，添加详细的启动说明。
- 创建 `start.sh` (Mac/Linux) 和 `start.bat` (Windows) 启动脚本，封装常用参数。

## Impact
- Affected specs: 无
- Affected code: 新增 `start.sh`, `start.bat`, 修改 `README.md`

## ADDED Requirements
### Requirement: 启动脚本
系统应提供一键启动脚本，自动检测模型并使用推荐参数运行服务。

#### Scenario: 默认启动
- **WHEN** 用户运行 `./start.sh`
- **THEN** 服务在默认端口启动，并自动加载 `models/` 目录下的可用模型。

## MODIFIED Requirements
### Requirement: 文档更新
`README.md` 必须包含：
- 环境要求 (Go, Make 等)
- 模型下载说明
- 启动命令示例
