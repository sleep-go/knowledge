@echo off
setlocal

:: 设置 CGO_ENABLED=1 以启用 CGO 支持
set CGO_ENABLED=1

:: 检查是否安装了 GCC (MinGW-w64)
where gcc >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] GCC not found. Please install MinGW-w64 to enable CGO.
    echo You can download it from: https://github.com/skeeto/w64devkit/releases
    echo.
    echo If you want to run without LLM features (stub mode), set CGO_ENABLED=0 in this script.
    pause
    exit /b 1
)

echo [INFO] CGO is enabled. Building and starting...

:: 运行项目
go run ./cmd/server

if %errorlevel% neq 0 (
    echo [ERROR] Failed to run the application.
    pause
    exit /b %errorlevel%
)

endlocal
