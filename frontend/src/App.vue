<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue'
import {
  UploadCloud,
  FileText,
  Image as ImageIcon,
  Video as VideoIcon,
  Music as MusicIcon,
  Archive as ArchiveIcon,
  FileCode as FileCodeIcon,
  File as GenericFile,
  Download,
  Trash2,
  QrCode,
  Copy,
  Check,
  RefreshCw,
  Smartphone,
  Search,
  Eye,
  X,
  Folder,
  Radio,
  ClipboardList
} from '@lucide/vue'
import QrcodeVue from 'qrcode.vue'
import { api, copyText, formatTime, formatBytes, getFileCategory, type FileCategory } from './api'
import type { FileItem, ServerInfo, TransferTask } from './types'

// 当前导航菜单
type NavMenu = 'files' | 'clipboard' | 'devices'
const currentMenu = ref<NavMenu>('files')

// 数据状态
const files = ref<FileItem[]>([])
const loading = ref(false)
const serverInfo = ref<ServerInfo>({ port: 8999, ips: [], urls: [] })
const searchQuery = ref('')
const activeTransfers = ref<TransferTask[]>([])

// 剪贴板状态
const clipboardContent = ref('')
const clipboardUpdatedAt = ref('')
const clipboardSaving = ref(false)

// 拖拽与上传状态
const isDragging = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)

// 弹窗状态
const qrModal = ref({
  visible: false,
  title: '',
  tip: '',
  url: '',
})

const previewModal = ref<{
  visible: boolean
  name: string
  url: string
  category: FileCategory
  mimeType: string
}>({
  visible: false,
  name: '',
  url: '',
  category: 'other',
  mimeType: '',
})

// Toast 状态
interface Toast {
  id: number
  message: string
}
const toasts = ref<Toast[]>([])
let toastSeq = 0
function showToast(message: string) {
  const id = ++toastSeq
  toasts.value.push({ id, message })
  setTimeout(() => {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }, 2200)
}

// 局域网基准 URL
function getBaseUrl(): string {
  const currentOrigin = window.location.origin
  const isLocal = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
  if (isLocal && serverInfo.value.urls.length > 0) {
    return serverInfo.value.urls[0]
  }
  return currentOrigin
}

// 过滤后的历史文件
const displayFiles = computed(() => {
  let list = files.value
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter((f) => f.name.toLowerCase().includes(q))
  }
  return list
})

// 分页状态 (方案 A: 复古像素分页器)
const currentPage = ref(1)
const pageSize = ref(10)

watch(searchQuery, () => {
  currentPage.value = 1
})

const totalPages = computed(() => {
  const count = Math.ceil(displayFiles.value.length / pageSize.value)
  return Math.max(1, count)
})

watch(totalPages, (newTotal) => {
  if (currentPage.value > newTotal) {
    currentPage.value = newTotal
  }
})

// 当前页显示的文件列表
const pagedFiles = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return displayFiles.value.slice(start, start + pageSize.value)
})

function prevPage() {
  if (currentPage.value > 1) {
    currentPage.value--
  }
}

function nextPage() {
  if (currentPage.value < totalPages.value) {
    currentPage.value++
  }
}

// 加载文件
async function loadFiles() {
  loading.value = true
  try {
    files.value = await api.getFiles()
  } catch (err: any) {
    showToast('获取文件失败: ' + (err.message || '网络异常'))
  } finally {
    loading.value = false
  }
}

// 加载服务器信息
async function loadServerInfo() {
  try {
    serverInfo.value = await api.getServerInfo()
  } catch {
    // 忽略
  }
}

// 加载剪贴板
async function loadClipboard() {
  try {
    const res = await api.getClipboard()
    clipboardContent.value = res.content
    clipboardUpdatedAt.value = res.updatedAt
  } catch {
    // 忽略
  }
}

