<template>
  <div class="home-page">
    <!-- 欢迎横幅 -->
    <el-card class="welcome-card" shadow="never">
      <div class="welcome-content">
        <div class="welcome-text">
          <h1 class="welcome-title">
            <span class="greeting">{{ greeting }}</span>
            <span class="wave">👋</span>
          </h1>
          <p class="welcome-desc">欢迎回来，这里是 Ollama 服务聚合网关控制台</p>
        </div>
        <div class="welcome-stats">
          <div class="quick-stat">
            <span class="stat-number">{{ stats.totalServices }}</span>
            <span class="stat-text">总服务</span>
          </div>
          <div class="quick-stat">
            <span class="stat-number">{{ stats.onlineServices }}</span>
            <span class="stat-text">在线</span>
          </div>
          <div class="quick-stat">
            <span class="stat-number">{{ stats.totalModels }}</span>
            <span class="stat-text">模型</span>
          </div>
        </div>
      </div>
    </el-card>

    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :xs="12" :sm="12" :md="6" :lg="6" :xl="6">
        <div class="stat-card primary">
          <div class="stat-icon">
            <el-icon :size="32"><Connection /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats.totalServices }}</div>
            <div class="stat-label">总服务数</div>
            <div class="stat-trend" :class="{ up: stats.serviceTrend > 0 }">
              <el-icon><TrendCharts /></el-icon>
              <span>{{ stats.serviceTrend >= 0 ? '+' : '' }}{{ stats.serviceTrend }}</span>
            </div>
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6" :lg="6" :xl="6">
        <div class="stat-card success">
          <div class="stat-icon">
            <el-icon :size="32"><CircleCheck /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats.onlineServices }}</div>
            <div class="stat-label">在线服务</div>
            <div class="stat-trend up">
              <el-icon><TrendCharts /></el-icon>
              <span>{{ onlinePercentage }}%</span>
            </div>
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6" :lg="6" :xl="6">
        <div class="stat-card warning">
          <div class="stat-icon">
            <el-icon :size="32"><Files /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats.totalModels }}</div>
            <div class="stat-label">可用模型</div>
            <div class="stat-trend up">
              <el-icon><TrendCharts /></el-icon>
              <span>{{ stats.availableModels }}</span>
            </div>
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6" :lg="6" :xl="6">
        <div class="stat-card danger">
          <div class="stat-icon">
            <el-icon :size="32"><Warning /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats.honeypotServices }}</div>
            <div class="stat-label">蜜罐服务</div>
            <div class="stat-trend">
              <el-icon><WarnTriangleFilled /></el-icon>
              <span>需注意</span>
            </div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 主要内容区 -->
    <el-row :gutter="20" class="content-row">
      <!-- 服务状态 -->
      <el-col :xs="24" :sm="24" :md="16" :lg="16" :xl="16">
        <el-card class="content-card" shadow="never">
          <template #header>
            <div class="card-header">
              <div class="header-title">
                <el-icon :size="20" color="#4f46e5"><Monitor /></el-icon>
                <span>服务状态监控</span>
              </div>
              <el-button-group>
                <el-button size="small" :type="statusView === 'list' ? 'primary' : ''" @click="statusView = 'list'">
                  <el-icon><List /></el-icon>
                </el-button>
                <el-button size="small" :type="statusView === 'grid' ? 'primary' : ''" @click="statusView = 'grid'">
                  <el-icon><Grid /></el-icon>
                </el-button>
              </el-button-group>
            </div>
          </template>

          <div v-if="statusView === 'list'" class="service-list">
            <div v-for="service in recentServices" :key="service.id" class="service-item">
              <div class="service-info">
                <el-avatar :size="40" :icon="Server" :class="`status-${service.status}`" />
                <div class="service-detail">
                  <div class="service-name">{{ service.name || service.url }}</div>
                  <div class="service-url">{{ service.url }}</div>
                </div>
              </div>
              <div class="service-meta">
                <el-tag :type="getStatusType(service.status)" effect="light" size="small">
                  {{ getStatusText(service.status) }}
                </el-tag>
                <span class="response-time">{{ formatResponseTime(service.response_time) }}</span>
              </div>
            </div>
            <el-empty v-if="recentServices.length === 0" description="暂无服务数据" :image-size="120" />
          </div>

          <div v-else class="service-grid">
            <div v-for="service in recentServices" :key="service.id" class="service-card-mini">
              <div class="service-card-header" :class="`status-${service.status}`">
                <el-icon :size="24"><Connection /></el-icon>
              </div>
              <div class="service-card-body">
                <div class="service-card-name">{{ service.name || '未命名' }}</div>
                <div class="service-card-url">{{ formatUrl(service.url) }}</div>
                <el-tag :type="getStatusType(service.status)" effect="plain" size="small">
                  {{ getStatusText(service.status) }}
                </el-tag>
              </div>
            </div>
            <el-empty v-if="recentServices.length === 0" description="暂无服务数据" :image-size="120" />
          </div>
        </el-card>
      </el-col>

      <!-- 快捷操作 & 最近活动 -->
      <el-col :xs="24" :sm="24" :md="8" :lg="8" :xl="8">
        <!-- 快捷操作 -->
        <el-card class="content-card quick-actions-card" shadow="never">
          <template #header>
            <div class="card-header">
              <div class="header-title">
                <el-icon :size="20" color="#10b981"><Lightning /></el-icon>
                <span>快捷操作</span>
              </div>
            </div>
          </template>
          <div class="quick-actions">
            <button class="action-btn" @click="$router.push('/services')">
              <div class="action-icon primary">
                <el-icon :size="24"><Plus /></el-icon>
              </div>
              <span>添加服务</span>
            </button>
            <button class="action-btn" @click="$router.push('/discovery')">
              <div class="action-icon success">
                <el-icon :size="24"><Search /></el-icon>
              </div>
              <span>服务发现</span>
            </button>
            <button class="action-btn" @click="$router.push('/models')">
              <div class="action-icon warning">
                <el-icon :size="24"><Files /></el-icon>
              </div>
              <span>模型管理</span>
            </button>
            <button class="action-btn" @click="$router.push('/settings')">
              <div class="action-icon danger">
                <el-icon :size="24"><Setting /></el-icon>
              </div>
              <span>系统设置</span>
            </button>
          </div>
        </el-card>

        <!-- 最近活动 -->
        <el-card class="content-card activity-card" shadow="never">
          <template #header>
            <div class="card-header">
              <div class="header-title">
                <el-icon :size="20" color="#f59e0b"><Clock /></el-icon>
                <span>最近活动</span>
              </div>
            </div>
          </template>
          <div class="activity-list">
            <div v-for="activity in recentActivities" :key="activity.id" class="activity-item">
              <div class="activity-icon" :class="activity.type">
                <el-icon :size="16">
                  <Plus v-if="activity.type === 'add'" />
                  <Refresh v-else-if="activity.type === 'check'" />
                  <Delete v-else-if="activity.type === 'delete'" />
                  <Warning v-else />
                </el-icon>
              </div>
              <div class="activity-content">
                <div class="activity-text">{{ activity.text }}</div>
                <div class="activity-time">{{ formatTime(activity.time) }}</div>
              </div>
            </div>
            <el-empty v-if="recentActivities.length === 0" description="暂无活动记录" :image-size="100" />
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 系统信息 -->
    <el-row :gutter="20" class="info-row">
      <el-col :span="24">
        <el-card class="content-card system-info-card" shadow="never">
          <template #header>
            <div class="card-header">
              <div class="header-title">
                <el-icon :size="20" color="#6366f1"><InfoFilled /></el-icon>
                <span>系统信息</span>
              </div>
            </div>
          </template>
          <el-descriptions :column="4" border>
            <el-descriptions-item label="系统版本">v1.0.0</el-descriptions-item>
            <el-descriptions-item label="运行时间">{{ uptime }}</el-descriptions-item>
            <el-descriptions-item label="监听地址">0.0.0.0:11435</el-descriptions-item>
            <el-descriptions-item label="认证状态">
              <el-tag type="success" effect="plain">已启用</el-tag>
            </el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import api from '@/api/client'

