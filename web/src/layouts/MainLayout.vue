<template>
  <el-container class="layout-container" :class="{ 'dark-mode': isDarkMode, 'sidebar-collapsed': isCollapsed }">
    <!-- 移动端遮罩 -->
    <div v-if="!isCollapsed && isMobile" class="sidebar-mask" @click="toggleSidebar"></div>

    <!-- 侧边栏 -->
    <el-aside :width="isCollapsed ? '64px' : '260px'" class="sidebar" :class="{ collapsed: isCollapsed, mobile: isMobile }">
      <!-- Logo -->
      <div class="logo">
        <div class="logo-icon">
          <el-icon :size="isCollapsed ? 24 : 32"><Monitor /></el-icon>
        </div>
        <transition name="fade">
          <div v-show="!isCollapsed" class="logo-text">
            <h2>Oppama</h2>
            <span class="subtitle">服务聚合网关</span>
          </div>
        </transition>
      </div>

      <!-- 折叠按钮 -->
      <div class="collapse-btn" @click="toggleSidebar">
        <el-icon :size="18">
          <Fold v-if="!isCollapsed" />
          <Expand v-else />
        </el-icon>
      </div>

      <!-- 菜单 -->
      <el-menu
        :default-active="$route.path"
        router
        background-color="transparent"
        text-color="#cbd5e1"
        active-text-color="#ffffff"
        class="side-menu"
        :collapse="isCollapsed"
      >
        <el-menu-item index="/" class="menu-item">
          <el-icon><DataAnalysis /></el-icon>
          <template #title>
            <span>仪表盘</span>
          </template>
        </el-menu-item>
        <el-menu-item index="/services" class="menu-item">
          <el-icon><Connection /></el-icon>
          <template #title>
            <span>服务列表</span>
          </template>
        </el-menu-item>
        <el-menu-item index="/models" class="menu-item">
          <el-icon><Files /></el-icon>
          <template #title>
            <span>模型管理</span>
          </template>
        </el-menu-item>
        <el-menu-item index="/discovery" class="menu-item">
          <el-icon><Search /></el-icon>
          <template #title>
            <span>服务发现</span>
          </template>
        </el-menu-item>
        <el-menu-item index="/settings" class="menu-item">
          <el-icon><Setting /></el-icon>
          <template #title>
            <span>系统设置</span>
          </template>
        </el-menu-item>
      </el-menu>

      <!-- 侧边栏底部 -->
      <div class="sidebar-footer">
        <transition name="fade">
          <div v-show="!isCollapsed" class="footer-content">
            <el-tag size="small" type="warning" effect="plain">v1.0.0</el-tag>
          </div>
        </transition>
      </div>
    </el-aside>

    <!-- 主内容区 -->
    <el-container class="main-container">
      <!-- 顶部导航栏 -->
      <el-header class="header">
        <div class="header-left">
          <!-- 移动端菜单按钮 -->
          <el-button v-if="isMobile" class="mobile-menu-btn" :icon="Fold" @click="toggleSidebar" circle />
          <breadcrumb />
        </div>
        <div class="header-right">
          <!-- 实时状态指示器 -->
          <div class="status-indicator">
            <el-icon class="pulse"><CircleCheck /></el-icon>
            <span>运行正常</span>
          </div>

          <!-- 深色模式切换 -->
          <el-tooltip :content="isDarkMode ? '切换到浅色模式' : '切换到深色模式'" placement="bottom">
            <el-button class="theme-toggle" :icon="isDarkMode ? Sunny : Moon" circle @click="toggleDarkMode" />
          </el-tooltip>

          <el-divider direction="vertical" />

          <!-- 用户信息 -->
          <el-dropdown class="user-dropdown" trigger="click">
            <div class="user-info">
              <el-avatar :size="32" :icon="UserFilled" />
              <transition name="fade">
                <span v-show="!isMobile" class="username">管理员</span>
              </transition>
              <el-icon class="dropdown-icon"><ArrowDown /></el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item>
                  <el-icon><User /></el-icon>
                  <span>个人资料</span>
                </el-dropdown-item>
                <el-dropdown-item>
                  <el-icon><Setting /></el-icon>
                  <span>偏好设置</span>
                </el-dropdown-item>
                <el-dropdown-item divided @click="handleLogout">
                  <el-icon><SwitchButton /></el-icon>
                  <span>退出登录</span>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <!-- 主内容 -->
      <el-main class="main-content">
        <transition name="fade-slide" mode="out-in">
          <router-view :key="$route.path" />
        </transition>
      </el-main>

      <!-- 全局任务通知面板 -->
      <TaskNotification />
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  ElContainer, ElAside, ElHeader, ElMain, ElMenu, ElMenuItem, ElTag,
  ElDivider, ElAvatar, ElButton, ElTooltip, ElDropdown, ElDropdownMenu, ElDropdownItem,
  ElMessage
} from 'element-plus'
import {
  Monitor, DataAnalysis, Connection, Files, Search, Setting,
  CircleCheck, UserFilled, ArrowDown, User, SwitchButton,
  Fold, Expand, Moon, Sunny
} from '@element-plus/icons-vue'
import Breadcrumb from '@/components/Breadcrumb.vue'
import TaskNotification from '@/components/TaskNotification.vue'

