<template>
  <div class="task-notification">
    <!-- 任务按钮 -->
    <transition name="slide-up">
      <div
        v-if="taskStore.activeTasks.length > 0"
        class="task-indicator"
        @click="showPanel = !showPanel"
      >
      <div class="task-icon-wrapper">
        <el-icon class="spin" :size="20"><Loading /></el-icon>
        <el-badge :value="taskStore.activeTasks.length" :max="99" />
      </div>
      <span class="task-count">{{ taskStore.activeTasks.length }} 个任务进行中</span>
    </div>
    </transition>

    <!-- 任务面板 -->
    <transition name="slide-up">
      <div v-if="showPanel && taskStore.activeTasks.length > 0" class="task-panel">
        <div class="panel-header">
          <span class="panel-title">后台任务</span>
          <el-button link @click="showPanel = false">
            <el-icon><Close /></el-icon>
          </el-button>
        </div>

        <div class="task-list">
          <div
            v-for="task in taskStore.activeTasks"
            :key="task.id"
            class="task-item"
          >
            <div class="task-info">
              <div class="task-title">{{ task.title }}</div>
              <div class="task-meta">
                <el-tag :type="getTaskTypeColor(task.type)" size="small">
                  {{ getTaskTypeName(task.type) }}
                </el-tag>
                <span class="task-status">{{ getTaskStatusText(task.status) }}</span>
              </div>
            </div>
            <div class="task-progress">
              <el-progress
                :percentage="Math.round((task.progress / task.total) * 100)"
                :stroke-width="6"
                :striped="task.status === 'running'"
                :striped-flow="task.status === 'running'"
                :show-text="false"
              />
              <span class="progress-text">{{ task.progress }}/{{ task.total }}</span>
            </div>
          </div>
        </div>

        <div class="panel-footer">
          <el-button size="small" link @click="taskStore.clearCompletedTasks()">
            清除已完成任务
          </el-button>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useTaskStore, TaskType } from '@/stores/taskStore'

const taskStore = useTaskStore()
const showPanel = ref(false)

const getTaskTypeName = (type: TaskType): string => {
  const names: Record<TaskType, string> = {
    'service-check': '服务检测',
    'batch-check': '批量检测',
    'discovery-search': '服务发现',
    'model-sync': '模型同步',
  }
  return names[type] || type
}

const getTaskTypeColor = (type: TaskType): string => {
  const colors: Record<TaskType, string> = {
    'service-check': 'primary',
    'batch-check': 'success',
    'discovery-search': 'warning',
    'model-sync': 'info',
  }
  return colors[type] || 'info'
}

const getTaskStatusText = (status: string): string => {
  const texts: Record<string, string> = {
    pending: '等待中',
    running: '进行中',
    completed: '已完成',
    failed: '失败',
  }
  return texts[status] || status
}
</script>

<style scoped>
.task-notification {
  position: fixed;
  bottom: 24px;
  right: 24px;
  z-index: 1000;
  max-width: 380px;
}

/* 任务指示器 */
.task-indicator {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 20px;
  background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
  border-radius: 50px;
  color: #fff;
  cursor: pointer;
  box-shadow: 0 8px 24px rgba(79, 70, 229, 0.4);
  transition: all 0.3s ease;
}

.task-indicator:hover {
  transform: translateY(-2px);
  box-shadow: 0 12px 32px rgba(79, 70, 229, 0.5);
}

.task-icon-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

.task-count {
  font-size: 14px;
  font-weight: 500;
}

/* 任务面板 */
.task-panel {
  position: absolute;
  bottom: 70px;
  right: 0;
  width: 340px;
  background: #fff;
  border-radius: 16px;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.15);
  overflow: hidden;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid #f1f5f9;
}

.panel-title {
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}

.task-list {
  max-height: 320px;
  overflow-y: auto;
  padding: 12px;
}

.task-item {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 12px;
  background: #f8fafc;
  border-radius: 10px;
  margin-bottom: 8px;
}

.task-item:last-child {
  margin-bottom: 0;
}

.task-info {
  flex: 1;
  min-width: 0;
}

.task-title {
  font-size: 13px;
  font-weight: 500;
  color: #1e293b;
  margin-bottom: 6px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.task-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.task-status {
  font-size: 12px;
  color: #64748b;
}

.task-progress {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
  min-width: 60px;
}

.task-progress .el-progress {
  width: 60px;
}

.progress-text {
  font-size: 11px;
  color: #64748b;
}

.panel-footer {
  padding: 12px 20px;
  border-top: 1px solid #f1f5f9;
  text-align: center;
}

/* 动画 */
.slide-up-enter-active,
.slide-up-leave-active {
  transition: all 0.3s ease;
}

.slide-up-enter-from,
.slide-up-leave-to {
  opacity: 0;
  transform: translateY(20px);
}

/* 响应式 */
@media (max-width: 768px) {
  .task-notification {
    right: 16px;
    bottom: 16px;
    left: 16px;
    max-width: none;
  }

  .task-panel {
    width: 100%;
    bottom: 60px;
  }
}
</style>