const statusView = ref<'list' | 'grid'>('list')

const stats = reactive({
  totalServices: 0,
  onlineServices: 0,
  totalModels: 0,
  availableModels: 0,
  honeypotServices: 0,
  serviceTrend: 0,
})

const recentServices = ref<any[]>([])
const recentActivities = ref<any[]>([
  { id: 1, type: 'add', text: '添加新服务 localhost:11434', time: new Date(Date.now() - 300000) },
  { id: 2, type: 'check', text: '完成服务健康检查', time: new Date(Date.now() - 600000) },
  { id: 3, type: 'delete', text: '删除离线服务 192.168.1.100', time: new Date(Date.now() - 1800000) },
  { id: 4, type: 'warning', text: '检测到蜜罐服务', time: new Date(Date.now() - 3600000) },
])

let uptimeTimer: any = null
const uptimeSeconds = ref(0)

const greeting = computed(() => {
  const hour = new Date().getHours()
  if (hour < 6) return '夜深了'
  if (hour < 12) return '早上好'
  if (hour < 14) return '中午好'
  if (hour < 18) return '下午好'
  return '晚上好'
})

const onlinePercentage = computed(() => {
  if (stats.totalServices === 0) return 0
  return Math.round((stats.onlineServices / stats.totalServices) * 100)
})

