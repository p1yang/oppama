<template>
  <div class="models-page">
    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :xs="24" :sm="8" :md="8" :lg="8" :xl="8">
        <div class="stat-card primary">
          <div class="stat-bg">
            <svg viewBox="0 0 200 200" xmlns="http://www.w3.org/2000/svg">
              <path fill="#667eea" d="M44.7,-76.4C58.9,-69.2,71.8,-59.1,79.6,-45.8C87.4,-32.6,90.1,-16.3,88.6,-0.9C87.1,14.5,81.4,29,73.3,42.1C65.2,55.2,54.7,66.9,42.1,73.8C29.5,80.7,14.8,82.8,-1.2,85.3C-17.3,87.8,-34.8,90.7,-48.6,83.4C-62.4,76.1,-72.5,58.6,-79.6,42.6C-86.7,26.6,-90.8,12.1,-88.9,-1.2C-87,-14.5,-79.1,-26.6,-69.6,-36.3C-60.1,-46,-48.9,-53.3,-37.4,-61.2C-25.9,-69.1,-14.1,-77.6,1.2,-79.6C16.5,-81.6,33,-77.1,44.7,-76.4Z" transform="translate(100 100)" />
            </svg>
          </div>
          <div class="stat-icon">
            <el-icon :size="28"><Files /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ models.length }}</div>
            <div class="stat-label">总模型数</div>
          </div>
        </div>
      </el-col>
      <el-col :xs="24" :sm="8" :md="8" :lg="8" :xl="8">
        <div class="stat-card success">
          <div class="stat-bg">
            <svg viewBox="0 0 200 200" xmlns="http://www.w3.org/2000/svg">
              <path fill="#4ade80" d="M39.9,-65.7C52.5,-56.9,64.3,-47.5,72.2,-35.6C80.1,-23.7,84.1,-9.3,82.3,4.3C80.5,17.9,72.9,30.7,63.6,41.4C54.3,52.1,43.3,60.7,31.2,66.5C19.1,72.3,5.9,75.3,-6.2,84.8C-18.3,94.3,-29.3,110.3,-40.5,112.2C-51.7,114.1,-63.1,101.9,-70.3,88.2C-77.5,74.5,-80.5,59.3,-83.4,44.9C-86.3,30.5,-89.1,16.9,-87.3,3.8C-85.5,-9.3,-79.1,-21.9,-70.5,-31.9C-61.9,-41.9,-51.1,-49.3,-39.8,-57.1C-28.5,-64.9,-16.7,-73.1,-2.4,-69.5C11.9,-65.9,23.8,-50.5,39.9,-65.7Z" transform="translate(100 100)" />
            </svg>
          </div>
          <div class="stat-icon">
            <el-icon :size="28"><CircleCheck /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ availableCount }}</div>
            <div class="stat-label">可用模型</div>
          </div>
        </div>
      </el-col>
      <el-col :xs="24" :sm="8" :md="8" :lg="8" :xl="8">
        <div class="stat-card warning">
          <div class="stat-bg">
            <svg viewBox="0 0 200 200" xmlns="http://www.w3.org/2000/svg">
              <path fill="#fbbf24" d="M35.1,-60.3C46.6,-54.2,57.6,-47.1,65.8,-37.3C74,-27.5,79.4,-15,81.1,-1.9C82.8,11.2,80.8,24.8,74.3,36.3C67.8,47.8,56.8,57.2,44.6,63.4C32.4,69.6,19,72.6,4.7,78.3C-9.6,84,-25.1,92.4,-38.2,89.3C-51.3,86.2,-62,71.6,-69.4,57.6C-76.8,43.6,-80.9,30.2,-82.4,16.3C-83.9,2.4,-82.8,-12,-77.6,-24.9C-72.4,-37.8,-63.1,-49.2,-51.7,-56.1C-40.3,-63,-26.8,-65.4,-13.8,-67.2C-0.8,-69,12.2,-70.2,23.6,-66.4L35.1,-60.3Z" transform="translate(100 100)" />
            </svg>
          </div>
          <div class="stat-icon">
            <el-icon :size="28"><Warning /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ unavailableCount }}</div>
            <div class="stat-label">不可用模型</div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 搜索和筛选 -->
    <el-card class="search-card" shadow="never">
      <div class="search-header">
        <div class="search-title">
          <el-icon :size="20" color="#4f46e5"><Search /></el-icon>
          <span>搜索和筛选模型</span>
        </div>
        <div class="view-toggle">
          <el-radio-group v-model="viewMode" size="small">
            <el-radio-button label="table">
              <el-icon><List /></el-icon>
              表格
            </el-radio-button>
            <el-radio-button label="card">
              <el-icon><Grid /></el-icon>
              卡片
            </el-radio-button>
          </el-radio-group>
        </div>
      </div>

      <div class="filter-bar">
        <el-input
          v-model="searchQuery"
          placeholder="搜索模型名称、家族..."
          clearable
          prefix-icon="Search"
          class="search-input"
          @input="filterModels"
        >
          <template #append>
            <el-button :icon="Search" />
          </template>
        </el-input>

        <el-select
          v-model="filterFamily"
          placeholder="模型家族"
          clearable
          class="filter-select"
          @change="filterModels"
        >
          <el-option v-for="family in families" :key="family" :label="family" :value="family" />
        </el-select>

        <el-select
          v-model="filterStatus"
          placeholder="状态"
          clearable
          class="filter-select"
          @change="filterModels"
        >
          <el-option label="全部" value="" />
          <el-option label="可用" value="available" />
          <el-option label="不可用" value="unavailable" />
        </el-select>

        <el-select
          v-model="sortBy"
          placeholder="排序方式"
          class="filter-select"
          @change="filterModels"
        >
          <el-option label="名称排序" value="name" />
          <el-option label="大小排序" value="size" />
          <el-option label="参数量排序" value="params" />
        </el-select>

        <el-button type="primary" :icon="Refresh" @click="loadModels" :loading="loading">
          刷新
        </el-button>
      </div>
    </el-card>

    <!-- 表格视图 -->
    <el-card v-if="viewMode === 'table'" class="table-card" shadow="never">
      <el-table
        :data="filteredModels"
        v-loading="loading"
        stripe
        :header-cell-style="{ background: '#f8fafc', color: '#475569', fontWeight: '600' }"
        @row-click="showModelDetail"
        style="cursor: pointer"
      >
        <el-table-column prop="name" label="模型名称" min-width="240">
          <template #default="{ row }">
            <div class="model-name-cell">
              <div class="model-icon">
                <el-icon :size="20"><Files /></el-icon>
              </div>
              <div class="model-info">
                <div class="name">{{ row.name }}</div>
                <div class="family">{{ row.family || '未知家族' }}</div>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="family" label="家族" width="120">
          <template #default="{ row }">
            <el-tag v-if="row.family" effect="plain" size="large">{{ row.family }}</el-tag>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="size" label="大小" width="120">
          <template #default="{ row }">
            <div class="size-tag">
              <el-icon><Coin /></el-icon>
              {{ formatSize(row.size) }}
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="parameter_size" label="参数量" width="120">
          <template #default="{ row }">
            <el-tag v-if="row.parameter_size" type="info" effect="plain">{{ row.parameter_size }}</el-tag>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="quantization_level" label="量化级别" width="110">
          <template #default="{ row }">
            <el-tag v-if="row.quantization_level" type="warning" effect="plain" size="small">
              {{ row.quantization_level }}
            </el-tag>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="is_available" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.is_available ? 'success' : 'danger'" effect="light" size="large">
              <el-icon class="status-icon">
                <CircleCheck v-if="row.is_available" />
                <CircleClose v-else />
              </el-icon>
              {{ row.is_available ? '可用' : '不可用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click.stop="showModelDetail(row)">
              <el-icon><View /></el-icon>
              详情
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && filteredModels.length === 0" description="暂无模型数据" :image-size="200" />
    </el-card>

    <!-- 卡片视图 -->
    <div v-else class="card-view">
      <transition-group name="card-fade" tag="div" class="model-grid">
        <div
          v-for="model in filteredModels"
          :key="model.name"
          class="model-card"
          :class="{ available: model.is_available }"
          @click="showModelDetail(model)"
        >
          <div class="card-header">
            <div class="model-emoji">{{ getModelEmoji(model.family) }}</div>
            <div class="card-status" :class="{ available: model.is_available }">
              <el-icon>
                <CircleCheck v-if="model.is_available" />
                <CircleClose v-else />
              </el-icon>
            </div>
          </div>
          <div class="card-body">
            <h3 class="card-title">{{ model.name }}</h3>
            <p class="card-family">{{ model.family || '未知家族' }}</p>
            <div class="card-meta">
              <span class="meta-item">
                <el-icon><Coin /></el-icon>
                {{ formatSize(model.size) }}
              </span>
              <span class="meta-item" v-if="model.parameter_size">
                <el-icon><DataLine /></el-icon>
                {{ model.parameter_size }}
              </span>
              <span class="meta-item" v-if="model.quantization_level">
                <el-icon><Stamp /></el-icon>
                {{ model.quantization_level }}
              </span>
            </div>
          </div>
        </div>
      </transition-group>

      <el-empty v-if="!loading && filteredModels.length === 0" description="暂无模型数据" :image-size="200" />
    </div>

    <!-- 模型详情对话框 -->
    <el-dialog v-model="showDetailDialog" :title="selectedModel?.name" width="600px" class="model-dialog">
      <div v-if="selectedModel" class="model-detail">
        <div class="detail-header">
          <div class="detail-emoji">{{ getModelEmoji(selectedModel.family) }}</div>
          <div class="detail-info">
            <h2 class="detail-name">{{ selectedModel.name }}</h2>
            <el-tag :type="selectedModel.is_available ? 'success' : 'danger'" effect="light" size="large">
              {{ selectedModel.is_available ? '可用' : '不可用' }}
            </el-tag>
          </div>
        </div>

        <el-divider />

        <el-descriptions :column="2" border>
          <el-descriptions-item label="模型名称" :span="2">
            <el-text type="primary" tag="code">{{ selectedModel.name }}</el-text>
          </el-descriptions-item>
          <el-descriptions-item label="家族">
            <el-tag effect="plain">{{ selectedModel.family || '-' }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="参数量">
            <el-tag v-if="selectedModel.parameter_size" type="info" effect="plain">
              {{ selectedModel.parameter_size }}
            </el-tag>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="大小">
            <span class="detail-value">{{ formatSize(selectedModel.size ) }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="量化级别">
            <el-tag v-if="selectedModel.quantization_level" type="warning" effect="plain" size="small">
              {{ selectedModel.quantization_level }}
            </el-tag>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="状态" :span="2">
            <el-tag :type="selectedModel.is_available ? 'success' : 'danger'" effect="light">
              <el-icon class="status-icon">
                <CircleCheck v-if="selectedModel.is_available" />
                <CircleClose v-else />
              </el-icon>
              {{ selectedModel.is_available ? '可用' : '不可用' }}
            </el-tag>
          </el-descriptions-item>
        </el-descriptions>

        <div class="detail-actions">
          <el-button type="primary" :icon="ChatDotRound" @click="openChat">
            对话
          </el-button>
          <el-button :icon="CopyDocument" @click="copyModelName">
            复制名称
          </el-button>
          <el-button :icon="Close" @click="showDetailDialog = false">
            关闭
          </el-button>
        </div>
      </div>
    </el-dialog>

    <!-- 对话对话框 -->
    <el-dialog v-model="showChatDialog" :title="`与 ${chatModel?.name} 对话`" width="800px" class="chat-dialog">
      <div class="chat-container">
        <div class="chat-messages" ref="messagesContainer">
          <div v-for="(message, index) in messages" :key="index" class="message" :class="message.role">
            <div class="message-avatar">
              <el-icon :size="20">
                <User v-if="message.role === 'user'" />
                <Cpu v-else />
              </el-icon>
            </div>
            <div class="message-content">
              <div class="message-text">{{ message.content }}</div>
              <div class="message-time">{{ message.time }}</div>
            </div>
          </div>
          <div v-if="loading" class="message assistant">
            <div class="message-avatar">
              <el-icon :size="20"><Cpu /></el-icon>
            </div>
            <div class="message-content">
              <div class="message-text thinking">思考中...</div>
            </div>
          </div>
        </div>
        <div class="chat-input-area">
          <el-input
            v-model="inputMessage"
            placeholder="输入消息... (Shift+Enter 换行，Enter 发送)"
            type="textarea"
            :rows="3"
            :disabled="loading || !selectedModel?.is_available"
            @keydown.enter.exact.prevent="sendMessage"
          >
            <template #append>
              <el-button type="primary" :icon="Promoted" @click="sendMessage" :loading="loading" :disabled="!inputMessage.trim()">
                发送
              </el-button>
            </template>
          </el-input>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api/client'
import axios from 'axios'

const loading = ref(false)
const models = ref<any[]>([])
const showDetailDialog = ref(false)
const selectedModel = ref<any>(null)

// 对话相关
const showChatDialog = ref(false)
const chatModel = ref<any>(null)
const messages = ref<any[]>([])
const inputMessage = ref('')
const messagesContainer = ref<HTMLElement | null>(null)

const viewMode = ref<'table' | 'card'>('table')
const searchQuery = ref('')
const filterFamily = ref('')
const filterStatus = ref('')
const sortBy = ref('name')

const availableCount = computed(() => {
  return models.value.filter(m => m.is_available).length
})

const unavailableCount = computed(() => {
  return models.value.filter(m => !m.is_available).length
})

const families = computed(() => {
  const familySet = new Set<string>()
  models.value.forEach(m => {
    if (m.family) familySet.add(m.family)
  })
  return Array.from(familySet).sort()
})

const filteredModels = computed(() => {
  let result = [...models.value]

  // 搜索过滤
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(m =>
      m.name?.toLowerCase().includes(query) ||
      m.family?.toLowerCase().includes(query)
    )
  }

  // 家族过滤
  if (filterFamily.value) {
    result = result.filter(m => m.family === filterFamily.value)
  }

  // 状态过滤
  if (filterStatus.value === 'available') {
    result = result.filter(m => m.is_available)
  } else if (filterStatus.value === 'unavailable') {
    result = result.filter(m => !m.is_available)
  }

  // 排序
  result.sort((a, b) => {
    switch (sortBy.value) {
      case 'name':
        return (a.name || '').localeCompare(b.name || '')
      case 'size':
        return (b.size || 0) - (a.size || 0)
      case 'params':
        const getParamValue = (p: string) => {
          if (!p) return 0
          const match = p.match(/(\d+\.?\d*)([BGM])?/)
          if (!match) return 0
          const value = parseFloat(match[1])
          const unit = match[2]
          if (unit === 'B') return value
          if (unit === 'M') return value * 1000
          if (unit === 'G') return value * 1000000
          return value
        }
        return getParamValue(b.parameter_size) - getParamValue(a.parameter_size)
      default:
        return 0
    }
  })

  return result
})

const loadModels = async () => {
  loading.value = true
  try {
    const res = await api.get('/models')
    models.value = res.data.data || []
  } catch (error: any) {
    ElMessage.error('加载模型列表失败：' + error.message)
  } finally {
    loading.value = false
  }
}

const filterModels = () => {
  // 触发计算属性重新计算
}

const formatSize = (bytes: number) => {
  if (!bytes) return '-'
  const gb = bytes / (1024 * 1024 * 1024)
  if (gb >= 1) return `${gb.toFixed(2)} GB`
  const mb = bytes / (1024 * 1024)
  return `${mb.toFixed(2)} MB`
}

const getModelEmoji = (family: string) => {
  const emojiMap: any = {
    'llama': '🦙',
    'llava': '🦙👁️',
    'mistral': '🌀',
    'mixtral': '🌀',
    'gemma': '💎',
    'qwen': '🌟',
    'phi': '🧠',
    'stablelm': '🎨',
    'neural': '🧩',
    'nomic': '🗺️',
    'orca': '🐋',
    'falcon': '🦅',
    'yi': '🎯',
  }
  const lowerFamily = (family || '').toLowerCase()
  for (const [key, emoji] of Object.entries(emojiMap)) {
    if (lowerFamily.includes(key)) return emoji
  }
  return '🤖'
}

const showModelDetail = (row: any) => {
  selectedModel.value = row
  showDetailDialog.value = true
}

const openChat = () => {
  if (!selectedModel.value?.is_available) {
    ElMessage.warning('该模型不可用，无法对话')
    return
  }
  chatModel.value = selectedModel.value
  messages.value = []
  inputMessage.value = ''
  showChatDialog.value = true
  showDetailDialog.value = false
  nextTick(() => {
    scrollToBottom()
  })
}

const sendMessage = async () => {
  const message = inputMessage.value.trim()
  if (!message || !chatModel.value) return

  // 添加用户消息
  messages.value.push({
    role: 'user',
    content: message,
    time: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  })

  inputMessage.value = ''
  loading.value = true

  try {
    // 调用对话 API - 使用 OpenAI 兼容接口
    // 注意：OpenAI 接口在 /v1 路由组，而 baseURL 是 /v1/api，所以需要使用 ../
    // 从 Settings 配置中获取 API Key（如果启用了认证）
    const apiKey = localStorage.getItem('api_key')
    const headers: any = {
      'Content-Type': 'application/json'
    }
    
    // 如果有 API Key，使用 API Key 认证；否则使用 JWT Token（向后兼容）
    if (apiKey) {
      headers.Authorization = `Bearer ${apiKey}`
    } else {
      const token = localStorage.getItem('access_token')
      if (token) {
        headers.Authorization = `Bearer ${token}`
      }
    }
    
    const response = await axios.post('/v1/chat/completions', {
      model: chatModel.value.name,
      messages: [
        { role: 'user', content: message }
      ],
      stream: false
    }, {
      headers
    })
    
    // 获取助手回复
    const assistantMessage = response.data.choices?.[0]?.message?.content || '暂无回复'
    
    messages.value.push({
      role: 'assistant',
      content: assistantMessage,
      time: new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
    })
  } catch (error: any) {
    ElMessage.error('对话失败：' + error.message)
  } finally {
    loading.value = false
    nextTick(() => {
      scrollToBottom()
    })
  }
}

const scrollToBottom = () => {
  if (messagesContainer.value) {
    nextTick(() => {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    })
  }
}

const copyModelName = () => {
  if (selectedModel.value?.name) {
    navigator.clipboard.writeText(selectedModel.value.name)
    ElMessage.success('已复制到剪贴板')
  }
}

onMounted(() => {
  loadModels()
})
</script>

<style scoped>
.models-page {
  height: 100%;
  padding-bottom: 20px;
}

/* 统计卡片 */
.stats-row {
  margin-bottom: 20px;
}

.stat-card {
  background: #fff;
  border-radius: 16px;
  padding: 24px;
  position: relative;
  overflow: hidden;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
  cursor: pointer;
  height: 100%;
  min-height: 120px;
}

.stat-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
}

.stat-bg {
  position: absolute;
  right: -20px;
  bottom: -20px;
  width: 120px;
  height: 120px;
  opacity: 0.1;
  transition: transform 0.3s ease;
}

.stat-card:hover .stat-bg {
  transform: scale(1.1) rotate(10deg);
}

.stat-bg svg {
  width: 100%;
  height: 100%;
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

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  margin-bottom: 16px;
  position: relative;
  z-index: 1;
}

.stat-content {
  position: relative;
  z-index: 1;
}

.stat-value {
  font-size: 32px;
  font-weight: 700;
  color: #1e293b;
  line-height: 1;
  margin-bottom: 6px;
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
  display: flex;
  justify-content: space-between;
  align-items: center;
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

.filter-bar {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.search-input {
  flex: 1;
  min-width: 200px;
}

.filter-select {
  width: 140px;
}

/* 表格卡片 */
.table-card {
  border-radius: 12px;
}

.model-name-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.model-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.model-info {
  flex: 1;
}

.model-info .name {
  font-weight: 600;
  color: #1e293b;
  margin-bottom: 2px;
}

.model-info .family {
  font-size: 12px;
  color: #94a3b8;
}

.size-tag {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: #475569;
}

.status-icon {
  margin-right: 4px;
}

.text-muted {
  color: #94a3b8;
}

/* 卡片视图 */
.card-view {
  min-height: 400px;
}

.model-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
}

.model-card {
  background: #fff;
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
  cursor: pointer;
  border: 2px solid transparent;
}

.model-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.12);
}

.model-card.available {
  border-color: rgba(34, 197, 94, 0.3);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 24px;
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
}

.model-emoji {
  font-size: 48px;
  line-height: 1;
}

.card-status {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  background: linear-gradient(135deg, #f87171 0%, #dc2626 100%);
}

.card-status.available {
  background: linear-gradient(135deg, #4ade80 0%, #22c55e 100%);
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

.card-family {
  font-size: 13px;
  color: #64748b;
  margin: 0 0 16px 0;
}

.card-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: #94a3b8;
  padding: 4px 8px;
  background: #f8fafc;
  border-radius: 6px;
}

/* 模型详情对话框 */
.detail-header {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 16px;
}

.detail-emoji {
  font-size: 64px;
  line-height: 1;
}

.detail-info {
  flex: 1;
}

.detail-name {
  font-size: 20px;
  font-weight: 700;
  color: #1e293b;
  margin: 0 0 8px 0;
}

.detail-value {
  font-weight: 600;
  color: #475569;
}

.detail-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
  margin-top: 24px;
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
  .filter-bar {
    flex-direction: column;
  }

  .search-input,
  .filter-select {
    width: 100%;
  }

  .model-grid {
    grid-template-columns: 1fr;
  }

  :deep(.el-table__body-wrapper) {
    overflow-x: auto;
  }

  .detail-header {
    flex-direction: column;
    text-align: center;
  }
}

/* 对话对话框样式 */
.chat-dialog {
  .chat-container {
    display: flex;
    flex-direction: column;
    height: 500px;
  }

  .chat-messages {
    flex: 1;
    overflow-y: auto;
    padding: 20px;
    background: #f8fafc;
    border-radius: 8px;
    margin-bottom: 16px;
  }

  .message {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    margin-bottom: 16px;
    animation: message-fade-in 0.3s ease;

    &.user {
      flex-direction: row-reverse;

      .message-content {
        align-items: flex-end;
      }

      .message-text {
        background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
        color: #fff;
      }
    }

    &.assistant {
      .message-text {
        background: #fff;
        border: 1px solid #e2e8f0;
      }
    }
  }

  .message-avatar {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    display: flex;
    align-items: center;
    justify-content: center;
    color: #fff;
    flex-shrink: 0;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  }

  .message-content {
    display: flex;
    flex-direction: column;
    max-width: 70%;
  }

  .message-text {
    padding: 12px 16px;
    border-radius: 12px;
    font-size: 14px;
    line-height: 1.6;
    word-break: break-word;

    &.thinking {
      color: #94a3b8;
      font-style: italic;
    }
  }

  .message-time {
    font-size: 12px;
    color: #94a3b8;
    margin-top: 4px;
    padding: 0 4px;
  }

  .chat-input-area {
    margin-top: 16px;
  }
}

@keyframes message-fade-in {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
