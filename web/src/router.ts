import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory('/admin/'),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/Login.vue'),
      meta: { title: '登录' }
    },
    {
      path: '/',
      component: () => import('@/layouts/MainLayout.vue'),
      redirect: '/dashboard',
      children: [
        {
          path: '/dashboard',
          name: 'dashboard',
          component: () => import('@/views/Home.vue'),
          meta: { title: '仪表盘' }
        },
        {
          path: '/services',
          name: 'services',
          component: () => import('@/views/Services.vue'),
          meta: { title: '服务列表' }
        },
        {
          path: '/models',
          name: 'models',
          component: () => import('@/views/Models.vue'),
          meta: { title: '模型管理' }
        },
        {
          path: '/discovery',
          name: 'discovery',
          component: () => import('@/views/Discovery.vue'),
          meta: { title: '服务发现' }
        },
        {
          path: '/settings',
          name: 'settings',
          component: () => import('@/views/Settings.vue'),
          meta: { title: '系统设置' }
        },
      ],
    },
  ],
})

// 路由守卫 - 检查登录状态
router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('access_token')
  
  // 如果访问登录页，直接放行
  if (to.path === '/login') {
    // 如果已登录，跳转到首页
    if (token) {
      next('/dashboard')
    } else {
      next()
    }
    return
  }

  // 如果没有 Token，重定向到登录页
  if (!token) {
    next('/login')
    return
  }

  // 有 Token，继续访问
  next()
})

export default router
