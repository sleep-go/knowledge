# Knowledge - 本地 LLM 知识库系统

这是一个基于 Go 和 llama.cpp 的本地大语言模型 (LLM) 运行环境。支持在本地运行 GGUF 格式的模型，并提供 Web 界面进行对话。

## 快速开始

### 1. 环境要求
- **Go**: 需要安装 Go 1.20+ ([下载地址](https://go.dev/dl/))
- **Git**: 用于克隆仓库
- **CMake**: 用于编译 llama.cpp
- **C++ 编译器**: (如 GCC, Clang, MSVC)

### 2. 编译依赖 (重要)
在使用之前，必须先编译 `llama.cpp` 的静态库：

```bash
git clone --recurse-submodules git@github.com:sleep-go/knowledge.git
cd llama.cpp
cmake -B build -DBUILD_SHARED_LIBS=OFF
cmake --build build --config Release --target llama common
cd ..
```

### 3. 下载模型
本项目使用 GGUF 格式的模型。你需要下载一个模型文件并放置在 `models/` 目录下。

**安装步骤：**
1. 下载 `.gguf` 文件（例如 `tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf`）。
2. 将文件移动到项目根目录下的 `models/` 文件夹中。
   ```bash
   mkdir -p models
   mv /path/to/downloaded/model.gguf models/
   ```

### 3. 启动服务

我们提供了简便的启动脚本，会自动检测 `models/` 目录下的模型并启动服务。

**macOS / Linux:**
```bash
./start.sh
```

**Windows:**
```cmd
start.bat
```

启动后，浏览器会自动打开 `http://localhost:8080` (端口可能根据可用性自动调整)。

### 4. 手动启动 (高级)
如果你想手动指定参数，可以使用以下命令：

```bash
go run ./cmd/server -port 8080 -model models/your-model.gguf
```

## 功能特性
- **本地推理**: 数据不出本地，隐私安全。
- **Web 界面**: 简洁的聊天界面。
- **历史记录**: 自动保存对话历史。
- **多模型支持**: 支持任何兼容 llama.cpp 的 GGUF 模型。

## 打包应用

### macOS 应用打包
我们提供了 `build_app.sh` 脚本用于在 macOS 上创建可分发的应用程序包：

```bash
# 确保脚本有执行权限
chmod +x scripts/build_app.sh

# 运行打包脚本
./scripts/build_app.sh
```

打包完成后，会在 `bin/` 目录下生成 `Knowledge.app` 应用程序包。

### 打包注意事项
- 打包前确保已编译好 Go 二进制文件
- 打包过程会自动复制 `models/` 目录下的模型文件
- 生成的应用程序包可以直接双击运行

## 故障排除
- **服务无法启动**: 检查 `bin/llama-server` 是否存在且有执行权限。
- **模型加载失败**: 确保下载的是 `.gguf` 格式，且文件未损坏。
- **中文乱码**: 尝试使用支持中文较好的模型（如 Qwen, Yi, Llama3-Chinese）。
