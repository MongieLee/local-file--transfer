import axios from 'axios'
import type { FileItem, ServerInfo, ClipboardData } from './types'

const apiClient = axios.create({
  baseURL: '/api',
  timeout: 300000,
})

export const api = {
  async getServerInfo(): Promise<ServerInfo> {
    const res = await apiClient.get<{ code: number; data: ServerInfo }>('/info')
    return res.data.data
  },

  async getFiles(): Promise<FileItem[]> {
    const res = await apiClient.get<{ code: number; data: FileItem[] }>('/files')
    return res.data.data
  },

  async uploadSingleFile(
    file: File,
    onProgress: (loaded: number, total: number, rate?: number) => void
  ): Promise<void> {
    const formData = new FormData()
    formData.append('files', file)

    await apiClient.post('/upload', formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
      timeout: 0, // 大文件上传取消固定超时，支持超大文件稳定流式传输
      onUploadProgress: (progressEvent) => {
        const total = progressEvent.total || file.size
        const loaded = progressEvent.loaded || 0
        const rate = (progressEvent as any).rate
        onProgress(loaded, total, rate)
      },
    })
  },

  async deleteFile(filename: string): Promise<void> {
    await apiClient.delete(`/files/${encodeURIComponent(filename)}`)
  },

  async batchDeleteFiles(filenames: string[]): Promise<void> {
    await apiClient.post('/files/batch-delete', { files: filenames })
  },

  async getClipboard(): Promise<ClipboardData> {
    const res = await apiClient.get<{ code: number; data: ClipboardData }>('/clipboard')
    return res.data.data
  },

  async updateClipboard(content: string): Promise<ClipboardData> {
    const res = await apiClient.post<{ code: number; data: ClipboardData }>('/clipboard', {
      content,
    })
    return res.data.data
  },
}

export function formatTime(isoStr: string): string {
  if (!isoStr) return ''
  const date = new Date(isoStr)
  const pad = (n: number) => (n < 10 ? '0' + n : n)
  const y = date.getFullYear()
  const m = pad(date.getMonth() + 1)
  const d = pad(date.getDate())
  const hh = pad(date.getHours())
  const mm = pad(date.getMinutes())
  return `${y}-${m}-${d} ${hh}:${mm}`
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

/**
 * 健壮的跨环境剪贴板复制工具
 * 针对局域网 HTTP（非安全上下文，navigator.clipboard 会失效或为 undefined）自动优雅降级为 execCommand
 */
export async function copyText(text: string): Promise<boolean> {
  if (!text) return false

  // 1. 尝试现代异步 Clipboard API (在 HTTPS 或 localhost 环境可用)
  if (typeof navigator !== 'undefined' && navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // 降级继续
    }
  }

  // 2. 传统通用降级方案（基于隐藏 textarea + execCommand，兼容所有局域网 HTTP 及移动端环境）
  try {
    const textArea = document.createElement('textarea')
    textArea.value = text
    textArea.style.position = 'fixed'
    textArea.style.top = '0'
    textArea.style.left = '0'
    textArea.style.width = '2em'
    textArea.style.height = '2em'
    textArea.style.padding = '0'
    textArea.style.border = 'none'
    textArea.style.outline = 'none'
    textArea.style.boxShadow = 'none'
    textArea.style.background = 'transparent'
    textArea.style.fontSize = '16px' // 避免 iOS 放大视口
    textArea.setAttribute('readonly', '')

    document.body.appendChild(textArea)
    textArea.focus()
    textArea.select()
    textArea.setSelectionRange(0, text.length)

    const successful = document.execCommand('copy')
    document.body.removeChild(textArea)
    return successful
  } catch (err) {
    console.error('Fallback copy failed:', err)
    return false
  }
}

export type FileCategory = 'image' | 'video' | 'audio' | 'archive' | 'code' | 'doc' | 'pdf' | 'other'

export function getFileCategory(filename: string, mime: string): FileCategory {
  const ext = filename.split('.').pop()?.toLowerCase() || ''
  
  if (mime?.startsWith('image/') || ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico'].includes(ext)) {
    return 'image'
  }
  if (mime?.startsWith('video/') || ['mp4', 'webm', 'ogg', 'mov', 'avi', 'mkv'].includes(ext)) {
    return 'video'
  }
  if (mime?.startsWith('audio/') || ['mp3', 'wav', 'flac', 'aac', 'm4a'].includes(ext)) {
    return 'audio'
  }
  if (['zip', 'rar', '7z', 'tar', 'gz', 'bz2', 'xz'].includes(ext)) {
    return 'archive'
  }
  if (['js', 'ts', 'jsx', 'tsx', 'html', 'css', 'json', 'py', 'go', 'java', 'c', 'cpp', 'rs', 'sh', 'sql', 'yaml', 'yml'].includes(ext)) {
    return 'code'
  }
  if (ext === 'pdf') {
    return 'pdf'
  }
  if (['doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'txt', 'md'].includes(ext)) {
    return 'doc'
  }
  return 'other'
}
