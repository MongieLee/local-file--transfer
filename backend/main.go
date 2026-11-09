package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type FileItem struct {
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	HumanSize    string    `json:"humanSize"`
	ModTime      time.Time `json:"modTime"`
	MimeType     string    `json:"mimeType"`
	DownloadURL  string    `json:"downloadUrl"`
	PreviewURL   string    `json:"previewUrl"`
}

type ServerInfo struct {
	Port int      `json:"port"`
	IPs  []string `json:"ips"`
	URLs []string `json:"urls"`
}

type ClipboardStore struct {
	sync.RWMutex
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updatedAt"`
}

var (
	uploadDir      = "./uploads"
	clipboardStore = &ClipboardStore{
		Content:   "",
		UpdatedAt: time.Now(),
	}
	broker = &EventBroker{
		clients: make(map[chan string]bool),
	}
)

type EventBroker struct {
	sync.Mutex
	clients map[chan string]bool
}

func (b *EventBroker) Subscribe() chan string {
	b.Lock()
	defer b.Unlock()
	ch := make(chan string, 16)
	b.clients[ch] = true
	return ch
}

func (b *EventBroker) Unsubscribe(ch chan string) {
	b.Lock()
	defer b.Unlock()
	delete(b.clients, ch)
	close(ch)
}

func (b *EventBroker) Broadcast(eventName, dataJSON string) {
	b.Lock()
	defer b.Unlock()
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventName, dataJSON)
	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// 获取本机所有有效的局域网 IPv4 地址
func getLocalIPs() []string {
	var ips []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return ips
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue // 忽略未启动或回环网卡
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// 只选取 IPv4
			ip = ip.To4()
			if ip != nil {
				ips = append(ips, ip.String())
			}
		}
	}
	return ips
}

func sanitizeFilename(name string) string {
	clean := filepath.Base(name)
	clean = strings.ReplaceAll(clean, "..", "")
	clean = strings.ReplaceAll(clean, "/", "")
	clean = strings.ReplaceAll(clean, "\\", "")
	if clean == "" || clean == "." {
		clean = fmt.Sprintf("upload_%d", time.Now().UnixNano())
	}
	return clean
}

func getUniqueFilename(dir, originalName string) string {
	clean := sanitizeFilename(originalName)
	target := filepath.Join(dir, clean)

	if _, err := os.Stat(target); os.IsNotExist(err) {
		return clean
	}

	ext := filepath.Ext(clean)
	base := strings.TrimSuffix(clean, ext)

	for i := 1; ; i++ {
		newName := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(filepath.Join(dir, newName)); os.IsNotExist(err) {
			return newName
		}
	}
}