const router = useRouter()

// 侧边栏折叠状态
const isCollapsed = ref(false)
const isMobile = ref(false)
const isDarkMode = ref(false)

// 检查是否为移动端
const checkMobile = () => {
  isMobile.value = window.innerWidth < 768
  if (isMobile.value) {
    isCollapsed.value = true
  }
}

// 切换侧边栏
const toggleSidebar = () => {
  isCollapsed.value = !isCollapsed.value
}

// 切换深色模式
const toggleDarkMode = () => {
  isDarkMode.value = !isDarkMode.value
  document.documentElement.classList.toggle('dark', isDarkMode.value)
  localStorage.setItem('theme', isDarkMode.value ? 'dark' : 'light')
}

// 初始化主题
const initTheme = () => {
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    isDarkMode.value = true
    document.documentElement.classList.add('dark')
  }
}

// 退出登录
const handleLogout = () => {
  // 清除本地存储
  localStorage.removeItem('access_token')
  localStorage.removeItem('user_info')
  
  ElMessage.success('已退出登录')
  
  // 跳转到登录页
  router.push('/login')
}

onMounted(() => {
  checkMobile()
  initTheme()
  window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped>
.layout-container {
  height: 100vh;
  transition: all 0.3s ease;
}

/* 侧边栏遮罩（移动端） */
.sidebar-mask {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 99;
  animation: fadeIn 0.3s ease;
}

/* 侧边栏 */
.sidebar {
  background: linear-gradient(180deg, #1e293b 0%, #0f172a 100%);
  color: #fff;
  display: flex;
  flex-direction: column;
  box-shadow: 4px 0 24px rgba(0, 0, 0, 0.15);
  z-index: 100;
  transition: width 0.3s ease;
  position: relative;
  overflow: hidden;
}

.sidebar.mobile {
  position: fixed;
  height: 100vh;
  left: 0;
  top: 0;
}

.sidebar.collapsed {
  width: 64px !important;
}

/* Logo */
.logo {
  height: 70px;
  display: flex;
  align-items: center;
  padding: 0 24px;
  background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
  box-shadow: 0 4px 12px rgba(79, 70, 229, 0.3);
  transition: padding 0.3s ease;
}

.sidebar.collapsed .logo {
  padding: 0;
  justify-content: center;
}

.logo-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  background: rgba(255, 255, 255, 0.15);
  border-radius: 12px;
  color: #ffffff;
  flex-shrink: 0;
  transition: all 0.3s ease;
}

.sidebar.collapsed .logo-icon {
  width: 40px;
  height: 40px;
}

.logo-text {
  display: flex;
  flex-direction: column;
  margin-left: 12px;
}

.logo-text h2 {
  color: #ffffff;
  font-size: 20px;
  font-weight: 700;
  margin: 0;
  letter-spacing: 0.5px;
}

.subtitle {
  color: rgba(255, 255, 255, 0.7);
  font-size: 11px;
  margin-top: 2px;
}

/* 折叠按钮 */
.collapse-btn {
  position: absolute;
  top: 80px;
  right: -12px;
  width: 24px;
  height: 24px;
  background: #4f46e5;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: #fff;
  box-shadow: 0 2px 8px rgba(79, 70, 229, 0.4);
  transition: all 0.3s ease;
  z-index: 10;
}

.collapse-btn:hover {
  transform: scale(1.1);
  box-shadow: 0 4px 12px rgba(79, 70, 229, 0.6);
}

.sidebar.collapsed .collapse-btn {
  right: -12px;
}

/* 菜单 */
.side-menu {
  flex: 1;
  border-right: none;
  padding: 16px 12px;
  background: transparent;
  overflow-x: hidden;
}

.side-menu:not(.el-menu--collapse) {
  width: 260px;
}

.menu-item {
  margin-bottom: 8px;
  border-radius: 8px;
  transition: all 0.3s ease;
  height: 50px;
  display: flex;
  align-items: center;
}

.menu-item:hover {
  background: rgba(255, 255, 255, 0.08) !important;
  transform: translateX(4px);
}

.menu-item.is-active {
  background: linear-gradient(90deg, rgba(79, 70, 229, 0.9) 0%, rgba(124, 58, 237, 0.8) 100%) !important;
  box-shadow: 0 4px 12px rgba(79, 70, 229, 0.4);
}

.menu-item .el-icon {
  font-size: 18px;
}

/* 侧边栏底部 */
.sidebar-footer {
  padding: 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
}

.footer-content {
  display: flex;
  justify-content: center;
}

/* 主容器 */
.main-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* 顶部导航栏 */
.header {
  background: #ffffff;
  border-bottom: 1px solid #e2e8f0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 32px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
  height: 70px;
  transition: all 0.3s ease;
}

.dark-mode .header {
  background: #1e293b;
  border-bottom-color: #334155;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.mobile-menu-btn {
  display: none;
}

@media (max-width: 768px) {
  .mobile-menu-btn {
    display: flex;
  }

  .header {
    padding: 0 16px;
  }
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* 状态指示器 */
.status-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #10b981;
  font-weight: 500;
  padding: 8px 16px;
  background: linear-gradient(135deg, #ecfdf5 0%, #d1fae5 100%);
  border-radius: 20px;
  font-size: 13px;
  transition: all 0.3s ease;
}

.dark-mode .status-indicator {
  background: linear-gradient(135deg, #064e3b 0%, #065f46 100%);
  color: #34d399;
}

.status-indicator .el-icon {
  font-size: 16px;
}

.pulse {
  animation: pulse-animation 2s infinite;
}

@keyframes pulse-animation {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.6;
  }
}

/* 主题切换按钮 */
.theme-toggle {
  border: none;
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
  color: #475569;
  transition: all 0.3s ease;
}

.dark-mode .theme-toggle {
  background: linear-gradient(135deg, #334155 0%, #1e293b 100%);
  color: #94a3b8;
}

.theme-toggle:hover {
  transform: rotate(15deg) scale(1.1);
}

/* 用户信息 */
.user-dropdown {
  cursor: pointer;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 12px;
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
  border-radius: 20px;
  cursor: pointer;
  transition: all 0.3s ease;
}

.dark-mode .user-info {
  background: linear-gradient(135deg, #334155 0%, #1e293b 100%);
}

.user-info:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
  transform: translateY(-2px);
}

.username {
  font-size: 13px;
  font-weight: 500;
  color: #334155;
}

.dark-mode .username {
  color: #cbd5e1;
}

.dropdown-icon {
  font-size: 12px;
  color: #64748b;
  transition: transform 0.3s ease;
}

.user-info:hover .dropdown-icon {
  transform: rotate(180deg);
}

/* 主内容区 */
.main-content {
  background: transparent;
  padding: 24px;
  overflow-y: auto;
  overflow-x: hidden;
}

@media (max-width: 768px) {
  .main-content {
    padding: 16px;
  }
}

/* 动画 */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.fade-slide-enter-active {
  transition: all 0.3s ease-out;
}

.fade-slide-leave-active {
  transition: all 0.2s ease-in;
}

.fade-slide-enter-from {
  opacity: 0;
  transform: translateY(20px);
}

.fade-slide-leave-to {
  opacity: 0;
  transform: translateY(-20px);
}

@keyframes fadeIn {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}
</style>
