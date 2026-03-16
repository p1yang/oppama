<template>
  <div class="services-page">
    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :xs="12" :sm="6" :md="6" :lg="6" :xl="6">
        <div class="stat-card" @click="filterByStatus('')">
          <div class="stat-icon total">
            <el-icon :size="24"><Connection /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ total }}</div>
            <div class="stat-label">总服务数</div>
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="6" :md="6" :lg="6" :xl="6">
        <div class="stat-card" @click="filterByStatus('online')">
          <div class="stat-icon online">
            <el-icon :size="24"><CircleCheck /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ onlineCount }}</div>
            <div class="stat-label">在线服务</div>
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="6" :md="6" :lg="6" :xl="6">
        <div class="stat-card" @click="filterByStatus('honeypot')">
          <div class="stat-icon honeypot">
            <el-icon :size="24"><Warning /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ honeypotCount }}</div>
            <div class="stat-label">蜜罐服务</div>
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="6" :md="6" :lg="6" :xl="6">
        <div class="stat-card" @click="filterByStatus('offline')">
          <div class="stat-icon offline">
            <el-icon :size="24"><CircleClose /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ offlineCount }}</div>
            <div class="stat-label">离线服务</div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 搜索和工具栏 -->
    <el-card class="search-card" shadow="never">
      <div class="search-header">
        <div class="search-title">
          <el-icon :size="20" color="#4f46e5"><Search /></el-icon>
          <span>搜索服务</span>
        </div>
      </div>

      <el-form :inline="true" :model="searchForm" class="search-form">
        <el-form-item>
          <el-input
            v-model="searchForm.search"
            placeholder="搜索服务名称或 URL..."
            clearable
            prefix-icon="Search"
            class="search-input"
            @keyup.enter="loadServices"
          >
            <template #append>
              <el-button @click="loadServices" :icon="Search" />
            </template>
          </el-input>
        </el-form-item>
        <el-form-item>
          <el-select
            v-model="searchForm.status"
            placeholder="全部状态"
            clearable
            class="filter-select"
            @change="loadServices"
          >
            <el-option label="在线" value="online">
              <span class="option-dot online"></span>
              在线
            </el-option>
            <el-option label="离线" value="offline">
              <span class="option-dot offline"></span>
              离线
            </el-option>
            <el-option label="蜜罐" value="honeypot">
              <span class="option-dot honeypot"></span>
              蜜罐
            </el-option>
            <el-option label="未知" value="unknown">
              <span class="option-dot unknown"></span>
              未知
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-select
            v-model="searchForm.source"
            placeholder="全部来源"
            clearable
            class="filter-select"
            @change="loadServices"
          >
            <el-option label="手动添加" value="manual" />
            <el-option label="FOFA" value="fofa" />
            <el-option label="Hunter" value="hunter" />
            <el-option label="ZoomEye" value="zoomeye" />
            <el-option label="Shodan" value="shodan" />
          </el-select>
        </el-form-item>
      </el-form>

      <div class="toolbar">
        <div class="toolbar-left">
          <el-button type="primary" @click="showAddDialog = true" :icon="Plus">
            添加服务
          </el-button>
          <el-button type="success" @click="showBatchCheck = true" :icon="Refresh">
            批量检测
          </el-button>
          <el-button type="warning" @click="checkAllServices" :icon="Lightning" :loading="checkingAll">
            一键检测
          </el-button>
          <el-button v-if="selectedIds.length > 0" type="danger" @click="batchDelete" :icon="Delete">
            批量删除 ({{ selectedIds.length }})
          </el-button>
        </div>
        <div class="toolbar-right">
          <el-tooltip content="刷新列表" placement="top">
            <el-button :icon="RefreshRight" @click="loadServices" circle />
          </el-tooltip>
          <el-tooltip :content="viewMode === 'table' ? '卡片视图' : '表格视图'" placement="top">
            <el-button :icon="viewMode === 'table' ? Grid : List" @click="toggleView" circle />
          </el-tooltip>
        </div>
      </div>
    </el-card>

    <!-- 表格视图 -->
    <el-card v-if="viewMode === 'table'" class="table-card" shadow="never">
      <el-table
        ref="tableRef"
        :data="services"
        v-loading="loading"
        stripe
        style="width: 100%"
        :header-cell-style="{ background: '#f8fafc', color: '#475569', fontWeight: '600' }"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="55" />
        <el-table-column prop="name" label="名称" min-width="150">
          <template #default="{ row }">
            <div class="service-name-cell">
              <el-avatar :size="32" :icon="Server" :class="`status-${row.status}`" />
              <div class="name-wrapper">
                <div class="name">{{ row.name || '-' }}</div>
                <div class="url-short">{{ formatUrlShort(row.url) }}</div>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="url" label="URL" min-width="200">
          <template #default="{ row }">
            <el-link :href="row.url" target="_blank" type="primary" :underline="false" class="url-link">
              <el-icon><Link /></el-icon>
              {{ row.url }}
            </el-link>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)" effect="light" size="large">
              <el-icon class="status-icon">
                <CircleCheck v-if="row.status === 'online'" />
                <CircleClose v-else-if="row.status === 'offline'" />
                <Warning v-else-if="row.status === 'honeypot'" />
                <QuestionFilled v-else />
              </el-icon>
              {{ getStatusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="version" label="版本" width="100">
          <template #default="{ row }">
            <el-tag v-if="row.version" type="info" effect="plain" size="small">v{{ row.version }}</el-tag>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="response_time" label="响应时间" width="110">
          <template #default="{ row }">
            <el-tag :type="getResponseTimeType(row.response_time)" effect="plain" size="small">
              {{ formatResponseTime(row.response_time) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="模型" width="80">
          <template #default="{ row }">
            <el-badge :value="row.models?.length || 0" :max="99" type="primary" />
          </template>
        </el-table-column>
        <el-table-column prop="source" label="来源" width="100">
          <template #default="{ row }">
            <el-tag v-if="row.source" effect="plain" size="small">{{ getSourceText(row.source) }}</el-tag>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-tooltip content="检测服务" placement="top">
                <el-button size="small" @click="checkService(row.id)" :loading="row.checking">
                  <el-icon><Connection /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="编辑服务" placement="top">
                <el-button size="small" @click="editService(row)">
                  <el-icon><Edit /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="删除服务" placement="top">
                <el-button size="small" type="danger" @click="deleteService(row.id)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </el-tooltip>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="loadServices"
          @current-change="loadServices"
        />
      </div>
    </el-card>

    <!-- 卡片视图 -->
    <div v-else class="card-view">
      <transition-group name="card-fade" tag="div" class="service-grid">
        <div v-for="service in services" :key="service.id" class="service-card" :class="`status-${service.status}`">
          <div class="card-header">
            <div class="card-icon">
              <el-icon :size="32"><Connection /></el-icon>
            </div>
            <div class="card-status" :class="service.status">
              <el-icon>
                <CircleCheck v-if="service.status === 'online'" />
                <CircleClose v-else-if="service.status === 'offline'" />
                <Warning v-else-if="service.status === 'honeypot'" />
                <QuestionFilled v-else />
              </el-icon>
            </div>
          </div>
          <div class="card-body">
            <h3 class="card-title">{{ service.name || '未命名服务' }}</h3>
            <p class="card-url">{{ service.url }}</p>
            <div class="card-meta">
              <span class="meta-item">
                <el-icon><Timer /></el-icon>
                {{ formatResponseTime(service.response_time) }}
              </span>
              <span class="meta-item">
                <el-icon><Files /></el-icon>
                {{ service.models?.length || 0 }} 模型
              </span>
            </div>
          </div>
          <div class="card-footer">
            <el-button size="small" @click="checkService(service.id)" :loading="service.checking">
              <el-icon><Connection /></el-icon>
              检测
            </el-button>
            <el-button size="small" @click="editService(service.id)">
              <el-icon><Edit /></el-icon>
              编辑
            </el-button>
            <el-button size="small" type="danger" @click="deleteService(service.id)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </div>
        </div>
      </transition-group>

      <!-- 卡片视图分页 -->
      <div v-if="services.length > 0" class="pagination-wrapper card-pagination">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="loadServices"
          @current-change="loadServices"
        />
      </div>

      <el-empty v-if="!loading && services.length === 0" description="暂无服务数据" :image-size="200" />
    </div>

    <!-- 添加/编辑服务对话框 -->
    <el-dialog v-model="showAddDialog" :title="editingService ? '编辑服务' : '添加服务'" width="500px" :close-on-click-modal="false">
      <div class="dialog-icon-wrapper">
        <div class="dialog-icon">
          <el-icon :size="32"><Plus /></el-icon>
        </div>
      </div>
      <el-form :model="newService" label-width="90px" label-position="left">
        <el-form-item label="服务 URL" required>
          <el-input
            v-model="newService.url"
            placeholder="http://localhost:11434"
            prefix-icon="Link"
          />
        </el-form-item>
        <el-form-item label="服务名称">
          <el-input
            v-model="newService.name"
            placeholder="可选，为空则自动生成"
            prefix-icon="Edit"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="closeAddDialog">取消</el-button>
        <el-button type="primary" @click="addService" :loading="submitting">
          {{ editingService ? '保存' : '添加' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 批量检测对话框 -->
    <el-dialog v-model="showBatchCheck" title="批量检测服务" width="600px" :close-on-click-modal="false">
      <div class="dialog-icon-wrapper success">
        <div class="dialog-icon success">
          <el-icon :size="32"><Refresh /></el-icon>
        </div>
      </div>
      <p class="dialog-tip">每行输入一个 URL，系统将先添加到列表，然后在后台异步检测</p>

      <!-- 检测进度 -->
      <div v-if="taskStore.activeTasks.length > 0" class="batch-progress">
        <div class="progress-header">
          <el-icon class="spin"><Loading /></el-icon>
          <span>正在后台检测中...</span>
        </div>
        <el-progress
          :percentage="Math.round((taskStore.activeTasks[0].progress / taskStore.activeTasks[0].total) * 100)"
          :stroke-width="12"
          :striped="true"
          :striped-flow="true"
        >
          <template #default="{ percentage }">
            <span class="progress-text">{{ taskStore.activeTasks[0].progress }} / {{ taskStore.activeTasks[0].total }}</span>
          </template>
        </el-progress>
        <p class="progress-tip">您可以关闭此对话框，检测将在后台继续进行</p>
      </div>

      <el-input
        v-model="batchUrls"
        type="textarea"
        :rows="10"
        placeholder="每行一个 URL，例如：&#10;http://192.168.1.1:11434&#10;http://example.com:11434"
        :disabled="batchChecking"
      />
      <template #footer>
        <el-button @click="showBatchCheck = false" :disabled="batchChecking">取消</el-button>
        <el-button type="primary" @click="executeBatchCheck" :loading="batchChecking">
          {{ batchChecking ? '正在添加...' : '开始检测' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '@/api/client'
import { useTaskStore } from '@/stores/taskStore'

const taskStore = useTaskStore()

const loading = ref(false)
const submitting = ref(false)
const batchChecking = ref(false)
const checkingAll = ref(false)
const services = ref<any[]>([])
const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)
const selectedIds = ref<string[]>([])
const viewMode = ref<'table' | 'card'>('table')
const editingService = ref<any>(null)

// 统计数据（从后端获取）
const onlineCount = ref(0)
const honeypotCount = ref(0)
const offlineCount = ref(0)

const searchForm = reactive({
  search: '',
  status: '',
  source: '',
})

const showAddDialog = ref(false)
const newService = reactive({
  url: '',
  name: '',
})

const showBatchCheck = ref(false)
const batchUrls = ref('')

const loadServices = async () => {
  loading.value = true
  try {
    const params: any = {
      page: currentPage.value,
      limit: pageSize.value,
    }
    if (searchForm.search) params.search = searchForm.search
    if (searchForm.status) params.status = searchForm.status
    if (searchForm.source) params.source = searchForm.source

    const res = await api.get('/services', { params })
    services.value = (res.data.data || []).map((s: any) => ({ ...s, checking: false }))
    total.value = res.data.total || 0

    // 加载统计数据
    loadStats()
  } catch (error: any) {
    ElMessage.error('加载服务列表失败：' + error.message)
  } finally {
    loading.value = false
  }
}

const loadStats = async () => {
  try {
    const res = await api.get('/services/stats')
    const stats = res.data.data || {}
    onlineCount.value = stats.online || 0
    honeypotCount.value = stats.honeypot || 0
    offlineCount.value = stats.offline || 0
  } catch (error) {
    console.error('加载统计数据失败:', error)
  }
}

const filterByStatus = (status: string) => {
  searchForm.status = status
  currentPage.value = 1
  loadServices()
}

const handleSelectionChange = (selection: any[]) => {
  selectedIds.value = selection.map(s => s.id)
}

const toggleView = () => {
  viewMode.value = viewMode.value === 'table' ? 'card' : 'table'
}

const closeAddDialog = () => {
  showAddDialog.value = false
  editingService.value = null
  newService.url = ''
  newService.name = ''
}

const editService = (service: any) => {
  if (typeof service === 'string') {
    // ID passed, find the service
    const found = services.value.find(s => s.id === service)
    if (found) {
      editingService.value = found
      newService.url = found.url
      newService.name = found.name || ''
      showAddDialog.value = true
    }
  } else {
    editingService.value = service
    newService.url = service.url
    newService.name = service.name || ''
    showAddDialog.value = true
  }
}

const addService = async () => {
  if (!newService.url) {
    ElMessage.warning('请输入服务 URL')
    return
  }

  submitting.value = true
  try {
    if (editingService.value) {
      await api.put(`/services/${editingService.value.id}`, newService)
      ElMessage.success('更新成功')
    } else {
      await api.post('/services', newService)
      ElMessage.success('添加成功')
    }
    closeAddDialog()
    loadServices()
  } catch (error: any) {
    ElMessage.error(editingService.value ? '更新失败：' : '添加失败：' + error.message)
  } finally {
    submitting.value = false
  }
}

const checkService = async (id: string) => {
  const service = services.value.find(s => s.id === id)
  if (service) {
    service.checking = true
    service.status = 'checking'
  }

  try {
    // 启动异步检测
    const res = await api.post(`/services/${id}/check`, { async: true })
    const taskId = res.data.data?.task_id

    if (taskId) {
      // 创建任务
      taskStore.addTask({
        id: taskId,
        type: 'service-check',
        title: `检测服务: ${service?.name || service?.url}`,
        status: 'running',
        progress: 0,
        total: 100,
      })

      // 轮询任务状态
      taskStore.pollTask(
        taskId,
        () => api.get(`/services/tasks/${taskId}`),
        (task) => {
          // 更新服务状态为检测中
          if (service) {
            service.checking = true
            service.status = 'checking'
          }
        },
        (task) => {
          // 完成
          ElMessage.success('检测完成')
          loadServices()
          taskStore.removeTask(taskId)
        },
        (task, error) => {
          // 失败
          ElMessage.error('检测失败：' + error.message)
          if (service) service.checking = false
          taskStore.removeTask(taskId)
        }
      )
    } else {
      // 同步完成
      ElMessage.success('检测完成')
      loadServices()
    }
  } catch (error: any) {
    ElMessage.error('检测失败：' + error.message)
    if (service) {
      service.checking = false
    }
  }
}

const deleteService = async (id: string) => {
  try {
    await ElMessageBox.confirm('确定要删除此服务吗？', '确认删除', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
    await api.delete(`/services/${id}`)
    ElMessage.success('删除成功')
    loadServices()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败：' + error.message)
    }
  }
}

const batchDelete = async () => {
  try {
    await ElMessageBox.confirm(`确定要删除选中的 ${selectedIds.value.length} 个服务吗？`, '确认删除', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
    await Promise.all(selectedIds.value.map(id => api.delete(`/services/${id}`)))
    ElMessage.success('批量删除成功')
    selectedIds.value = []
    loadServices()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('批量删除失败：' + error.message)
    }
  }
}

const executeBatchCheck = async () => {
  const urls = batchUrls.value.split('\n').filter(u => u.trim())
  if (urls.length === 0) {
    ElMessage.warning('请输入至少一个 URL')
    return
  }

  batchChecking.value = true
  showBatchCheck.value = false

  try {
    // 步骤 1: 先添加所有服务到列表
    const addPromises = urls.map(url =>
      api.post('/services', { url })
        .then(res => ({ success: true, data: res.data }))
        .catch(err => ({ success: false, url, error: err.message }))
    )

    const results = await Promise.all(addPromises)
    const successCount = results.filter(r => r.success).length
    const failCount = results.filter(r => !r.success).length

    if (successCount > 0) {
      ElMessage.success(`已添加 ${successCount} 个服务，开始后台检测...`)
      await loadServices()
    }

    if (failCount > 0) {
      ElMessage.warning(`${failCount} 个服务添加失败（可能已存在）`)
    }

    // 步骤 2: 启动批量检测任务
    try {
      const checkRes = await api.post('/services/batch-check', {
        urls: urls.filter(u => u.trim()),
        concurrency: 10,
        timeout: 30,
      })

      const taskId = checkRes.data.data?.task_id
      if (taskId) {
        // 创建批量检测任务
        taskStore.addTask({
          id: taskId,
          type: 'batch-check',
          title: `批量检测 ${urls.length} 个服务`,
          status: 'running',
          progress: 0,
          total: urls.length,
        })

        // 轮询任务状态
        taskStore.pollTask(
          taskId,
          () => api.get(`/services/tasks/${taskId}`),
          (task) => {
            // 实时刷新服务列表
            loadServices()
          },
          (task) => {
            // 完成
            ElMessage.success(`批量检测完成！发现 ${task.result?.found_count || 0} 个可用服务`)
            taskStore.removeTask(taskId)
            loadServices()
          },
          (task, error) => {
            // 失败
            ElMessage.error('批量检测失败：' + error.message)
            taskStore.removeTask(taskId)
          }
        )
      }
    } catch (checkError: any) {
      ElMessage.warning('服务已添加，但检测任务启动失败：' + checkError.message)
    }

    batchUrls.value = ''
  } catch (error: any) {
    ElMessage.error('批量操作失败：' + error.message)
  } finally {
    batchChecking.value = false
  }
}

const checkAllServices = async () => {
  try {
    await ElMessageBox.confirm('确定要检测所有服务吗？此操作将在后台异步执行。', '确认检测', {
      type: 'warning',
      confirmButtonText: '开始检测',
      cancelButtonText: '取消',
    })

    checkingAll.value = true

    // 调用一键检测 API
    const res = await api.post('/services/check-all')
    const taskId = res.data.data?.task_id

    if (taskId) {
      // 创建任务
      taskStore.addTask({
        id: taskId,
        type: 'batch-check',
        title: `一键检测所有服务`,
        status: 'running',
        progress: 0,
        total: total.value,
      })

      // 轮询任务状态
      taskStore.pollTask(
        taskId,
        () => api.get(`/services/tasks/${taskId}`),
        (task) => {
          // 实时刷新服务列表
          loadServices()
        },
        (task) => {
          // 完成
          const result = task.result || {}
          ElMessage.success(
            `检测完成！成功：${result.success || 0}, 发现 ${result.found_count || 0} 个模型，离线：${result.offline || 0}, 蜜罐：${result.honeypot || 0}`
          )
          taskStore.removeTask(taskId)
          loadServices()
        },
        (task, error) => {
          // 失败
          ElMessage.error('检测失败：' + error.message)
          taskStore.removeTask(taskId)
        }
      )

      ElMessage.success('已启动检测任务，请在右上角查看进度')
    } else {
      ElMessage.info(res.data.data?.message || '没有需要检测的服务')
    }
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('启动检测失败：' + error.message)
    }
  } finally {
    checkingAll.value = false
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

const getSourceText = (source: string) => {
  const texts: any = {
    manual: '手动',
    fofa: 'FOFA',
    hunter: 'Hunter',
    zoomeye: 'ZoomEye',
    shodan: 'Shodan',
  }
  return texts[source] || source
}

const formatResponseTime = (ms: number) => {
  if (!ms) return '-'
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

const getResponseTimeType = (ms: number) => {
  if (!ms) return 'info'
  if (ms < 500) return 'success'
  if (ms < 1000) return 'warning'
  return 'danger'
}

const formatUrlShort = (url: string) => {
  if (!url) return '-'
  try {
    const u = new URL(url)
    return u.hostname + (u.port ? ':' + u.port : '')
  } catch {
    return url.length > 20 ? url.substring(0, 20) + '...' : url
  }
}

onMounted(() => {
  loadServices()
})

onUnmounted(() => {
  // 清理该组件相关的任务
  // taskStore 会在全局管理
})
</script>

<style scoped>
.services-page {
  height: 100%;
  padding-bottom: 20px;
}

/* 统计卡片 */
.stats-row {
  margin-bottom: 20px;
}

.stat-card {
  background: #fff;
  border-radius: 12px;
  padding: 20px;
  display: flex;
  align-items: center;
  gap: 16px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  transition: all 0.3s ease;
  cursor: pointer;
  height: 100%;
}

.stat-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.1);
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  flex-shrink: 0;
}

.stat-icon.total {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.stat-icon.online {
  background: linear-gradient(135deg, #4ade80 0%, #22c55e 100%);
}

.stat-icon.honeypot {
  background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%);
}

.stat-icon.offline {
  background: linear-gradient(135deg, #f87171 0%, #dc2626 100%);
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
}

/* 搜索卡片 */
.search-card {
  margin-bottom: 20px;
  border-radius: 12px;
}

.search-header {
  margin-bottom: 16px;
}

.search-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
}

.search-form {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.search-input {
  width: 300px;
}

.filter-select {
  width: 140px;
}

.filter-select :deep(.el-input__wrapper) {
  padding-left: 12px;
}

.option-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 8px;
}

.option-dot.online {
  background: #22c55e;
}

.option-dot.offline {
  background: #dc2626;
}

.option-dot.honeypot {
  background: #f59e0b;
}

.option-dot.unknown {
  background: #94a3b8;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 16px;
}

.toolbar-left,
.toolbar-right {
  display: flex;
  gap: 8px;
}

/* 表格卡片 */
.table-card {
  border-radius: 12px;
}

.service-name-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.service-name-cell .el-avatar {
  border-radius: 10px;
}

.service-name-cell .el-avatar.status-online {
  background: linear-gradient(135deg, #4ade80 0%, #22c55e 100%);
  color: #fff;
}

.service-name-cell .el-avatar.status-offline {
  background: linear-gradient(135deg, #f87171 0%, #dc2626 100%);
  color: #fff;
}

.service-name-cell .el-avatar.status-honeypot {
  background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%);
  color: #fff;
}

.service-name-cell .el-avatar.status-unknown {
  background: linear-gradient(135deg, #94a3b8 0%, #64748b 100%);
  color: #fff;
}

.name-wrapper {
  flex: 1;
}

.name {
  font-weight: 600;
  color: #1e293b;
}

.url-short {
  font-size: 12px;
  color: #94a3b8;
}

.url-link {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
}

.status-icon {
  margin-right: 4px;
}

.text-muted {
  color: #94a3b8;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  padding: 20px 0;
}

.card-pagination {
  margin-top: 24px;
  padding: 24px 0;
}

/* 卡片视图 */
.card-view {
  min-height: 400px;
}

.service-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
}

.service-card {
  background: #fff;
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
  border: 2px solid transparent;
}

.service-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.12);
}

.service-card.status-online {
  border-color: rgba(34, 197, 94, 0.3);
}

.service-card.status-offline {
  border-color: rgba(220, 38, 38, 0.3);
}

.service-card.status-honeypot {
  border-color: rgba(245, 158, 11, 0.3);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px;
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
}

.card-icon {
  width: 48px;
  height: 48px;
  background: #fff;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #4f46e5;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

.card-status {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.card-status.online {
  background: linear-gradient(135deg, #4ade80 0%, #22c55e 100%);
}

.card-status.offline {
  background: linear-gradient(135deg, #f87171 0%, #dc2626 100%);
}

.card-status.honeypot {
  background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%);
}

.card-status.unknown {
  background: linear-gradient(135deg, #94a3b8 0%, #64748b 100%);
}

.card-body {
  padding: 20px;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
  margin: 0 0 8px 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.card-url {
  font-size: 13px;
  color: #64748b;
  margin: 0 0 16px 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.card-meta {
  display: flex;
  gap: 16px;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: #94a3b8;
}

.card-footer {
  display: flex;
  gap: 8px;
  padding: 16px 20px;
  border-top: 1px solid #f1f5f9;
}

.card-footer .el-button {
  flex: 1;
}

/* 对话框样式 */
.dialog-icon-wrapper {
  display: flex;
  justify-content: center;
  margin-bottom: 24px;
}

.dialog-icon {
  width: 64px;
  height: 64px;
  border-radius: 16px;
  background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  box-shadow: 0 8px 24px rgba(79, 70, 229, 0.3);
}

.dialog-icon.success {
  background: linear-gradient(135deg, #10b981 0%, #059669 100%);
  box-shadow: 0 8px 24px rgba(16, 185, 129, 0.3);
}

.dialog-tip {
  text-align: center;
  color: #64748b;
  margin-bottom: 16px;
  font-size: 14px;
}

/* 批量检测进度 */
.batch-progress {
  padding: 20px;
  background: linear-gradient(135deg, #f0fdf4 0%, #dcfce7 100%);
  border-radius: 12px;
  margin-bottom: 20px;
}

.progress-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 500;
  color: #16a34a;
  margin-bottom: 12px;
}

.progress-text {
  font-size: 13px;
  font-weight: 600;
}

.progress-tip {
  text-align: center;
  font-size: 12px;
  color: #64748b;
  margin-top: 12px;
  margin-bottom: 0;
}

/* 卡片过渡动画 */
.card-fade-enter-active {
  transition: all 0.3s ease;
}

.card-fade-enter-from {
  opacity: 0;
  transform: translateY(20px);
}

/* 响应式 */
@media (max-width: 768px) {
  .search-input {
    width: 100%;
  }

  .filter-select {
    width: 100%;
  }

  .toolbar {
    flex-direction: column;
  }

  .toolbar-left,
  .toolbar-right {
    width: 100%;
    justify-content: center;
  }

  .service-grid {
    grid-template-columns: 1fr;
  }

  :deep(.el-table__body-wrapper) {
    overflow-x: auto;
  }
}
</style>
