import axios from 'axios'

const apiClient = axios.create({
  baseURL: '/v1/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// 请求拦截器
apiClient.interceptors.request.use(
  (config) => {
    // 从 localStorage 获取 token
    const token = localStorage.getItem('access_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器
apiClient.interceptors.response.use(
  (response) => {
    return response
  },
  (error) => {
    if (error.response) {
      const isLoginRequest = error.config?.url?.includes('/auth/login')

      switch (error.response.status) {
        case 401:
          // 登录请求失败不跳转，让组件自己处理错误提示
          if (!isLoginRequest) {
            console.error('未授权，请登录后重试')
            // 清除本地存储
            localStorage.removeItem('access_token')
            localStorage.removeItem('user_info')
            // 跳转到登录页
            window.location.href = '/admin/login'
          }
          break
        case 403:
          console.error('权限不足')
          break
        case 404:
          console.error('请求的资源不存在')
          break
        case 500:
          console.error('服务器内部错误')
          break
        default:
          console.error('请求失败:', error.response.data)
      }
    }
    return Promise.reject(error)
  }
)

// ============ API 方法封装 ============

export const api = {
  // ============ 认证 ============
  login(credentials: { username: string; password: string }) {
    return apiClient.post('/auth/login', credentials)
  },
  
  logout() {
    return apiClient.post('/auth/logout')
  },
  
  // ============ 服务管理 ============
  getServices(params?: any) {
    return apiClient.get('/services', { params })
  },
  
  // ============ 模型管理 ============
  getModels(params?: any) {
    return apiClient.get('/models', { params })
  },
  
  getRecommendedModels(serviceId?: string) {
    return apiClient.get('/models/recommend', { params: { service_id: serviceId } })
  },
  
  // ============ 服务发现 ============
  searchServices(query: string, page: number = 1, pageSize: number = 20) {
    return apiClient.post('/discovery/search', { query, page, page_size: pageSize })
  },
  
  importURLs(file: File) {
    const formData = new FormData()
    formData.append('file', file)
    return apiClient.post('/discovery/import', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
  
  getDiscoveryTask(taskId: string) {
    return apiClient.get(`/discovery/tasks/${taskId}`)
  },
  
  // ============ 任务管理 ============
  getTasks(params?: any) {
    return apiClient.get('/tasks', { params })
  },
  
  getTask(id: string) {
    return apiClient.get(`/tasks/${id}`)
  },
  
  // ============ 用户管理 ============
  getUsers(params?: any) {
    return apiClient.get('/users', { params })
  },
  
  getUser(id: string) {
    return apiClient.get(`/users/${id}`)
  },
  
  createUser(data: any) {
    return apiClient.post('/users', data)
  },
  
  updateUser(id: string, data: any) {
    return apiClient.put(`/users/${id}`, data)
  },
  
  deleteUser(id: string) {
    return apiClient.delete(`/users/${id}`)
  },
}

export default apiClient