const uptime = computed(() => {
  const days = Math.floor(uptimeSeconds.value / 86400)
  const hours = Math.floor((uptimeSeconds.value % 86400) / 3600)
  const minutes = Math.floor((uptimeSeconds.value % 3600) / 60)
  if (days > 0) return `${days}天 ${hours}小时 ${minutes}分钟`
  if (hours > 0) return `${hours}小时 ${minutes}分钟`
  return `${minutes}分钟`
})

const loadStats = async () => {
  try {
    const res = await api.get('/services', { params: { limit: 100 } })
    const services = res.data.data || []
    stats.totalServices = res.data.total || services.length
    // 使用统计数据 API
    const statsRes = await api.get('/services/stats')
    const statsData = statsRes.data.data || {}
    stats.onlineServices = statsData.online || 0
    stats.honeypotServices = statsData.honeypot || 0
    recentServices.value = services.slice(0, 6)
  } catch (error) {
    console.error('加载统计数据失败:', error)
  }
}

const loadModels = async () => {
  try {
    const res = await api.get('/models')
    const models = res.data.data || []
    stats.totalModels = models.length
    stats.availableModels = models.filter((m: any) => m.is_available).length
  } catch (error) {
    console.error('加载模型数据失败:', error)
  }
}

const getStatusType = (status: string) => {
  const types: any = {
    online: 'success',
    offline: 'danger',
    honeypot: 'warning',
    unknown: 'info',
  }
  return types[status] || 'info'
}

const getStatusText = (status: string) => {
  const texts: any = {
    online: '在线',
    offline: '离线',
    honeypot: '蜜罐',
    unknown: '未知',
  }
  return texts[status] || status
}

const formatResponseTime = (ms: number) => {
  if (!ms) return '-'
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

const formatUrl = (url: string) => {
  if (!url) return '-'
  try {
    const u = new URL(url)
    return u.hostname + (u.port ? ':' + u.port : '')
  } catch {
    return url.length > 25 ? url.substring(0, 25) + '...' : url
  }
}

const formatTime = (date: Date) => {
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const minutes = Math.floor(diff / 60000)
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes}分钟前`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}小时前`
  const days = Math.floor(hours / 24)
  return `${days}天前`
}

onMounted(() => {
  loadStats()
  loadModels()
  uptimeSeconds.value = 7200 // 模拟运行时间
  uptimeTimer = setInterval(() => {
    uptimeSeconds.value++
  }, 1000)
})

onUnmounted(() => {
  if (uptimeTimer) clearInterval(uptimeTimer)
})
</script>

<style scoped>
.home-page {
  height: 100%;
  padding-bottom: 20px;
}

/* 欢迎卡片 */
.welcome-card {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border: none;
  margin-bottom: 24px;
  border-radius: 16px;
  overflow: hidden;
}

.welcome-card :deep(.el-card__body) {
  padding: 32px;
}

.welcome-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  color: #fff;
}

.welcome-text {
  flex: 1;
}

.welcome-title {
  font-size: 32px;
  font-weight: 700;
  margin: 0 0 8px 0;
  display: flex;
  align-items: center;
  gap: 12px;
}

.greeting {
  color: #fff;
}

.wave {
  animation: wave 2s infinite;
  display: inline-block;
}

@keyframes wave {
  0%, 100% { transform: rotate(0deg); }
  25% { transform: rotate(20deg); }
  75% { transform: rotate(-20deg); }
}

.welcome-desc {
  font-size: 16px;
  color: rgba(255, 255, 255, 0.85);
  margin: 0;
}

.welcome-stats {
  display: flex;
  gap: 32px;
}

.quick-stat {
  text-align: center;
}

.stat-number {
  display: block;
  font-size: 36px;
  font-weight: 700;
  line-height: 1;
  margin-bottom: 4px;
}

.stat-text {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.7);
}

/* 统计行 */
.stats-row {
  margin-bottom: 24px;
}

.stat-card {
  background: #fff;
  border-radius: 16px;
  padding: 24px;
  display: flex;
  align-items: center;
  gap: 20px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
  cursor: pointer;
  height: 100%;
}

.stat-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
}

