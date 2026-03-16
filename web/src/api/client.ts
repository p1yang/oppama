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

export default apiClient
