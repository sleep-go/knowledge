#!/bin/bash

# 检查是否安装了 go
if ! command -v go &> /dev/null; then
    echo "错误: 未安装 go。"
    exit 1
fi

# 运行服务器
# 允许通过参数传递模型路径，例如: ./start.sh -model models/my-model.gguf
# 如果没有传递参数，go run 会使用代码中的默认逻辑（自动寻找 models/*.gguf）
go run -tags cgo_llama ./cmd/server "$@"