func main() {
	// 确保上传目录存在
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		panic("Failed to create upload dir: " + err.Error())
	}

	portFlag := flag.Int("port", 8999, "Server port")
	flag.Parse()

	port := *portFlag
	if envPort := os.Getenv("PORT"); envPort != "" {
		fmt.Sscanf(envPort, "%d", &port)
	}

	router := gin.Default()
	// 设置 64MB 内存缓冲区，超出部分流式落盘，支持大文件上传
	router.MaxMultipartMemory = 64 << 20

	// CORS 配置，允许局域网跨域
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Range"}
	corsConfig.ExposeHeaders = []string{"Content-Length", "Content-Range", "Content-Disposition"}
	router.Use(cors.New(corsConfig))

	api := router.Group("/api")
	{
		// SSE 实时事件推送 (剪贴板、文件变动毫秒级推送)
		api.GET("/events", func(c *gin.Context) {
			c.Writer.Header().Set("Content-Type", "text/event-stream")
			c.Writer.Header().Set("Cache-Control", "no-cache")
			c.Writer.Header().Set("Connection", "keep-alive")
			c.Writer.Header().Set("Transfer-Encoding", "chunked")

			ch := broker.Subscribe()
			defer broker.Unsubscribe(ch)

			c.Stream(func(w io.Writer) bool {
				select {
				case msg, ok := <-ch:
					if !ok {
						return false
					}
					c.Writer.WriteString(msg)
					c.Writer.Flush()
					return true
				case <-c.Request.Context().Done():
					return false
				}
			})
		})

		// 局域网信息
		api.GET("/info", func(c *gin.Context) {
			ips := getLocalIPs()
			var urls []string
			for _, ip := range ips {
				urls = append(urls, fmt.Sprintf("http://%s:%d", ip, port))
			}
			c.JSON(http.StatusOK, gin.H{
				"code": 0,
				"data": ServerInfo{
					Port: port,
					IPs:  ips,
					URLs: urls,
				},
			})
		})

		// 获取文件列表
		api.GET("/files", func(c *gin.Context) {
			entries, err := os.ReadDir(uploadDir)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Failed to read upload directory"})
				return
			}

			files := make([]FileItem, 0, len(entries))
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				info, err := entry.Info()
				if err != nil {
					continue
				}

				name := entry.Name()
				mimeType := mime.TypeByExtension(filepath.Ext(name))
				if mimeType == "" {
					mimeType = "application/octet-stream"
				}

				encodedName := url.PathEscape(name)

				files = append(files, FileItem{
					Name:        name,
					Size:        info.Size(),
					HumanSize:   formatFileSize(info.Size()),
					ModTime:     info.ModTime(),
					MimeType:    mimeType,
					DownloadURL: "/api/download/" + encodedName,
					PreviewURL:  "/api/preview/" + encodedName,
				})
			}

			// 按修改时间从新到旧排序
			sort.Slice(files, func(i, j int) bool {
				return files[i].ModTime.After(files[j].ModTime)
			})

			c.JSON(http.StatusOK, gin.H{
				"code": 0,
				"data": files,
			})
		})

		// 上传文件（支持单/多文件流式保存）
		api.POST("/upload", func(c *gin.Context) {
			form, err := c.MultipartForm()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "Invalid multipart form: " + err.Error()})
				return
			}

			files := form.File["files"]
			if len(files) == 0 {
				// 兼容以 "file" 为 key 的单文件上传
				files = form.File["file"]
			}

			if len(files) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "No file uploaded"})
				return
			}

			type UploadedResult struct {
				OriginalName string `json:"originalName"`
				SavedName    string `json:"savedName"`
				Size         int64  `json:"size"`
				DownloadURL  string `json:"downloadUrl"`
			}

			var results []UploadedResult

			for _, fileHeader := range files {
				uniqueName := getUniqueFilename(uploadDir, fileHeader.Filename)
				targetPath := filepath.Join(uploadDir, uniqueName)

				src, err := fileHeader.Open()
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Failed to open uploaded file: " + err.Error()})
					return
				}

				dst, err := os.Create(targetPath)
				if err != nil {
					src.Close()
					c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Failed to create destination file: " + err.Error()})
					return
				}

				written, err := io.Copy(dst, src)
				src.Close()
				dst.Close()

				if err != nil {
					os.Remove(targetPath)
					c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Failed to save file: " + err.Error()})
					return
				}

				results = append(results, UploadedResult{
					OriginalName: fileHeader.Filename,
					SavedName:    uniqueName,
					Size:         written,
					DownloadURL:  "/api/download/" + url.PathEscape(uniqueName),
				})
			}

			// 广播文件变动给所有客户端
			broker.Broadcast("files", `{"action":"upload"}`)

			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "Upload successful",
				"data":    results,
			})
		})

		// 下载文件 (强制下载)
		api.GET("/download/:filename", func(c *gin.Context) {
			filename := c.Param("filename")
			cleanName := sanitizeFilename(filename)
			filePath := filepath.Join(uploadDir, cleanName)

			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "File not found"})
				return
			}

			// 支持 UTF-8 编码的文件名下载
			encodedFilename := url.QueryEscape(cleanName)
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedFilename))
			c.File(filePath)
		})

		// 预览文件 (inline，支持 Range，可在线看图、看视频、听音频、读文本)
		api.GET("/preview/:filename", func(c *gin.Context) {
			filename := c.Param("filename")
			cleanName := sanitizeFilename(filename)
			filePath := filepath.Join(uploadDir, cleanName)

			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "File not found"})
				return
			}

			mimeType := mime.TypeByExtension(filepath.Ext(cleanName))
			if mimeType != "" {
				c.Header("Content-Type", mimeType)
			}
			c.Header("Content-Disposition", "inline")
			c.File(filePath)
		})

		// 单个文件删除
		api.DELETE("/files/:filename", func(c *gin.Context) {
			filename := c.Param("filename")
			cleanName := sanitizeFilename(filename)
			filePath := filepath.Join(uploadDir, cleanName)

			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "File not found"})
				return
			}

			if err := os.Remove(filePath); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Failed to delete file"})
				return
			}

			// 广播文件删除
			broker.Broadcast("files", `{"action":"delete"}`)

			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "File deleted successfully"})
		})

		// 批量删除
		type BatchDeleteReq struct {
			Files []string `json:"files"`
		}
		api.POST("/files/batch-delete", func(c *gin.Context) {
			var req BatchDeleteReq
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "Invalid request body"})
				return
			}

			deleted := 0
			for _, fn := range req.Files {
				cleanName := sanitizeFilename(fn)
				filePath := filepath.Join(uploadDir, cleanName)
				if err := os.Remove(filePath); err == nil {
					deleted++
				}
			}

			if deleted > 0 {
				broker.Broadcast("files", `{"action":"batch-delete"}`)
			}

			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": fmt.Sprintf("Successfully deleted %d file(s)", deleted),
				"deleted": deleted,
			})
		})

		// 文本剪贴板 - 获取
		api.GET("/clipboard", func(c *gin.Context) {
			clipboardStore.RLock()
			defer clipboardStore.RUnlock()

			c.JSON(http.StatusOK, gin.H{
				"code": 0,
				"data": gin.H{
					"content":   clipboardStore.Content,
					"updatedAt": clipboardStore.UpdatedAt,
				},
			})
		})

		// 文本剪贴板 - 更新
		type ClipboardReq struct {
			Content string `json:"content"`
		}
		api.POST("/clipboard", func(c *gin.Context) {
			var req ClipboardReq
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "Invalid request body"})
				return
			}

			clipboardStore.Lock()
			clipboardStore.Content = req.Content
			clipboardStore.UpdatedAt = time.Now()
			updatedContent := clipboardStore.Content
			updatedAt := clipboardStore.UpdatedAt
			clipboardStore.Unlock()

			// 实时广播给所有局域网已连接客户端
			payload, _ := json.Marshal(gin.H{
				"content":   updatedContent,
				"updatedAt": updatedAt,
			})
			broker.Broadcast("clipboard", string(payload))

			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "Clipboard updated successfully",
				"data": gin.H{
					"content":   clipboardStore.Content,
					"updatedAt": clipboardStore.UpdatedAt,
				},
			})
		})
	}

	// 尝试托管前端静态文件（支持多路径探测）
	candidateDirs := []string{
		"./frontend/dist",
		"../frontend/dist",
		"./dist",
	}
	var frontendDist string
	for _, dir := range candidateDirs {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			frontendDist = dir
			break
		}
	}

	if frontendDist != "" {
		router.Static("/assets", filepath.Join(frontendDist, "assets"))
		router.StaticFile("/favicon.ico", filepath.Join(frontendDist, "favicon.ico"))
		router.StaticFile("/favicon.svg", filepath.Join(frontendDist, "favicon.svg"))
		router.NoRoute(func(c *gin.Context) {
			if !strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.File(filepath.Join(frontendDist, "index.html"))
			}
		})
		fmt.Printf("   前端静态资源: 已挂载自 %s\n", frontendDist)
	}

	// 打印局域网启动信息
	ips := getLocalIPs()
	fmt.Printf("\n🚀 文件床服务已启动!\n")
	fmt.Printf("   本地访问:   http://localhost:%d\n", port)
	for _, ip := range ips {
		fmt.Printf("   局域网访问: http://%s:%d\n", ip, port)
	}
	fmt.Printf("   上传存储目录: %s\n\n", uploadDir)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	if err := router.Run(addr); err != nil {
		fmt.Printf("Failed to run server: %v\n", err)
	}
}