async function saveClipboard() {
  clipboardSaving.value = true
  try {
    const res = await api.updateClipboard(clipboardContent.value)
    clipboardUpdatedAt.value = res.updatedAt
    showToast('剪贴板同步成功')
  } catch {
    showToast('保存失败')
  } finally {
    clipboardSaving.value = false
  }
}

async function copyClipboardText() {
  if (!clipboardContent.value) return
  const ok = await copyText(clipboardContent.value)
  if (ok) {
    showToast('文本已复制到本地剪贴板')
  } else {
    showToast('复制失败，请长按手动复制')
  }
}

function clearClipboard() {
  clipboardContent.value = ''
}

// 处理上传
async function handleFilesUpload(fileList: FileList | File[] | null) {
  if (!fileList || fileList.length === 0) return

  const list = Array.from(fileList)
  for (const file of list) {
    const taskId = `${Date.now()}_${Math.random().toString(36).substring(2, 6)}`
    const startTime = Date.now()

    const task = reactive<TransferTask>({
      id: taskId,
      name: file.name,
      size: file.size,
      progress: 0,
      speed: '0 KB/s',
      eta: '连接中...',
      status: 'uploading',
      startTime,
      loadedBytes: 0,
    })
    activeTransfers.value.unshift(task)

    try {
      await api.uploadSingleFile(file, (loaded, total, rate) => {
        task.loadedBytes = loaded
        const safeTotal = total || file.size || 1
        const percent = Math.min(99, Math.round((loaded * 100) / safeTotal))
        task.progress = percent

        if (rate && rate > 0) {
          task.speed = `${formatBytes(rate)}/s`
          const remainingBytes = Math.max(0, safeTotal - loaded)
          const remainingSec = Math.round(remainingBytes / rate)
          task.eta = remainingSec <= 0 ? '即将完成' : `剩余 ${remainingSec}秒`
        } else {
          const elapsedSeconds = (Date.now() - startTime) / 1000
          if (elapsedSeconds > 0.1 && loaded > 0) {
            const speedBytesPerSec = loaded / elapsedSeconds
            task.speed = `${formatBytes(speedBytesPerSec)}/s`
            const remainingBytes = Math.max(0, safeTotal - loaded)
            const remainingSec = Math.round(remainingBytes / speedBytesPerSec)
            task.eta = remainingSec <= 0 ? '即将完成' : `剩余 ${remainingSec}秒`
          }
        }
      })

      task.status = 'success'
      task.progress = 100
      task.speed = '完成'
      task.eta = '已就绪'
      showToast(`《${file.name}》传输完成`)
    } catch (err: any) {
      task.status = 'error'
      task.errorMsg = err.message || '上传异常'
      showToast(`《${file.name}》传输中断`)
    }
  }

  await loadFiles()
  setTimeout(() => {
    activeTransfers.value = activeTransfers.value.filter((t) => t.status === 'uploading')
  }, 4000)
}

function onDrop(e: DragEvent) {
  isDragging.value = false
  if (e.dataTransfer?.files) {
    handleFilesUpload(e.dataTransfer.files)
  }
}

function onFileSelect(e: Event) {
  const target = e.target as HTMLInputElement
  if (target.files) {
    handleFilesUpload(target.files)
  }
  target.value = ''
}

// 快捷粘贴
function handlePaste(e: ClipboardEvent) {
  if ((e.target as HTMLElement)?.tagName === 'TEXTAREA') return
  if (e.clipboardData?.files && e.clipboardData.files.length > 0) {
    e.preventDefault()
    handleFilesUpload(e.clipboardData.files)
    showToast('检测到剪贴板文件，开始上传')
  }
}

// 复制局域网下载链接
async function copyDownloadLink(file: FileItem) {
  const fullUrl = `${getBaseUrl()}${file.downloadUrl}`
  const ok = await copyText(fullUrl)
  if (ok) {
    showToast('下载直链已复制到剪贴板')
  } else {
    showToast('复制失败，请手动选择复制')
  }
}

