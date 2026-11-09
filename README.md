# 局域网文件床 (Local Drop)

一个专为局域网环境打造的高性能、免登录文件互传与共享工具。
后端采用 **Golang (Gin)** 实现高并发与流式大文件传输，前端采用 **Vue 3 + TypeScript + Vite** 构建现代化自适应 UI。

---

## ✨ 核心特性

- ⚡️ **免登录极速传输**：局域网内任意设备（PC、Mac、iPhone、Android、iPad等）打开浏览器即可使用。
- 📱 **二维码一键互联**：
  - 自动获取本机局域网 IP（如 `192.168.x.x:8080`），顶部提供手机扫码直达弹窗。
  - 每个文件都支持生成独立的**扫码下载二维码**，手机扫码秒下。
- 📦 **大文件 & 多文件流式上传**：
  - 支持拖拽多文件、点击上传。
  - 实时独立进度条（0-100%）、状态提示。
  - 后端流式落盘，突破内存限制，轻松传输数十 GB 大文件。
- 🎬 **在线预览**：
  - 支持图片（JPG/PNG/GIF/WebP等）灯箱预览。
  - 支持视频（MP4/WebM）在线流畅播放（支持 HTTP Range 断点续传与进度拖动）。
  - 支持音频播放。
- 📋 **局域网共享剪贴板**：
  - 跨设备互传文字、URL、Token、命令或随手记，一键同步，多端一键复制。
- 🔍 **管理功能**：
  - 实时文件名搜索筛选。
  - 多选与批量删除、单个快速删除。
  - 一键复制局域网下载直链。

---

## 📁 目录结构

```text
.
├── backend/            # Golang 后端服务
│   ├── main.go         # API 路由、文件流式保存、局域网 IP 自动检测
│   ├── uploads/        # 上传文件物理存储目录
│   └── go.mod
├── frontend/           # Vue 3 + TypeScript 前端
│   ├── src/
│   │   ├── App.vue     # 主应用界面（自适应响应式）
│   │   ├── api.ts      # API 请求与工具库
│   │   ├── types.ts    # TypeScript 类型定义
│   │   └── style.css   # 现代设计风格样式
│   └── package.json
├── start.sh            # 一键启动脚本（自动构建并启动）
├── Makefile            # 常用开发与构建指令
└── README.md
```

---

## 🚀 快速启动

### 方式一：一键脚本（推荐）

在项目根目录下直接运行：

```bash
./start.sh
```

脚本会自动检查并构建前端、编译后端，并在控制台输出当前局域网访问地址：
```text
🚀 文件床服务已启动!
   本地访问:   http://localhost:8999
   局域网访问: http://192.168.1.54:8999
   上传存储目录: ./uploads
```

### 方式二：使用 Makefile

- **构建完整应用**：
  ```bash
  make build
  ```
- **启动服务**：
  ```bash
  make run
  ```

### 方式三：前后端独立开发模式

1. **启动后端服务**：
   ```bash
   cd backend
   go run main.go
   ```
2. **启动前端开发服务器（带热重载）**：
   ```bash
   cd frontend
   pnpm dev
   ```
   前端本地访问 `http://localhost:5173`，接口会自动反向代理至后端 `http://localhost:8080`。

---

## 🛠 技术栈

- **后端**：Go 1.20+、Gin Web Framework、CORS 中间件
- **前端**：Vue 3 (Composition API)、TypeScript、Vite、Axios、@lucide/vue、qrcode.vue
- **部署**：支持前端构建静态资源与 Go 二进制服务一体化运行
