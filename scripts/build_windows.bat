@echo off
echo Building for Windows...

REM Check if GCC is available (required for CGO)
where gcc >nul 2>nul
if %errorlevel% neq 0 (
    echo Warning: GCC not found. Building without embedded llama.cpp (Mock mode).
    echo Please install MinGW-w64 to build with full LLM support.
    go build -o knowledge.exe -ldflags "-H=windowsgui" ./cmd/server
) else (
    echo GCC found. Building with embedded llama.cpp...
    set CGO_ENABLED=1
    go build -tags cgo_llama -ldflags "-H=windowsgui" -o knowledge.exe ./cmd/server
)

if %errorlevel% neq 0 (
    echo Build failed!
    exit /b %errorlevel%
)

echo Build successful! Output: knowledge.exe
pause
