@echo off
chcp 65001 >nul

REM 检查是否安装了 go
where go >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo 错误: 未安装 go。
    exit /b 1
)

REM 运行服务器
REM 允许通过参数传递模型路径，例如: start.bat -model models/my-model.gguf
go run -tags cgo_llama ./cmd/server %*
