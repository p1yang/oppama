<template>
  <div class="discovery-page">
    <el-row :gutter="20">
      <!-- 左侧搜索配置 -->
      <el-col :xs="24" :sm="24" :md="14" :lg="14" :xl="14">
        <el-card class="search-card" shadow="never">
          <template #header>
            <div class="card-header">
              <div class="header-title">
                <div class="title-icon primary">
                  <el-icon :size="20"><Search /></el-icon>
                </div>
                <span>服务发现配置</span>
              </div>
            </div>
          </template>

          <!-- 搜索引擎选择 -->
          <div class="form-section">
            <div class="section-label">
              <el-icon><Platform /></el-icon>
              <span>选择搜索引擎</span>
            </div>
            <div class="engines-grid">
              <div
                v-for="engine in engines"
                :key="engine.key"
                class="engine-option"
                :class="{
                  active: searchForm.engines.includes(engine.key),
                  disabled: !engine.enabled
                }"
                @click="toggleEngine(engine.key, engine.enabled)"
              >
                <div class="engine-check" v-if="searchForm.engines.includes(engine.key)">
                  <el-icon><Check /></el-icon>
                </div>
                <div class="engine-icon" :style="{ background: engine.color }">
                  <span class="engine-emoji">{{ engine.emoji }}</span>
                </div>
                <div class="engine-info">
                  <div class="engine-name">{{ engine.name }}</div>
                  <div class="engine-status" :class="{ enabled: engine.enabled }">
                    {{ engine.enabled ? '已配置' : '未配置' }}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- 搜索语法 -->
          <div class="form-section">
            <div class="section-label">
              <el-icon><EditPen /></el-icon>
              <span>搜索语法</span>
            </div>
            <el-input
              v-model="searchForm.query"
              placeholder='app="Ollama"'
              class="query-input"
            >
              <template #prepend>
                <el-icon><Search /></el-icon>
              </template>
              <template #append>
                <el-button @click="useDefaultQuery">
                  <el-icon><Document /></el-icon>
                  默认语法
                </el-button>
              </template>
            </el-input>
            <div class="query-tips">
              <el-text size="small" type="info">
                <el-icon><InfoFilled /></el-icon>
                支持 FOFA、Hunter、ZoomEye 等搜索引擎的查询语法
              </el-text>
            </div>
          </div>

          <!-- 结果数量 -->
          <div class="form-section">
            <div class="section-label">
              <el-icon><Histogram /></el-icon>
              <span>最大结果数</span>
            </div>
            <el-slider
              v-model="searchForm.maxResults"
              :min="10"
              :max="1000"
              :step="10"
              :marks="{ 100: '100', 500: '500', 1000: '1000' }"
              show-input
            />
          </div>

          <!-- 操作按钮 -->
          <div class="action-buttons">
            <el-button
              type="primary"
              size="large"
              @click="startSearch"
              :loading="searching"
              :disabled="searchForm.engines.length === 0"
            >
              <el-icon><Search /></el-icon>
              {{ searching ? '搜索中...' : '开始搜索' }}
            </el-button>
            <el-button size="large" @click="resetSearch" :icon="RefreshLeft">
              重置
            </el-button>
          </div>
        </el-card>
      </el-col>

      <!-- 右侧任务状态 -->
      <el-col :xs="24" :sm="24" :md="10" :lg="10" :xl="10">
        <el-card class="task-card" shadow="never">
          <template #header>
            <div class="card-header">
              <div class="header-title">
                <div class="title-icon success">
                  <el-icon :size="20"><Monitor /></el-icon>
                </div>
                <span>任务状态</span>
              </div>
              <el-button v-if="currentTask" :icon="Refresh" @click="loadRecentServices" circle size="small" />
            </div>
          </template>

          <div v-if="currentTask" class="task-info">
            <!-- 任务状态标签 -->
            <div class="task-status-wrapper">
              <el-tag :type="getTaskStatusType(currentTask.status)" size="large" effect="light" class="status-tag">
                <el-icon class="status-icon">
                  <Loading v-if="currentTask.status === 'running'" />
                  <CircleCheck v-else-if="currentTask.status === 'completed'" />
                  <CircleClose v-else-if="currentTask.status === 'failed'" />
                  <Clock v-else />
                </el-icon>
                {{ getTaskStatusText(currentTask.status) }}
              </el-tag>
            </div>

            <!-- 任务详情 -->
            <div class="task-details">
              <div class="detail-item">
                <span class="label">任务 ID</span>
                <el-tag type="info" effect="plain" size="small">{{ currentTask.id.slice(0, 8) }}</el-tag>
              </div>
              <div class="detail-item">
                <span class="label">进度</span>
                <span class="value">{{ currentTask.progress || 0 }} / {{ currentTask.total || 0 }}</span>
              </div>
              <div class="detail-item">
                <span class="label">发现数量</span>
                <el-tag type="success" effect="plain">{{ currentTask?.result?.found_count || currentTask?.found_count || 0 }}</el-tag>
              </div>
              <div class="detail-item">
                <span class="label">使用引擎</span>
                <span class="value">{{ currentTask.engines?.join(', ') || '-' }}</span>
              </div>
            </div>

            <!-- 进度条 -->
            <div class="progress-wrapper">
              <el-progress
                :percentage="taskProgress"
                :status="currentTask.status === 'completed' ? 'success' : currentTask.status === 'failed' ? 'exception' : undefined"
                :stroke-width="12"
                :striped="currentTask.status === 'running'"
                :striped-flow="currentTask.status === 'running'"
              />
            </div>

            <!-- 操作按钮 -->
            <div v-if="currentTask.status === 'completed'" class="task-actions">
              <el-button type="primary" :icon="View" @click="viewResults">查看结果</el-button>
            </div>
          </div>

          <!-- 空状态 -->
          <div v-else class="empty-task">
            <el-empty description="暂无执行任务" :image-size="140">
              <template #image>
                <div class="empty-icon">
                  <el-icon :size="80"><Search /></el-icon>
                </div>
              </template>
            </el-empty>
          </div>
        </el-card>

        <!-- 搜索历史 -->
        <el-card class="history-card" shadow="never">
          <template #header>
            <div class="card-header">
              <div class="header-title">
                <div class="title-icon warning">
                  <el-icon :size="18"><Clock /></el-icon>
                </div>
                <span>最近搜索</span>
              </div>
              <el-button v-if="searchHistory.length > 0" link type="danger" @click="clearHistory">
                清空
              </el-button>
            </div>
          </template>

          <div class="history-list">
            <div v-for="item in searchHistory.slice(0, 5)" :key="item.id" class="history-item" @click="useHistory(item)">
              <div class="history-query">{{ item.query }}</div>
              <div class="history-meta">
                <span class="history-time">{{ formatTime(item.time) }}</span>
                <el-tag size="small" type="info">{{ item.count }} 结果</el-tag>
              </div>
            </div>
            <el-empty v-if="searchHistory.length === 0" description="暂无搜索历史" :image-size="100" />
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 最近发现的服务 -->
    <el-card class="result-card" shadow="never">
      <template #header>
        <div class="card-header">
          <div class="header-title">
            <div class="title-icon danger">
              <el-icon :size="20"><Star /></el-icon>
            </div>
            <span>最近发现的服务</span>
          </div>
          <el-button @click="loadRecentServices" :icon="Refresh" circle />
        </div>
      </template>

      <el-table
        :data="recentServices"
        stripe
        :header-cell-style="{ background: '#f8fafc', color: '#475569', fontWeight: '600' }"
      >
        <el-table-column prop="url" label="URL" min-width="280">
          <template #default="{ row }">
            <div class="url-cell">
              <el-icon><Link /></el-icon>
              <el-link :href="row.url" target="_blank" type="primary" :underline="false">
                {{ row.url }}
              </el-link>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="source" label="来源" width="120">
          <template #default="{ row }">
            <el-tag :type="getSourceTagType(row.source)" effect="plain" size="large">
              {{ getSourceText(row.source) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'online' ? 'success' : 'info'" effect="light">
              {{ row.status === 'online' ? '在线' : row.status === 'unknown' ? '未知' : '离线' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="发现时间" width="180">
          <template #default="{ row }">
            <span class="time-text">{{ formatDate(row.created_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button size="small" @click="checkService(row.id)" :loading="row.checking">
              <el-icon><Connection /></el-icon>
              检测
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="recentServices.length === 0" description="暂无发现的服务" :image-size="160" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import api from '@/api/client'
import { useTaskStore } from '@/stores/taskStore'

const router = useRouter()
const taskStore = useTaskStore()

const searching = ref(false)
const currentTask = ref<any>(null)
const recentServices = ref<any[]>([])
const searchHistory = ref<any[]>([])

// 从 localStorage 加载搜索历史
const loadSearchHistory = () => {
  try {
    const saved = localStorage.getItem('discovery_search_history')
    if (saved) {
      searchHistory.value = JSON.parse(saved)
    }
  } catch (error) {
    console.error('加载搜索历史失败:', error)
  }
}

// 保存搜索历史到 localStorage
const saveSearchHistory = () => {
  try {
    localStorage.setItem('discovery_search_history', JSON.stringify(searchHistory.value))
  } catch (error) {
    console.error('保存搜索历史失败:', error)
  }
}

// 添加到搜索历史
const addToSearchHistory = (query: string, count: number) => {
  // 移除相同的查询（如果有）
  const index = searchHistory.value.findIndex(h => h.query === query)
  if (index > -1) {
    searchHistory.value.splice(index, 1)
  }

  // 添加到开头
  searchHistory.value.unshift({
    id: Date.now(),
    query,
    time: new Date(),
    count,
  })

  // 限制最多保存 10 条
  if (searchHistory.value.length > 10) {
    searchHistory.value = searchHistory.value.slice(0, 10)
  }

  saveSearchHistory()
}

const engines = ref([
  { key: 'fofa', name: 'FOFA', emoji: '🔍', color: '#4f46e5', enabled: false },
  { key: 'hunter', name: 'Hunter', emoji: '🎯', color: '#10b981', enabled: false },
  { key: 'zoomeye', name: 'ZoomEye', emoji: '👁️', color: '#f59e0b', enabled: false },
  { key: 'shodan', name: 'Shodan', emoji: '🌐', color: '#dc2626', enabled: false },
])

// 加载搜索引擎配置状态
const loadEngineConfig = async () => {
  try {
    const res = await api.get('/proxy/config')
    const searchEngines = res.data.data?.search_engines
    if (searchEngines) {
      engines.value[0].enabled = searchEngines.fofa_enabled ?? false
      engines.value[1].enabled = searchEngines.hunter_enabled ?? false
      engines.value[2].enabled = searchEngines.zoomeye_enabled ?? false
      engines.value[3].enabled = searchEngines.shodan_enabled ?? false

      // 更新默认选中的引擎
      if (searchForm.engines.length === 1 && searchForm.engines[0] === 'fofa') {
        searchForm.engines = engines.value.filter(e => e.enabled).map(e => e.key)
      }
    }
  } catch (error) {
    console.error('加载搜索引擎配置失败:', error)
  }
}

const searchForm = reactive({
  engines: ['fofa'],
  query: 'port="11434"', // Hunter 使用 port 查询更有效
  maxResults: 100,
})

const taskProgress = computed(() => {
  const discoveryTasks = taskStore.getTasksByType('discovery-search')
  if (discoveryTasks.length === 0) return 0
  const task = discoveryTasks[0]
  return task.total > 0 ? Math.round((task.progress / task.total) * 100) : 0
})

const startSearch = async () => {
  if (searchForm.engines.length === 0) {
    ElMessage.warning('请至少选择一个搜索引擎')
    return
  }

  searching.value = true
  try {
    const res = await api.post('/discovery/search', {
      engines: searchForm.engines,
      query: searchForm.query,
      max_results: searchForm.maxResults,
    })

    const taskId = res.data.data.task_id
    ElMessage.success('搜索任务已启动')

    // 创建任务
    taskStore.addTask({
      id: taskId,
      type: 'discovery-search',
      title: `搜索: ${searchForm.query}`,
      status: 'running',
      progress: 0,
      total: searchForm.maxResults,
    })

    // 轮询任务状态
    taskStore.pollTask(
      taskId,
      () => api.get(`/discovery/tasks/${taskId}`),
      (task) => {
        // 更新当前任务显示 - 使用整个 task 对象而不是仅 result
        currentTask.value = task || currentTask.value
      },
      (task) => {
        // 完成
        const foundCount = task.found_count || task.result?.found_count || task.result?.data?.found_count || 0
        ElMessage.success(`搜索完成，发现 ${foundCount} 个服务`)
    
        // 添加到搜索历史
        addToSearchHistory(searchForm.query, foundCount)
    
        loadRecentServices()
        taskStore.removeTask(taskId)
        
        // 提示用户自动检测已启动
        if (foundCount > 0) {
          ElMessage.info({
            message: '正在自动检测新发现的服务，请稍候...',
            duration: 3000,
          })
        }
      },
      (task, error) => {
        // 失败
        ElMessage.error('搜索失败：' + error.message)
        taskStore.removeTask(taskId)
      },
      3000,
      600000 // 搜索任务最长 10 分钟
    )
  } catch (error: any) {
    ElMessage.error('启动搜索失败：' + error.message)
  } finally {
    searching.value = false
  }
}

const viewResults = () => {
  router.push('/admin/services')
}

const loadRecentServices = async () => {
  try {
    const res = await api.get('/services', { params: { page: 1, limit: 10, sort: '-created_at' } })
    recentServices.value = (res.data.data || []).map((s: any) => ({ ...s, checking: false }))
  } catch (error: any) {
    console.error('加载最近服务失败:', error)
  }
}

const checkService = async (id: string) => {
  const service = recentServices.value.find(s => s.id === id)
  if (service) service.checking = true

  try {
    await api.post(`/services/${id}/check`)
    ElMessage.success('检测完成')
    loadRecentServices()
  } catch (error: any) {
    ElMessage.error('检测失败：' + error.message)
  } finally {
    if (service) service.checking = false
  }
}

const toggleEngine = (key: string, enabled: boolean) => {
  if (!enabled) {
    ElMessage.warning('请先在系统设置中配置该搜索引擎')
    return
  }

  const index = searchForm.engines.indexOf(key)
  if (index > -1) {
    searchForm.engines.splice(index, 1)
  } else {
    searchForm.engines.push(key)
  }
}

const useDefaultQuery = () => {
  searchForm.query = 'port="11434"' // 通用查询，适合所有引擎
}

const resetSearch = () => {
  searchForm.engines = ['fofa']
  searchForm.query = 'port="11434"' // 通用查询，适合所有引擎
  searchForm.maxResults = 100
}

const useHistory = (item: any) => {
  searchForm.query = item.query
}

const clearHistory = () => {
  searchHistory.value = []
  localStorage.removeItem('discovery_search_history')
}

const getTaskStatusType = (status: string) => {
  const types: any = {
    pending: 'info',
    running: 'warning',
    completed: 'success',
    failed: 'danger',
  }
  return types[status] || 'info'
}

const getTaskStatusText = (status: string) => {
  const texts: any = {
    pending: '等待中',
    running: '进行中',
    completed: '已完成',
    failed: '失败',
  }
  return texts[status] || status
}

const getSourceText = (source: string) => {
  const texts: any = {
    fofa: 'FOFA',
    hunter: 'Hunter',
    zoomeye: 'ZoomEye',
    shodan: 'Shodan',
    manual: '手动',
  }
  return texts[source] || source
}

const getSourceTagType = (source: string) => {
  const types: any = {
    fofa: '',
    hunter: 'success',
    zoomeye: 'warning',
    shodan: 'danger',
    manual: 'info',
  }
  return types[source] || 'info'
}

const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('zh-CN')
}

const formatTime = (date: Date | string) => {
  const now = new Date()
  const target = typeof date === 'string' ? new Date(date) : date
  const diff = now.getTime() - target.getTime()
  const minutes = Math.floor(diff / 60000)
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes}分钟前`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}小时前`
  const days = Math.floor(hours / 24)
  return `${days}天前`
}

onMounted(() => {
  loadRecentServices()
  loadEngineConfig()
  loadSearchHistory()
})

onUnmounted(() => {
  // 任务由 taskStore 全局管理，无需清理
})
</script>

<style scoped>
.discovery-page {
  height: 100%;
  padding-bottom: 20px;
}

/* 卡片通用样式 */
.search-card,
.task-card,
.history-card,
.result-card {
  border-radius: 12px;
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-title {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
}

.title-icon {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.title-icon.primary {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.title-icon.success {
  background: linear-gradient(135deg, #4ade80 0%, #22c55e 100%);
}

.title-icon.warning {
  background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%);
}

.title-icon.danger {
  background: linear-gradient(135deg, #f87171 0%, #dc2626 100%);
}

/* 表单部分 */
.form-section {
  margin-bottom: 24px;
  padding-bottom: 24px;
  border-bottom: 1px solid #f1f5f9;
}

.form-section:last-child {
  border-bottom: none;
  margin-bottom: 0;
  padding-bottom: 0;
}

.section-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: #334155;
  margin-bottom: 12px;
}

/* 引擎选择网格 */
.engines-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  margin-bottom: 8px;
}

.engine-option {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  background: #f8fafc;
  border: 2px solid transparent;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.3s ease;
  min-height: 80px;
}

.engine-option:hover {
  background: #f1f5f9;
}

.engine-option.active {
  border-color: #4f46e5;
  background: linear-gradient(135deg, #eff6ff 0%, #e0e7ff 100%);
}

.engine-option.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.engine-check {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 20px;
  height: 20px;
  background: #4f46e5;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 12px;
}

.engine-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.engine-emoji {
  font-size: 24px;
}

.engine-info {
  flex: 1;
}

.engine-name {
  font-weight: 600;
  color: #1e293b;
  margin-bottom: 4px;
}

.engine-status {
  font-size: 12px;
  color: #f87171;
}

.engine-status.enabled {
  color: #22c55e;
}

/* 查询输入 */
.query-input {
  margin-bottom: 8px;
}

.query-tips {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 8px 12px;
  background: #f8fafc;
  border-radius: 6px;
}

/* 操作按钮 */
.action-buttons {
  display: flex;
  gap: 12px;
  justify-content: center;
  padding-top: 8px;
}

/* 任务卡片 */
.task-card {
  min-height: 400px;
  margin-bottom: 0;
}

.task-info {
  padding: 8px 0;
}

.task-status-wrapper {
  margin-bottom: 20px;
  text-align: center;
}

.status-tag {
  padding: 12px 24px;
  font-size: 15px;
}

.status-icon {
  margin-right: 6px;
}

.task-details {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 24px;
}

.detail-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
  border-radius: 10px;
}

.detail-item .label {
  font-weight: 600;
  color: #64748b;
  font-size: 13px;
}

.detail-item .value {
  color: #1e293b;
  font-weight: 500;
}

.progress-wrapper {
  margin-bottom: 20px;
}

.task-actions {
  display: flex;
  justify-content: center;
}

.empty-task {
  padding: 40px 0;
}

.empty-icon {
  color: #cbd5e1;
}

/* 历史记录卡片 */
.history-card {
  margin-top: 20px;
  margin-bottom: 0;
}

.history-list {
  max-height: 280px;
  overflow-y: auto;
}

.history-item {
  padding: 12px;
  background: #f8fafc;
  border-radius: 8px;
  margin-bottom: 8px;
  cursor: pointer;
  transition: all 0.3s ease;
}

.history-item:hover {
  background: #f1f5f9;
  transform: translateX(4px);
}

.history-query {
  font-weight: 500;
  color: #334155;
  margin-bottom: 6px;
  font-family: 'Courier New', monospace;
  font-size: 13px;
}

.history-meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.history-time {
  font-size: 12px;
  color: #94a3b8;
}

/* 结果卡片 */
.result-card {
  margin-bottom: 0;
  margin-top: 20px;
}

.url-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.time-text {
  font-size: 13px;
  color: #64748b;
}

/* 响应式 */
@media (max-width: 768px) {
  .engines-grid {
    grid-template-columns: 1fr;
  }

  .action-buttons {
    flex-direction: column;
  }

  .action-buttons .el-button {
    width: 100%;
  }
  
  .search-card,
  .task-card,
  .history-card,
  .result-card {
    margin-bottom: 16px;
  }
}
</style>