.stat-card.primary .stat-icon {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.stat-card.success .stat-icon {
  background: linear-gradient(135deg, #4ade80 0%, #22c55e 100%);
}

.stat-card.warning .stat-icon {
  background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%);
}

.stat-card.danger .stat-icon {
  background: linear-gradient(135deg, #f87171 0%, #dc2626 100%);
}

.stat-icon {
  width: 64px;
  height: 64px;
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  flex-shrink: 0;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.stat-content {
  flex: 1;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: #1e293b;
  line-height: 1;
  margin-bottom: 4px;
}

.stat-label {
  font-size: 13px;
  color: #64748b;
  font-weight: 500;
  margin-bottom: 4px;
}

.stat-trend {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: #64748b;
}

.stat-trend.up {
  color: #10b981;
}

/* 内容行 */
.content-row {
  margin-bottom: 24px;
}

.content-card {
  border-radius: 12px;
  height: 100%;
  margin-bottom: 20px;
}

.content-card :deep(.el-card__header) {
  padding: 16px 20px;
  border-bottom: 1px solid #f1f5f9;
}

.content-card :deep(.el-card__body) {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
}

/* 服务列表 */
.service-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.service-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background: #f8fafc;
  border-radius: 12px;
  transition: all 0.3s ease;
  cursor: pointer;
}

.service-item:hover {
  background: #f1f5f9;
  transform: translateX(4px);
}

.service-info {
  display: flex;
  align-items: center;
  gap: 16px;
}

.service-info .el-avatar {
  border-radius: 12px;
}

.service-info .el-avatar.status-online {
  background: linear-gradient(135deg, #4ade80 0%, #22c55e 100%);
  color: #fff;
}

.service-info .el-avatar.status-offline {
  background: linear-gradient(135deg, #f87171 0%, #dc2626 100%);
  color: #fff;
}

.service-info .el-avatar.status-honeypot {
  background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%);
  color: #fff;
}

.service-info .el-avatar.status-unknown {
  background: linear-gradient(135deg, #94a3b8 0%, #64748b 100%);
  color: #fff;
}

.service-detail {
  flex: 1;
}

.service-name {
  font-weight: 600;
  color: #1e293b;
  margin-bottom: 4px;
}

.service-url {
  font-size: 12px;
  color: #94a3b8;
}

.service-meta {
  display: flex;
  align-items: center;
  gap: 12px;
}

.response-time {
  font-size: 12px;
  color: #64748b;
}

/* 服务网格 */
.service-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
}

.service-card-mini {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  overflow: hidden;
  transition: all 0.3s ease;
  cursor: pointer;
}

.service-card-mini:hover {
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
  transform: translateY(-4px);
}

.service-card-header {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.service-card-header.status-online {
  background: linear-gradient(135deg, #4ade80 0%, #22c55e 100%);
}

.service-card-header.status-offline {
  background: linear-gradient(135deg, #f87171 0%, #dc2626 100%);
}

.service-card-header.status-honeypot {
  background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%);
}

.service-card-header.status-unknown {
  background: linear-gradient(135deg, #94a3b8 0%, #64748b 100%);
}

.service-card-body {
  padding: 16px;
  text-align: center;
}

.service-card-name {
  font-weight: 600;
  color: #1e293b;
  margin-bottom: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.service-card-url {
  font-size: 12px;
  color: #94a3b8;
  margin-bottom: 12px;
}

/* 快捷操作 */
.quick-actions {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.action-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 20px 16px;
  background: #f8fafc;
  border: 2px solid transparent;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.3s ease;
  font-size: 13px;
  font-weight: 500;
  color: #475569;
}

.action-btn:hover {
  background: #fff;
  border-color: #4f46e5;
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(79, 70, 229, 0.15);
}

.action-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.action-icon.primary {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.action-icon.success {
  background: linear-gradient(135deg, #4ade80 0%, #22c55e 100%);
}

.action-icon.warning {
  background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%);
}

.action-icon.danger {
  background: linear-gradient(135deg, #f87171 0%, #dc2626 100%);
}

/* 活动列表 */
.activity-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 280px;
  overflow-y: auto;
}

.activity-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px;
  background: #f8fafc;
  border-radius: 8px;
}

.activity-icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: #fff;
}

.activity-icon.add {
  background: linear-gradient(135deg, #4ade80 0%, #22c55e 100%);
}

.activity-icon.check {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.activity-icon.delete {
  background: linear-gradient(135deg, #f87171 0%, #dc2626 100%);
}

.activity-icon.warning {
  background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%);
}

.activity-content {
  flex: 1;
}

.activity-text {
  font-size: 13px;
  color: #334155;
  margin-bottom: 4px;
}

.activity-time {
  font-size: 11px;
  color: #94a3b8;
}

/* 系统信息 */
.system-info-card {
  margin-bottom: 0;
}

/* 响应式 */
@media (max-width: 768px) {
  .welcome-content {
    flex-direction: column;
    gap: 24px;
  }

  .welcome-title {
    font-size: 24px;
  }

  .welcome-stats {
    width: 100%;
    justify-content: space-around;
  }

  .stat-card {
    padding: 16px;
  }

  .stat-icon {
    width: 48px;
    height: 48px;
  }

  .stat-value {
    font-size: 24px;
  }

  .service-grid {
    grid-template-columns: 1fr;
  }

  .quick-actions {
    grid-template-columns: 1fr;
  }
}
</style>
