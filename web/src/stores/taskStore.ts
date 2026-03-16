import { defineStore } from 'pinia'
import { ref } from 'vue'

export type TaskStatus = 'pending' | 'running' | 'completed' | 'failed'
export type TaskType = 'service-check' | 'batch-check' | 'discovery-search' | 'model-sync'

export interface AsyncTask {
  id: string
  type: TaskType
  title: string
  status: TaskStatus
  progress: number
  total: number
  result?: any
  error?: string
  createdAt: Date
  updatedAt: Date
}

export const useTaskStore = defineStore('task', () => {
  // 任务列表
  const tasks = ref<Map<string, AsyncTask>>(new Map())

  // 当前活动的任务
  const activeTasks = ref<AsyncTask[]>([])

  // 添加任务
  const addTask = (task: Omit<AsyncTask, 'createdAt' | 'updatedAt'>): AsyncTask => {
    const newTask: AsyncTask = {
      ...task,
      createdAt: new Date(),
      updatedAt: new Date(),
    }
    tasks.value.set(newTask.id, newTask)
    updateActiveTasks()
    return newTask
  }

  // 更新任务
  const updateTask = (id: string, updates: Partial<AsyncTask>): void => {
    const task = tasks.value.get(id)
    if (task) {
      Object.assign(task, updates, { updatedAt: new Date() })
      updateActiveTasks()
    }
  }

  // 获取任务
  const getTask = (id: string): AsyncTask | undefined => {
    return tasks.value.get(id)
  }

  // 移除任务
  const removeTask = (id: string): void => {
    tasks.value.delete(id)
    updateActiveTasks()
  }

  // 清空已完成/失败的任务
  const clearCompletedTasks = (): void => {
    for (const [id, task] of tasks.value) {
      if (task.status === 'completed' || task.status === 'failed') {
        tasks.value.delete(id)
      }
    }
    updateActiveTasks()
  }

  // 更新活动任务列表
  const updateActiveTasks = (): void => {
    activeTasks.value = Array.from(tasks.value.values())
      .filter(t => t.status === 'pending' || t.status === 'running')
      .sort((a, b) => a.createdAt.getTime() - b.createdAt.getTime())
  }

  // 获取指定类型的任务
  const getTasksByType = (type: TaskType): AsyncTask[] => {
    return Array.from(tasks.value.values())
      .filter(t => t.type === type)
      .sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime())
  }

  // 轮询任务状态
  const pollTask = async (
    taskId: string,
    pollFn: () => Promise<any>,
    onUpdate?: (task: AsyncTask) => void,
    onComplete?: (task: AsyncTask) => void,
    onError?: (task: AsyncTask, error: any) => void,
    interval: number = 2000,
    maxDuration: number = 300000
  ): Promise<void> => {
    const task = getTask(taskId)
    if (!task) return

    const startTime = Date.now()
    let pollTimer: any = null

    const poll = async () => {
      try {
        const result = await pollFn()
        // 兼容不同的响应格式
        const data = result.data?.data || result.data || {}

        // 更新任务状态
        if (data.status === 'completed' || data.status === 'failed') {
          updateTask(taskId, {
            status: data.status,
            progress: data.total || task.total,
            result: data,
          })
          onComplete?.(getTask(taskId)!)
          return
        }

        // 更新进度
        updateTask(taskId, {
          progress: data.progress || task.progress,
          total: data.total || task.total,
        })
        onUpdate?.(getTask(taskId)!)

        // 继续轮询
        if (Date.now() - startTime < maxDuration) {
          pollTimer = setTimeout(poll, interval)
        } else {
          // 超时
          updateTask(taskId, { status: 'failed', error: '任务超时' })
          onError?.(getTask(taskId)!, new Error('任务超时'))
        }
      } catch (error: any) {
        updateTask(taskId, { status: 'failed', error: error.message })
        onError?.(getTask(taskId)!, error)
      }
    }

    poll()

    // 返回清理函数
    return () => {
      if (pollTimer) {
        clearTimeout(pollTimer)
      }
    }
  }

  return {
    tasks,
    activeTasks,
    addTask,
    updateTask,
    getTask,
    removeTask,
    clearCompletedTasks,
    getTasksByType,
    pollTask,
  }
})
