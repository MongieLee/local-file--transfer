.PHONY: all build run dev clean help

all: build

build:
	@echo "==> 构建前端..."
	cd frontend && pnpm install && pnpm build
	@echo "==> 编译 Go 后端..."
	cd backend && go build -o server main.go
	@echo "==> 构建完成! 运行 ./backend/server 即可启动。"

run:
	@cd backend && ./server

dev-backend:
	@cd backend && go run main.go

dev-frontend:
	@cd frontend && pnpm dev

clean:
	rm -rf backend/server frontend/dist