// 复制本机 IP 访问地址
async function copyHostIp() {
  const fullUrl = getBaseUrl()
  const ok = await copyText(fullUrl)
  if (ok) {
    showToast(`局域网地址已复制: ${fullUrl}`)
  } else {
    showToast('复制失败，请手动选择复制')
  }
}

// 下载文件
function downloadFile(file: FileItem) {
  const link = document.createElement('a')
  link.href = file.downloadUrl
  link.setAttribute('download', file.name)
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

// 预览
function openPreview(file: FileItem) {
  const cat = getFileCategory(file.name, file.mimeType)
  const fullUrl = `${getBaseUrl()}${file.previewUrl}`
  if (['image', 'video', 'audio'].includes(cat)) {
    previewModal.value = {
      visible: true,
      name: file.name,
      url: fullUrl,
      category: cat,
      mimeType: file.mimeType,
    }
  } else {
    window.open(fullUrl, '_blank')
  }
}

// 二维码
function showQR(file: FileItem) {
  const fullUrl = `${getBaseUrl()}${file.downloadUrl}`
  qrModal.value = {
    visible: true,
    title: `手机扫码下载 《${file.name}》`,
    tip: '使用手机相机或扫码工具直接下载该文件',
    url: fullUrl,
  }
}

function showDeviceConnectQR() {
  const fullUrl = getBaseUrl()
  qrModal.value = {
    visible: true,
    title: '手机扫码连接文件床',
    tip: '确保手机与电脑在同一 Wi-Fi 下扫码接入',
    url: fullUrl,
  }
}

// 删除
async function deleteFile(file: FileItem) {
  if (!confirm(`确定要删除文件 《${file.name}》 吗？`)) return
  try {
    await api.deleteFile(file.name)
    showToast('文件已删除')
    await loadFiles()
  } catch {
    showToast('删除失败')
  }
}

let timer: any = null
let eventSource: EventSource | null = null
const isSseConnected = ref(false)

function setupEventSource() {
  if (typeof EventSource === 'undefined') return
  try {
    eventSource = new EventSource('/api/events')

    eventSource.onopen = () => {
      isSseConnected.value = true
    }

    // 实时剪贴板推送 (毫秒级响应)
    eventSource.addEventListener('clipboard', (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data)
        if (data.content !== clipboardContent.value) {
          clipboardContent.value = data.content
          clipboardUpdatedAt.value = data.updatedAt
          showToast('★ 收到远端剪贴板实时同步 ★')
        }
      } catch (err) {
        console.error('SSE clipboard error:', err)
      }
    })

    // 实时文件库变动推送 (上传/删除立即同步)
    eventSource.addEventListener('files', () => {
      loadFiles()
    })

    eventSource.onerror = () => {
      isSseConnected.value = false
    }
  } catch (err) {
    console.warn('SSE 连接异常:', err)
  }
}

const isMobile = ref(typeof window !== 'undefined' ? window.innerWidth <= 768 : false)
function handleResize() {
  isMobile.value = window.innerWidth <= 768
}

onMounted(async () => {
  window.addEventListener('resize', handleResize)
  window.addEventListener('paste', handlePaste)
  await Promise.all([loadFiles(), loadServerInfo(), loadClipboard()])
  setupEventSource()
  timer = setInterval(loadFiles, 30000)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  window.removeEventListener('paste', handlePaste)
  if (timer) clearInterval(timer)
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
})
</script>

