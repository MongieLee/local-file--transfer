#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "$DIR"

echo "=========================================="
echo "        局域网文件床 (Local Drop)         "
echo "=========================================="

# 检查前端 dist 是否已构建
if [ ! -d "frontend/dist" ]; then
    echo "检测到前端尚未构建，正在编译前端静态资源..."
    cd frontend
    pnpm install
    pnpm build
    cd "$DIR"
fi

# 检查后端二进制是否存在
if [ ! -f "backend/server" ]; then
    echo "检测到后端尚未编译，正在编译 Golang 后端..."
    cd backend
    go build -o server main.go
    cd "$DIR"
fi

echo "正在启动文件床服务..."
cd backend
exec ./server
