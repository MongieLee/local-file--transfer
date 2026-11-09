export interface FileItem {
  name: string
  size: number
  humanSize: string
  modTime: string
  mimeType: string
  downloadUrl: string
  previewUrl: string
}

export interface ServerInfo {
  port: number
  ips: string[]
  urls: string[]
}

export interface TransferTask {
  id: string
  name: string
  size: number
  progress: number // 0 - 100
  speed: string    // e.g. "12.5 MB/s"
  eta: string      // e.g. "3s"
  status: 'pending' | 'uploading' | 'success' | 'error'
  errorMsg?: string
  startTime: number
  loadedBytes: number
}

export interface ClipboardData {
  content: string
  updatedAt: string
}