<template>
  <div class="pixel-layout">
    <!-- 移动端专属顶部栏 (Mobile Top Bar) -->
    <header class="pixel-mobile-header">
      <div class="pixel-mobile-brand">
        <div class="pixel-brand-icon" style="width: 28px; height: 28px; padding: 2px;">
          <img src="/favicon.svg" alt="PixelDrop" style="width: 20px; height: 20px; image-rendering: pixelated;" />
        </div>
        <div style="display: flex; align-items: baseline; gap: 4px;">
          <span class="pixel-mobile-brand-title">PixelDrop</span>
          <span style="font-size: 0.62rem; font-weight: 800; color: #868e96;">闪传</span>
        </div>
      </div>

      <div
        v-if="serverInfo.ips.length > 0"
        class="pixel-ip-badge"
        style="margin-top: 0; padding: 4px 8px;"
        title="点击复制访问地址"
        @click="copyHostIp"
      >
        <div class="pixel-ip-text">
          <span class="pixel-dot"></span>
          <span>{{ serverInfo.ips[0] }}:{{ serverInfo.port }}</span>
        </div>
        <Copy :size="11" style="margin-left: 4px;" />
      </div>
    </header>

    <!-- 1. 桌面端左侧像素 Sidebar -->
    <aside class="pixel-sidebar">
      <div class="pixel-sidebar-header">
        <div class="pixel-brand">
          <div class="pixel-brand-icon">
            <img src="/favicon.svg" alt="PixelDrop" style="width: 24px; height: 24px; image-rendering: pixelated;" />
          </div>
          <div style="display: flex; flex-direction: column;">
            <span class="pixel-brand-title">PixelDrop</span>
            <span style="font-size: 0.62rem; font-weight: 800; color: #868e96; margin-top: 1px;">闪传像素 · 文件床</span>
          </div>
        </div>

        <!-- 局域网 IP 状态板 -->
        <div
          v-if="serverInfo.ips.length > 0"
          class="pixel-ip-badge"
          title="点击复制访问地址"
          @click="copyHostIp"
        >
          <div class="pixel-ip-text">
            <span class="pixel-dot"></span>
            <span>{{ serverInfo.ips[0] }}:{{ serverInfo.port }}</span>
          </div>
          <Copy :size="12" />
        </div>
      </div>

      <!-- 像素导航菜单 (极简核心三大板块) -->
      <nav class="pixel-nav">
        <button
          class="pixel-nav-item"
          :class="{ active: currentMenu === 'files' }"
          @click="currentMenu = 'files'"
        >
          <span v-if="currentMenu === 'files'">▶</span>
          <Folder v-else :size="16" />
          <span>文件互传</span>
        </button>

        <button
          class="pixel-nav-item"
          :class="{ active: currentMenu === 'clipboard' }"
          @click="currentMenu = 'clipboard'"
        >
          <span v-if="currentMenu === 'clipboard'">▶</span>
          <ClipboardList v-else :size="16" />
          <span>共享剪贴板</span>
        </button>

        <button
          class="pixel-nav-item"
          :class="{ active: currentMenu === 'devices' }"
          @click="currentMenu = 'devices'"
        >
          <span v-if="currentMenu === 'devices'">▶</span>
          <Smartphone v-else :size="16" />
          <span>设备连接</span>
        </button>
      </nav>

      <!-- 底部扫码接入 -->
      <div style="padding-top: 14px; border-top: 3px dashed var(--pixel-gray);">
        <button class="pixel-btn" style="width: 100%; padding: 8px;" @click="showDeviceConnectQR">
          <QrCode :size="14" style="margin-right: 6px;" />
          <span>手机扫码接入</span>
        </button>
      </div>
    </aside>

    <!-- 2. 右侧像素 Main Dashboard (居中容器) -->
    <main class="pixel-main">
      <div class="pixel-main-content">
        <template v-if="currentMenu === 'files'">
          <!-- 上半部分：复古像素拖拽上传区 -->
          <section
            class="pixel-dropzone"
            :class="{ 'active-drag': isDragging }"
            @dragover.prevent="isDragging = true"
            @dragleave.prevent="isDragging = false"
            @drop.prevent="onDrop"
            @click="fileInputRef?.click()"
          >
            <input
              type="file"
              ref="fileInputRef"
              style="display: none;"
              multiple
              @change="onFileSelect"
            />
            <div class="pixel-dropzone-icon">
              <UploadCloud :size="24" />
            </div>
            <div class="pixel-dropzone-title">拖拽文件到此处，或点击浏览</div>
            <div class="pixel-dropzone-desc">
              支持任意类型文件 · 单文件无上限流式秒传 · 支持快捷键 Cmd/Ctrl+V 快速粘贴
            </div>
          </section>

          <!-- 下半部分：双栏像素卡片并排（严格视觉对齐） -->
          <div class="pixel-grid-cols">
            <!-- 左栏：正在传输 (Active Transfers) -->
            <div class="pixel-card" :class="{ 'active-transfer-card-empty': activeTransfers.length === 0 }">
              <div class="pixel-card-header">
                <div class="pixel-card-title">
                  <span>正在传输</span>
                  <span class="pixel-badge">{{ activeTransfers.length }}</span>
                </div>
                <span style="font-size: 0.72rem; font-weight: bold; color: #6c757d;">
                  {{ activeTransfers.length > 0 ? '传输队列中' : '空闲就绪' }}
                </span>
              </div>

              <div class="pixel-card-body">
                <div v-if="activeTransfers.length === 0" class="pixel-empty">
                  <Radio :size="24" style="opacity: 0.4; margin-bottom: 8px;" />
                  <div style="font-size: 0.84rem; font-weight: bold; color: var(--pixel-dark);">当前暂无传输任务</div>
                  <div style="font-size: 0.72rem; color: #868e96; margin-top: 3px;">
                    拖入或选择文件后将在此显示进度
                  </div>
                </div>

                <!-- 传输任务 -->
                <div
                  v-for="task in activeTransfers"
                  :key="task.id"
                  class="pixel-transfer-item"
                >
                  <div class="pixel-transfer-header">
                    <span class="pixel-transfer-name" :title="task.name">{{ task.name }}</span>
                    <span class="pixel-transfer-stats">
                      <span>{{ task.speed }}</span>
                      <span>•</span>
                      <span>{{ task.progress }}%</span>
                    </span>
                  </div>

                  <!-- 像素血条 / 经验条风格实时进度槽 -->
                  <div class="pixel-progress-wrap">
                    <div
                      class="pixel-progress-bar-fill"
                      :style="{ width: `${task.progress}%` }"
                      :class="{ 'is-error': task.status === 'error' }"
                    ></div>
                    <div class="pixel-progress-text">{{ task.progress }}%</div>
                  </div>

                  <div style="display: flex; justify-content: space-between; font-size: 0.7rem; font-family: monospace; color: #6c757d;">
                    <span>大小: {{ formatBytes(task.size) }}</span>
                    <span>{{ task.eta }}</span>
                  </div>
                </div>
              </div>
            </div>

            <!-- 右栏：历史文件 (Recent Files) -->
            <div class="pixel-card">
              <div class="pixel-card-header">
                <div class="pixel-card-title">
                  <span>历史文件</span>
                  <span class="pixel-badge">{{ displayFiles.length }}</span>
                </div>

                <div style="display: flex; align-items: center; gap: 6px;">
                  <div style="position: relative; display: flex; align-items: center;">
                    <Search :size="12" style="position: absolute; left: 8px; color: #868e96; pointer-events: none;" />
                    <input
                      v-model="searchQuery"
                      type="text"
                      placeholder="搜索文件名..."
                      class="pixel-input"
                    />
                  </div>
                  <button
                    class="pixel-btn pixel-btn-icon"
                    title="刷新文件"
                    @click="loadFiles"
                  >
                    <RefreshCw :size="12" :class="{ 'spin-anim': loading }" />
                  </button>
                </div>
              </div>

              <div class="pixel-card-body">
                <div v-if="displayFiles.length === 0" class="pixel-empty">
                  <Folder :size="24" style="opacity: 0.4; margin-bottom: 8px;" />
                  <div style="font-size: 0.84rem; font-weight: bold; color: var(--pixel-dark);">暂无历史文件</div>
                  <div style="font-size: 0.72rem; color: #868e96; margin-top: 3px;">
                    通过上方区域上传文件
                  </div>
                </div>

                <!-- 历史文件行 -->
                <div
                  v-for="file in pagedFiles"
                  :key="file.name"
                  class="pixel-file-row"
                >
                  <div class="pixel-file-left">
                    <div class="pixel-file-icon">
                      <ImageIcon v-if="getFileCategory(file.name, file.mimeType) === 'image'" :size="16" />
                      <VideoIcon v-else-if="getFileCategory(file.name, file.mimeType) === 'video'" :size="16" />
                      <MusicIcon v-else-if="getFileCategory(file.name, file.mimeType) === 'audio'" :size="16" />
                      <ArchiveIcon v-else-if="getFileCategory(file.name, file.mimeType) === 'archive'" :size="16" />
                      <FileCodeIcon v-else-if="getFileCategory(file.name, file.mimeType) === 'code'" :size="16" />
                      <FileText v-else-if="['pdf', 'doc'].includes(getFileCategory(file.name, file.mimeType))" :size="16" />
                      <GenericFile v-else :size="16" />
                    </div>

                    <div class="pixel-file-details">
                      <div class="pixel-file-name" :title="file.name">{{ file.name }}</div>
                      <div class="pixel-file-meta">
                        <span>{{ file.humanSize }}</span>
                        <span>•</span>
                        <span>{{ formatTime(file.modTime) }}</span>
                      </div>
                    </div>
                  </div>

                  <div class="pixel-file-actions">
                    <!-- 预览 -->
                    <button
                      v-if="['image', 'video', 'audio'].includes(getFileCategory(file.name, file.mimeType))"
                      class="pixel-btn pixel-btn-icon"
                      title="在线预览"
                      @click="openPreview(file)"
                    >
                      <Eye :size="13" />
                    </button>

                    <!-- 复制下载链接 -->
                    <button
                      class="pixel-btn pixel-btn-icon"
                      title="复制下载链接"
                      @click="copyDownloadLink(file)"
                    >
                      <Copy :size="13" />
                    </button>

                    <!-- 二维码 (移动端隐藏以给操作腾出宽裕空间) -->
                    <button
                      class="pixel-btn pixel-btn-icon btn-hide-on-mobile"
                      title="手机扫码下载"
                      @click="showQR(file)"
                    >
                      <QrCode :size="13" />
                    </button>

                    <!-- 下载 -->
                    <button
                      class="pixel-btn pixel-btn-icon"
                      title="下载文件"
                      @click="downloadFile(file)"
                    >
                      <Download :size="13" />
                    </button>

                    <!-- 删除 -->
                    <button
                      class="pixel-btn pixel-btn-icon pixel-btn-danger"
                      title="删除文件"
                      @click="deleteFile(file)"
                    >
                      <Trash2 :size="13" />
                    </button>
                  </div>
                </div>
              </div>

              <!-- 历史文件专属像素分页器 (复古掌机风格) -->
              <div v-if="displayFiles.length > 0" class="pixel-card-footer pixel-pagination">
                <button
                  class="pixel-btn pixel-page-btn"
                  :disabled="currentPage <= 1"
                  @click="prevPage"
                  title="上一页"
                >
                  <span>◀ 上一页</span>
                </button>

                <div class="pixel-page-info">
                  <span class="pixel-page-current">第 {{ currentPage }} / {{ totalPages }} 页</span>
                  <span class="pixel-page-count">（共 {{ displayFiles.length }} 项）</span>
                </div>

                <button
                  class="pixel-btn pixel-page-btn"
                  :disabled="currentPage >= totalPages"
                  @click="nextPage"
                  title="下一页"
                >
                  <span>下一页 ▶</span>
                </button>
              </div>
            </div>
          </div>
        </template>

        <!-- 视图 B: 共享剪贴板 (独立工作台) -->
        <template v-else-if="currentMenu === 'clipboard'">
          <div class="pixel-card pixel-card-standalone" style="padding: 20px; gap: 16px;">
            <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px;">
              <div>
                <h2 style="font-size: 1.1rem; font-weight: 800;">局域网共享剪贴板</h2>
                <p style="font-size: 0.8rem; color: #6c757d; margin-top: 4px;">
                  跨设备互传文字、URL、Token 或备忘 · 任何设备保存即可全网实时推送
                  <span v-if="clipboardUpdatedAt">（最后同步: {{ formatTime(clipboardUpdatedAt) }}）</span>
                </p>
              </div>

              <!-- 实时推送指示灯 -->
              <span
                class="pixel-badge"
                :style="{
                  backgroundColor: isSseConnected ? '#92cc41' : '#868e96',
                  color: isSseConnected ? '#212529' : '#ffffff'
                }"
              >
                {{ isSseConnected ? '● 毫秒级即时推送中' : '○ 推送就绪' }}
              </span>
            </div>

            <div>
              <textarea
                v-model="clipboardContent"
                placeholder="在此粘贴网址、文字、代码、Token 或备忘，点击下方保存即可实时推送到所有设备..."
                style="width: 100%; height: 240px; padding: 12px; border: 3px solid var(--pixel-dark); box-shadow: 3px 3px 0 var(--pixel-dark); outline: none; font-family: inherit; font-size: 0.85rem; line-height: 1.6; resize: vertical;"
              ></textarea>

              <div style="display: flex; gap: 10px; margin-top: 14px; justify-content: flex-end; flex-wrap: wrap;">
                <button
                  class="pixel-btn"
                  style="padding: 8px 14px;"
                  :disabled="!clipboardContent"
                  @click="copyClipboardText"
                >
                  <Copy :size="13" style="margin-right: 6px;" />
                  <span>复制全部文本</span>
                </button>

                <button
                  class="pixel-btn"
                  style="padding: 8px 14px;"
                  :disabled="!clipboardContent"
                  @click="clearClipboard"
                >
                  <span>清空</span>
                </button>

                <button class="pixel-btn" style="padding: 8px 14px;" @click="loadClipboard">
                  <RefreshCw :size="12" style="margin-right: 6px;" />
                  <span>刷新</span>
                </button>

                <button
                  class="pixel-btn pixel-btn-primary"
                  style="padding: 8px 16px;"
                  :disabled="clipboardSaving"
                  @click="saveClipboard"
                >
                  <Check :size="13" style="margin-right: 6px;" />
                  <span>{{ clipboardSaving ? '同步中...' : '保存并实时广播' }}</span>
                </button>
              </div>
            </div>
          </div>
        </template>

        <!-- 视图 C: 设备连接与服务信息 -->
        <template v-else-if="currentMenu === 'devices'">
          <div class="pixel-card pixel-card-standalone pixel-device-card">
            <div>
              <h2 class="pixel-device-title">局域网设备连接 & 服务状态</h2>
              <p class="pixel-device-subtitle">
                同一局域网 Wi-Fi 下的手机、电脑、平板等均可免登录直接连线互传
              </p>
            </div>

            <div class="pixel-device-body">
              <div class="pixel-qr-wrapper">
                <qrcode-vue :value="getBaseUrl()" :size="isMobile ? 150 : 190" level="M" />
              </div>

              <div class="pixel-device-info">
                <div class="pixel-device-address-box">
                  <span class="pixel-label">本机局域网服务地址</span>
                  <div class="pixel-device-url">
                    {{ getBaseUrl() }}
                  </div>
                </div>

                <div class="pixel-device-stat-grid">
                  <div class="pixel-stat-box">
                    <div class="pixel-stat-label">服务端口</div>
                    <div class="pixel-stat-value">{{ serverInfo.port }}</div>
                  </div>

                  <div class="pixel-stat-box">
                    <div class="pixel-stat-label">存储落盘目录</div>
                    <div class="pixel-stat-value">./uploads</div>
                  </div>
                </div>

                <button class="pixel-btn pixel-btn-primary pixel-device-btn" @click="copyHostIp">
                  <Copy :size="13" style="margin-right: 6px;" />
                  <span>复制接入地址</span>
                </button>
              </div>
            </div>
          </div>
        </template>
      </div>
    </main>

    <!-- 移动端专属吸底导航栏 (Mobile Bottom Bar) -->
    <nav class="pixel-mobile-bottom-nav">
      <button
        class="pixel-bottom-nav-item"
        :class="{ active: currentMenu === 'files' }"
        @click="currentMenu = 'files'"
      >
        <Folder :size="18" />
        <span>文件互传</span>
      </button>

      <button
        class="pixel-bottom-nav-item"
        :class="{ active: currentMenu === 'clipboard' }"
        @click="currentMenu = 'clipboard'"
      >
        <ClipboardList :size="18" />
        <span>共享文本</span>
      </button>

      <button
        class="pixel-bottom-nav-item"
        :class="{ active: currentMenu === 'devices' }"
        @click="currentMenu = 'devices'"
      >
        <Smartphone :size="18" />
        <span>设备连线</span>
      </button>
    </nav>

    <!-- 弹窗：二维码 (像素对话框) -->
    <div v-if="qrModal.visible" class="pixel-modal-mask" @click.self="qrModal.visible = false">
      <div class="pixel-modal-card">
        <div class="pixel-modal-header">
          <h3 style="font-size: 0.95rem; font-weight: 800;">{{ qrModal.title }}</h3>
          <button class="pixel-btn pixel-btn-icon" @click="qrModal.visible = false">
            <X :size="14" />
          </button>
        </div>

        <div style="display: flex; flex-direction: column; align-items: center; padding: 14px 0;">
          <div style="padding: 10px; border: 3px solid var(--pixel-dark); box-shadow: 4px 4px 0 var(--pixel-dark); background: #ffffff;">
            <qrcode-vue :value="qrModal.url" :size="190" level="M" />
          </div>
          <p style="margin-top: 14px; font-size: 0.82rem; font-weight: bold; color: var(--pixel-dark); text-align: center;">
            {{ qrModal.tip }}
          </p>
          <p style="font-size: 0.74rem; color: #6c757d; margin-top: 4px; word-break: break-all; max-width: 90%; text-align: center; font-family: monospace;">
            {{ qrModal.url }}
          </p>
        </div>
      </div>
    </div>

    <!-- 弹窗：在线预览 (像素大图视窗) -->
    <div v-if="previewModal.visible" class="pixel-modal-mask" @click.self="previewModal.visible = false">
      <div class="pixel-modal-card" style="max-width: 760px;">
        <div class="pixel-modal-header">
          <h3 style="font-size: 0.9rem; font-weight: 800; max-width: 85%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
            {{ previewModal.name }}
          </h3>
          <button class="pixel-btn pixel-btn-icon" @click="previewModal.visible = false">
            <X :size="14" />
          </button>
        </div>

        <div style="display: flex; align-items: center; justify-content: center; max-height: 65vh; overflow: auto; padding: 8px 0;">
          <img
            v-if="previewModal.category === 'image'"
            :src="previewModal.url"
            :alt="previewModal.name"
            style="max-width: 100%; max-height: 60vh; object-fit: contain; border: 2px solid var(--pixel-dark);"
          />
          <video
            v-else-if="previewModal.category === 'video'"
            controls
            autoplay
            :src="previewModal.url"
            style="width: 100%; max-height: 60vh; border: 2px solid var(--pixel-dark);"
          ></video>
          <audio
            v-else-if="previewModal.category === 'audio'"
            controls
            autoplay
            :src="previewModal.url"
            style="width: 100%;"
          ></audio>
        </div>
      </div>
    </div>

    <!-- 像素风格 Toast -->
    <div class="pixel-toast-shelf">
      <div v-for="t in toasts" :key="t.id" class="pixel-toast-box">
        <span>{{ t.message }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.spin-anim {
  animation: spin 1s linear infinite;
}
@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
