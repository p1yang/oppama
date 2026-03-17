<template>
  <div class="settings-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <div class="header-content">
        <div class="header-icon">
          <el-icon :size="28"><Setting /></el-icon>
        </div>
        <div>
          <h1 class="page-title">系统设置</h1>
          <p class="page-desc">配置系统参数、API 密钥和检测器选项</p>
        </div>
      </div>
    </div>

    <!-- 设置选项卡 -->
    <el-card class="settings-card" shadow="never">
      <el-tabs v-model="activeTab" class="settings-tabs">
        <!-- 代理配置 -->
        <el-tab-pane name="proxy">
          <template #label>
            <div class="tab-label">
              <el-icon><Connection /></el-icon>
              <span>代理配置</span>
            </div>
          </template>

          <div class="setting-section">
            <div class="section-header">
              <div class="section-icon primary">
                <el-icon><Connection /></el-icon>
              </div>
              <div>
                <h3 class="section-title">服务代理设置</h3>
                <p class="section-desc">配置 Oppama 服务聚合网关的监听参数</p>
              </div>
            </div>

            <el-form :model="proxyConfig" label-width="140px" class="settings-form">
              <div class="form-group">
                <div class="form-group-title">
                  <el-icon><Lock /></el-icon>
                  <span>认证设置</span>
                </div>
                <el-form-item label="启用认证">
                  <el-switch
                    v-model="proxyConfig.enable_auth"
                    active-text="已启用"
                    inactive-text="未启用"
                    :active-icon="Unlock"
                    :inactive-icon="Lock"
                  />
                  <template #suffix>
                    <el-text size="small" type="info">启用后需要 API Key 才能访问</el-text>
                  </template>
                </el-form-item>
                <el-form-item label="API Key" v-if="proxyConfig.enable_auth">
                  <el-input
                    v-model="proxyConfig.api_key"
                    type="password"
                    show-password
                    placeholder="请输入 API Key"
                    clearable
                  >
                    <template #prepend>
                      <el-icon><Key /></el-icon>
                    </template>
                    <template #append>
                      <el-button :icon="RefreshRight" @click="generateApiKey">生成</el-button>
                    </template>
                  </el-input>
                  <template #suffix>
                    <el-text size="small" type="info">用于 API 访问的密钥</el-text>
                  </template>
                </el-form-item>
              </div>

              <div class="form-group">
                <div class="form-group-title">
                  <el-icon><Link /></el-icon>
                  <span>HTTP 代理</span>
                </div>
                <el-form-item label="HTTP 代理">
                  <el-input v-model="proxyConfig.http_proxy" placeholder="http://proxy.example.com:8080" clearable>
                    <template #prepend>
                      <el-icon><Connection /></el-icon>
                    </template>
                  </el-input>
                  <template #suffix>
                    <el-text size="small" type="info">访问 Ollama 服务时使用的 HTTP 代理</el-text>
                  </template>
                </el-form-item>
                <el-form-item label="HTTPS 代理">
                  <el-input v-model="proxyConfig.https_proxy" placeholder="http://proxy.example.com:8080" clearable>
                    <template #prepend>
                      <el-icon><Lock /></el-icon>
                    </template>
                  </el-input>
                  <template #suffix>
                    <el-text size="small" type="info">访问 HTTPS Ollama 服务时使用的代理</el-text>
                  </template>
                </el-form-item>
                <el-form-item label="不使用代理">
                  <el-input v-model="proxyConfig.no_proxy" placeholder="localhost,127.0.0.1,*.local" clearable>
                    <template #prepend>
                      <el-icon><CircleClose /></el-icon>
                    </template>
                  </el-input>
                  <template #suffix>
                    <el-text size="small" type="info">逗号分隔的地址列表，这些地址不使用代理</el-text>
                  </template>
                </el-form-item>
              </div>

              <div class="form-group">
                <div class="form-group-title">
                  <el-icon><Setting /></el-icon>
                  <span>高级选项</span>
                </div>
                <el-form-item label="默认模型">
                  <el-input v-model="proxyConfig.default_model" placeholder="留空则自动选择" clearable>
                    <template #prepend>
                      <el-icon><Files /></el-icon>
                    </template>
                  </el-input>
                  <template #suffix>
                    <el-text size="small" type="info">未指定模型时使用的默认模型</el-text>
                  </template>
                </el-form-item>
                <el-form-item label="故障转移">
                  <el-switch
                    v-model="proxyConfig.fallback_enabled"
                    active-text="已启用"
                    inactive-text="未启用"
                  />
                  <template #suffix>
                    <el-text size="small" type="info">当服务不可用时自动切换到备用服务</el-text>
                  </template>
                </el-form-item>
              </div>

              <div class="form-actions">
                <el-button type="primary" :icon="Select" @click="saveProxyConfig" :loading="saving">
                  保存配置
                </el-button>
                <el-button :icon="RefreshLeft" @click="loadProxyConfig">重置</el-button>
              </div>
            </el-form>
          </div>
        </el-tab-pane>

        <!-- 搜索引擎配置 -->
        <el-tab-pane name="engines">
          <template #label>
            <div class="tab-label">
              <el-icon><Search /></el-icon>
              <span>搜索引擎 API</span>
            </div>
          </template>

          <div class="setting-section">
            <div class="section-header">
              <div class="section-icon success">
                <el-icon><Platform /></el-icon>
              </div>
              <div>
                <h3 class="section-title">搜索引擎 API 配置</h3>
                <p class="section-desc">配置各搜索引擎的 API 密钥以启用服务发现功能</p>
              </div>
            </div>

            <el-form :model="engineConfig" label-width="120px" class="settings-form">
              <!-- FOFA -->
              <div class="engine-card">
                <div class="engine-header">
                  <div class="engine-info">
                    <span class="engine-emoji">🔍</span>
                    <div>
                      <h4 class="engine-name">FOFA</h4>
                      <p class="engine-desc">网络空间搜索引擎</p>
                    </div>
                  </div>
                  <el-switch
                    v-model="engineConfig.fofa_enabled"
                    active-color="#4f46e5"
                    :active-icon="Check"
                  />
                </div>
                <div class="engine-body" v-if="engineConfig.fofa_enabled">
                  <el-form-item label="Email">
                    <el-input v-model="engineConfig.fofa_email" placeholder="your@email.com" clearable />
                  </el-form-item>
                  <el-form-item label="API Key">
                    <el-input v-model="engineConfig.fofa_key" type="password" show-password clearable />
                  </el-form-item>
                </div>
              </div>

              <!-- Hunter -->
              <div class="engine-card">
                <div class="engine-header">
                  <div class="engine-info">
                    <span class="engine-emoji">🎯</span>
                    <div>
                      <h4 class="engine-name">Hunter</h4>
                      <p class="engine-desc">互联网空间测绘搜索引擎</p>
                    </div>
                  </div>
                  <el-switch
                    v-model="engineConfig.hunter_enabled"
                    active-color="#10b981"
                    :active-icon="Check"
                  />
                </div>
                <div class="engine-body" v-if="engineConfig.hunter_enabled">
                  <el-form-item label="API Key">
                    <el-input v-model="engineConfig.hunter_key" type="password" show-password clearable />
                  </el-form-item>
                </div>
              </div>

              <!-- Shodan -->
              <div class="engine-card">
                <div class="engine-header">
                  <div class="engine-info">
                    <span class="engine-emoji">🌐</span>
                    <div>
                      <h4 class="engine-name">Shodan</h4>
                      <p class="engine-desc">物联网搜索引擎</p>
                    </div>
                  </div>
                  <el-switch
                    v-model="engineConfig.shodan_enabled"
                    active-color="#dc2626"
                    :active-icon="Check"
                  />
                </div>
                <div class="engine-body" v-if="engineConfig.shodan_enabled">
                  <el-form-item label="API Key">
                    <el-input v-model="engineConfig.shodan_key" type="password" show-password clearable />
                  </el-form-item>
                </div>
              </div>

              <div class="form-actions">
                <el-button type="primary" :icon="Select" @click="saveEngineConfig" :loading="saving">
                  保存配置
                </el-button>
                <el-button :icon="Connection" @click="testEngineConfig">测试连接</el-button>
              </div>
            </el-form>
          </div>
        </el-tab-pane>

        <!-- 检测器配置 -->
        <el-tab-pane name="detector">
          <template #label>
            <div class="tab-label">
              <el-icon><Monitor /></el-icon>
              <span>检测器配置</span>
            </div>
          </template>

          <div class="setting-section">
            <div class="section-header">
              <div class="section-icon warning">
                <el-icon><Monitor /></el-icon>
              </div>
              <div>
                <h3 class="section-title">服务检测器设置</h3>
                <p class="section-desc">配置服务健康检查和蜜罐检测参数</p>
              </div>
            </div>

            <el-form :model="detectorConfig" label-width="140px" class="settings-form">
              <div class="form-group">
                <div class="form-group-title">
                  <el-icon><Timer /></el-icon>
                  <span>检测参数</span>
                </div>
                <el-form-item label="并发数">
                  <el-slider v-model="detectorConfig.concurrency" :min="1" :max="50" :step="1" show-input />
                  <template #suffix>
                    <el-text size="small" type="info">同时检测的服务数量</el-text>
                  </template>
                </el-form-item>
                <el-form-item label="超时时间 (秒)">
                  <el-slider v-model="detectorConfig.timeout" :min="5" :max="300" :step="5" show-input />
                  <template #suffix>
                    <el-text size="small" type="info">单个服务检测的最大等待时间</el-text>
                  </template>
                </el-form-item>
              </div>

              <div class="form-group">
                <div class="form-group-title">
                  <el-icon><Clock /></el-icon>
                  <span>定时任务间隔</span>
                </div>
                <el-form-item label="健康检查间隔">
                  <el-input-number
                    v-model="detectorConfig.health_check_interval"
                    :min="1"
                    :max="60"
                    :step="1"
                    style="width: 150px;"
                  />
                  <span style="margin-left: 12px;">分钟</span>
                  <template #suffix>
                    <el-text size="small" type="info">定期检查服务存活状态的时间间隔</el-text>
                  </template>
                </el-form-item>
                <el-form-item label="模型同步间隔">
                  <el-input-number
                    v-model="detectorConfig.model_sync_interval"
                    :min="1"
                    :max="120"
                    :step="1"
                    style="width: 150px;"
                  />
                  <span style="margin-left: 12px;">分钟</span>
                  <template #suffix>
                    <el-text size="small" type="info">定期同步在线服务模型列表的时间间隔</el-text>
                  </template>
                </el-form-item>
              </div>

              <div class="form-group">
                <div class="form-group-title">
                  <el-icon><Warning /></el-icon>
                  <span>蜜罐检测</span>
                </div>
                <el-form-item label="启用蜜罐检测">
                  <el-switch
                    v-model="detectorConfig.honeypot_enabled"
                    active-text="已启用"
                    inactive-text="未启用"
                  />
                  <template #suffix>
                    <el-text size="small" type="info">自动识别并标记蜜罐服务</el-text>
                  </template>
                </el-form-item>
                <el-form-item label="检测阈值">
                  <el-slider v-model="detectorConfig.honeypot_threshold" :min="1" :max="10" :step="1" show-input />
                  <template #suffix>
                    <el-text size="small" type="info">蜜罐特征匹配阈值 (1-10)</el-text>
                  </template>
                </el-form-item>
              </div>

              <div class="form-actions">
                <el-button type="primary" :icon="Select" @click="saveDetectorConfig" :loading="saving">
                  保存配置
                </el-button>
                <el-button :icon="RefreshLeft" @click="loadDetectorConfig">重置</el-button>
              </div>
            </el-form>
          </div>
        </el-tab-pane>

        <!-- 账户设置 -->
        <el-tab-pane name="account">
          <template #label>
            <div class="tab-label">
              <el-icon><User /></el-icon>
              <span>账户设置</span>
            </div>
          </template>

          <div class="setting-section">
            <div class="section-header">
              <div class="section-icon info">
                <el-icon><User /></el-icon>
              </div>
              <div>
                <h3 class="section-title">账户安全</h3>
                <p class="section-desc">修改您的账户密码</p>
              </div>
            </div>

            <el-form :model="passwordForm" label-width="120px" class="settings-form" :rules="passwordRules" ref="passwordFormRef">
              <el-form-item label="原密码" prop="old_password">
                <el-input
                  v-model="passwordForm.old_password"
                  type="password"
                  show-password
                  placeholder="请输入原密码"
                  clearable
                />
              </el-form-item>
              <el-form-item label="新密码" prop="new_password">
                <el-input
                  v-model="passwordForm.new_password"
                  type="password"
                  show-password
                  placeholder="请输入新密码（至少8位，包含大小写字母和数字）"
                  clearable
                />
              </el-form-item>
              <el-form-item label="确认密码" prop="confirm_password">
                <el-input
                  v-model="passwordForm.confirm_password"
                  type="password"
                  show-password
                  placeholder="请再次输入新密码"
                  clearable
                />
              </el-form-item>

              <div class="form-actions">
                <el-button type="primary" :icon="Select" @click="changePassword" :loading="saving">
                  修改密码
                </el-button>
                <el-button :icon="RefreshLeft" @click="resetPasswordForm">重置</el-button>
              </div>
            </el-form>
          </div>
        </el-tab-pane>

        <!-- 用户管理（仅管理员） -->
        <el-tab-pane name="users" v-if="isAdmin">
          <template #label>
            <div class="tab-label">
              <el-icon><UserFilled /></el-icon>
              <span>用户管理</span>
            </div>
          </template>

          <div class="setting-section">
            <div class="section-header">
              <div class="section-icon success">
                <el-icon><UserFilled /></el-icon>
              </div>
              <div>
                <h3 class="section-title">用户管理</h3>
                <p class="section-desc">管理系统用户和权限</p>
              </div>
              <div style="margin-left: auto;">
                <el-button type="primary" :icon="Plus" @click="showCreateUserDialog">创建用户</el-button>
              </div>
            </div>

            <el-table :data="users" style="width: 100%" v-loading="loadingUsers">
              <el-table-column prop="username" label="用户名" width="150" />
              <el-table-column prop="nickname" label="昵称" width="150" />
              <el-table-column prop="email" label="邮箱" />
              <el-table-column prop="role" label="角色" width="100">
                <template #default="{ row }">
                  <el-tag :type="row.role === 'admin' ? 'danger' : 'primary'" size="small">
                    {{ row.role === 'admin' ? '管理员' : '普通用户' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="status" label="状态" width="120">
                <template #default="{ row }">
                  <el-tag
                    :type="getStatusType(row.status)"
                    size="small"
                  >
                    {{ getStatusText(row.status) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="220" fixed="right">
                <template #default="{ row }">
                  <el-button link type="primary" size="small" @click="editUser(row)">
                    <el-icon><Edit /></el-icon> 编辑
                  </el-button>
                  <el-button link type="warning" size="small" @click="resetUserPassword(row)">
                    <el-icon><RefreshRight /></el-icon> 重置密码
                  </el-button>
                  <el-popconfirm
                    v-if="row.id !== currentUserId"
                    title="确定要删除此用户吗？"
                    @confirm="deleteUser(row.id)"
                  >
                    <template #reference>
                      <el-button link type="danger" size="small">
                        <el-icon><Delete /></el-icon> 删除
                      </el-button>
                    </template>
                  </el-popconfirm>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-tab-pane>

        <!-- 关于 -->
        <el-tab-pane name="about">
          <template #label>
            <div class="tab-label">
              <el-icon><InfoFilled /></el-icon>
              <span>关于</span>
            </div>
          </template>

          <div class="about-section">
            <div class="about-logo">
              <div class="logo-inner">
                <el-icon :size="64"><Monitor /></el-icon>
              </div>
            </div>
            <h2 class="about-title">Oppama</h2>
            <p class="about-version">v1.0.0</p>
            <p class="about-desc">
              Oppama - 一个功能强大的 Ollama 服务发现、聚合和管理工具
            </p>

            <div class="about-info">
              <div class="info-item">
                <span class="info-label">作者</span>
                <span class="info-value">P1yang, Qwen</span>
              </div>
              <div class="info-item">
                <span class="info-label">许可证</span>
                <span class="info-value">MIT License</span>
              </div>
              <div class="info-item">
                <span class="info-label">项目地址</span>
                <el-link type="primary" :underline="false" href="https://github.com" target="_blank">
                  <el-icon><Link /></el-icon>
                  github.com/oppama
                </el-link>
              </div>
            </div>

            <div class="about-actions">
              <el-button type="primary" :icon="Document">查看文档</el-button>
              <el-button :icon="ChatLineRound">反馈问题</el-button>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <!-- 用户编辑对话框 -->
    <el-dialog
      v-model="userDialogVisible"
      :title="userForm.id ? '编辑用户' : '创建用户'"
      width="500px"
      @close="userDialogRef?.clearValidate()"
    >
      <el-form :model="userForm" label-width="80px" :rules="userFormRules" ref="userDialogRef">
        <el-form-item label="用户名" prop="username">
          <el-input
            v-model="userForm.username"
            :disabled="!!userForm.id"
            placeholder="请输入用户名"
          />
        </el-form-item>
        <el-form-item label="昵称" prop="nickname">
          <el-input v-model="userForm.nickname" placeholder="请输入昵称" />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="userForm.email" placeholder="请输入邮箱" />
        </el-form-item>
        <el-form-item label="角色" prop="role">
          <el-select v-model="userForm.role" placeholder="请选择角色" style="width: 100%">
            <el-option label="普通用户" value="user" />
            <el-option label="管理员" value="admin" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态" prop="status" v-if="userForm.id">
          <el-select v-model="userForm.status" placeholder="请选择状态" style="width: 100%">
            <el-option label="正常" value="active" />
            <el-option label="已禁用" value="disabled" />
            <el-option label="已锁定" value="locked" />
            <el-option label="需修改密码" value="require_password_change" />
          </el-select>
        </el-form-item>
        <el-form-item label="密码" prop="password" v-if="!userForm.id">
          <el-input
            v-model="userForm.password"
            type="password"
            show-password
            placeholder="请输入密码（至少8位，包含大小写字母和数字）"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="userDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveUser" :loading="saving">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed, watch } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  Setting,
  Connection,
  Lock,
  Unlock,
  Key,
  RefreshRight,
  Search,
  Platform,
  Timer,
  Warning,
  User,
  Plus,
  Edit,
  Delete,
  RefreshLeft,
  Select,
  Coin,
  DataLine,
  Stamp,
  View,
  Grid,
  List,
  Monitor,
  Lightning,
  Clock,
  InfoFilled,
  ChatDotRound,
  CopyDocument,
  Close,
  Promoted
} from '@element-plus/icons-vue'
import api from '@/api/client'

const activeTab = ref('proxy')
const saving = ref(false)

// 用户信息
const currentUser = ref<any>(null)
const currentUserId = computed(() => currentUser.value?.id || '')
const isAdmin = computed(() => currentUser.value?.role === 'admin')

const proxyConfig = reactive({
  enable_auth: true,
  api_key: '',
  default_model: '',
  fallback_enabled: true,
  max_retries: 3,
  timeout: 120,
  http_proxy: '',
  https_proxy: '',
  no_proxy: '',
  rate_limit: {
    enabled: false,
    requests_per_minute: 60,
  },
})

const engineConfig = reactive({
  fofa_enabled: true,
  fofa_email: '',
  fofa_key: '',
  hunter_enabled: false,
  hunter_key: '',
  shodan_enabled: false,
  shodan_key: '',
})

const detectorConfig = reactive({
  concurrency: 10,
  timeout: 30,
  honeypot_enabled: true,
  honeypot_threshold: 5,
  health_check_interval: 5,   // 健康检查间隔（分钟）
  model_sync_interval: 10,    // 模型同步间隔（分钟）
})

// 密码修改表单
const passwordFormRef = ref<FormInstance>()
const passwordForm = reactive({
  old_password: '',
  new_password: '',
  confirm_password: '',
})

const passwordRules: FormRules = {
  old_password: [
    { required: true, message: '请输入原密码', trigger: 'blur' },
  ],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 8, message: '密码长度至少8位', trigger: 'blur' },
    {
      pattern: /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)/,
      message: '密码必须包含大小写字母和数字',
      trigger: 'blur',
    },
  ],
  confirm_password: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== passwordForm.new_password) {
          callback(new Error('两次输入的密码不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur',
    },
  ],
}

// 用户管理
const users = ref<any[]>([])
const loadingUsers = ref(false)
const userDialogRef = ref<FormInstance>()
const userDialogVisible = ref(false)
const userForm = reactive({
  id: '',
  username: '',
  nickname: '',
  email: '',
  role: 'user',
  status: 'active',
  password: '',
})
const userFormRules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 50, message: '用户名长度在3-50之间', trigger: 'blur' },
  ],
  password: [
    {
      validator: (rule, value, callback) => {
        if (!userForm.id && !value) {
          callback(new Error('请输入密码'))
        } else if (value && value.length < 8) {
          callback(new Error('密码长度至少8位'))
        } else if (value && !/^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)/.test(value)) {
          callback(new Error('密码必须包含大小写字母和数字'))
        } else {
          callback()
        }
      },
      trigger: 'blur',
    },
  ],
  email: [
    { type: 'email', message: '请输入正确的邮箱地址', trigger: 'blur' },
  ],
}

const loadProxyConfig = async () => {
  try {
    const res = await api.get('/proxy/config')
    const data = res.data.data
    if (data) {
      // 加载代理配置
      proxyConfig.enable_auth = data.enable_auth ?? true
      proxyConfig.api_key = data.api_key || ''
      proxyConfig.default_model = data.default_model || ''
      proxyConfig.fallback_enabled = data.fallback_enabled ?? true
      proxyConfig.max_retries = data.max_retries ?? 3
      proxyConfig.timeout = data.timeout ?? 120
      proxyConfig.http_proxy = data.http_proxy || ''
      proxyConfig.https_proxy = data.https_proxy || ''
      proxyConfig.no_proxy = data.no_proxy || ''
      // 加载 rate_limit 配置
      if (data.rate_limit) {
        proxyConfig.rate_limit.enabled = data.rate_limit.enabled ?? false
        proxyConfig.rate_limit.requests_per_minute = data.rate_limit.requests_per_minute ?? 60
      }
      // 加载搜索引擎配置
      if (data.search_engines) {
        engineConfig.fofa_enabled = data.search_engines.fofa_enabled ?? true
        engineConfig.fofa_email = data.search_engines.fofa_email || ''
        engineConfig.fofa_key = data.search_engines.fofa_key || ''
        engineConfig.hunter_enabled = data.search_engines.hunter_enabled ?? false
        engineConfig.hunter_key = data.search_engines.hunter_key || ''
        engineConfig.shodan_enabled = data.search_engines.shodan_enabled ?? false
        engineConfig.shodan_key = data.search_engines.shodan_key || ''
      }
      // 加载检测器配置
      if (data.detector) {
        detectorConfig.concurrency = data.detector.concurrency ?? 10
        detectorConfig.timeout = data.detector.timeout ?? 30
        detectorConfig.honeypot_enabled = data.detector.honeypot_enabled ?? true
        detectorConfig.honeypot_threshold = data.detector.honeypot_threshold ?? 5
        // 加载时间间隔配置
        detectorConfig.health_check_interval = data.detector.health_check_interval ?? 5
        detectorConfig.model_sync_interval = data.detector.model_sync_interval ?? 10
      }
      
      // 如果启用了认证且有 API Key，保存到 localStorage
      if (proxyConfig.enable_auth && proxyConfig.api_key) {
        localStorage.setItem('api_key', proxyConfig.api_key)
      }
    }
  } catch (error: any) {
    console.error('加载配置失败:', error)
  }
}

const saveProxyConfig = async () => {
  saving.value = true
  try {
    await api.put('/proxy/config', proxyConfig)
    ElMessage.success('代理配置已保存')
    
    // 如果启用了认证，保存 API Key 到 localStorage 供 OpenAI 接口使用
    if (proxyConfig.enable_auth && proxyConfig.api_key) {
      localStorage.setItem('api_key', proxyConfig.api_key)
    } else {
      localStorage.removeItem('api_key')
    }
  } catch (error: any) {
    ElMessage.error('保存失败：' + error.message)
  } finally {
    saving.value = false
  }
}

const generateApiKey = () => {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  let key = 'sk-'
  for (let i = 0; i < 48; i++) {
    key += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  proxyConfig.api_key = key
  ElMessage.success('已生成新的 API Key')
}

const saveEngineConfig = async () => {
  saving.value = true
  try {
    await api.put('/proxy/config', { search_engines: engineConfig })
    ElMessage.success('搜索引擎配置已保存')
  } catch (error: any) {
    ElMessage.error('保存失败：' + error.message)
  } finally {
    saving.value = false
  }
}

const testEngineConfig = async () => {
  // 获取已启用的引擎列表
  const enginesToTest = []
  if (engineConfig.fofa_enabled) {
    enginesToTest.push('fofa')
  }
  if (engineConfig.hunter_enabled) {
    enginesToTest.push('hunter')
  }
  if (engineConfig.shodan_enabled) {
    enginesToTest.push('shodan')
  }

  if (enginesToTest.length === 0) {
    ElMessage.warning('请先启用至少一个搜索引擎')
    return
  }

  // 显示测试中状态
  const loading = ElMessage({
    message: '正在测试搜索引擎连接...',
    type: 'info',
    duration: 0,
  })

  try {
    const res = await api.post('/proxy/test-engines', { engines: enginesToTest })
    loading.close()

    const results = res.data.data || []
    let successCount = 0
    let failCount = 0

    // 使用 ElMessageBox 展示详细结果
    let resultHTML = '<div style="max-height: 300px; overflow-y: auto;">'
    results.forEach((result: any) => {
      const icon = result.success ? '✅' : '❌'
      const engineName = getEngineDisplayName(result.engine)
      const message = result.message
      const quotaInfo = result.quota !== undefined ? ` (剩余配额: ${result.quota})` : ''
      resultHTML += `<p style="margin: 8px 0;">${icon} <strong>${engineName}</strong>: ${message}${quotaInfo}</p>`

      if (result.success) successCount++
      else failCount++
    })
    resultHTML += '</div>'

    // 显示结果对话框
    ElMessageBox.alert(resultHTML, '测试连接结果', {
      dangerouslyUseHTMLString: true,
      confirmButtonText: '确定',
      type: failCount === 0 ? 'success' : 'warning',
    })

    // 同时显示简短消息
    if (failCount === 0) {
      ElMessage.success(`所有引擎连接测试成功 (${successCount}/${successCount + failCount})`)
    } else {
      ElMessage.warning(`部分引擎连接测试失败 (${successCount}成功, ${failCount}失败)`)
    }
  } catch (error: any) {
    loading.close()
    ElMessage.error('测试连接失败：' + (error.response?.data?.error || error.message))
  }
}

// 获取引擎显示名称
const getEngineDisplayName = (engine: string) => {
  const names: Record<string, string> = {
    fofa: 'FOFA',
    hunter: 'Hunter',
    shodan: 'Shodan',
  }
  return names[engine] || engine
}

const loadDetectorConfig = () => {
  // 重新从已加载的配置中恢复
  detectorConfig.concurrency = 10
  detectorConfig.timeout = 30
  detectorConfig.honeypot_enabled = true
  detectorConfig.honeypot_threshold = 5
  detectorConfig.health_check_interval = 5
  detectorConfig.model_sync_interval = 10
  ElMessage.info('已重置为默认配置，请点击保存以应用')
}

const saveDetectorConfig = async () => {
  saving.value = true
  try {
    await api.put('/proxy/config', { detector: detectorConfig })
    ElMessage.success('检测器配置已保存')
  } catch (error: any) {
    ElMessage.error('保存失败：' + error.message)
  } finally {
    saving.value = false
  }
}

// 修改密码
const changePassword = async () => {
  if (!passwordFormRef.value) return

  await passwordFormRef.value.validate(async (valid) => {
    if (!valid) return

    saving.value = true
    try {
      await api.post('/auth/change-password', {
        old_password: passwordForm.old_password,
        new_password: passwordForm.new_password,
      })
      ElMessage.success('密码修改成功，请重新登录')
      resetPasswordForm()
      // 清空 token 并跳转到登录页
      setTimeout(() => {
        localStorage.removeItem('access_token')
        localStorage.removeItem('user_info')
        window.location.href = '/admin/login'
      }, 1500)
    } catch (error: any) {
      ElMessage.error('修改失败：' + (error.response?.data?.error || error.message))
    } finally {
      saving.value = false
    }
  })
}

const resetPasswordForm = () => {
  passwordForm.old_password = ''
  passwordForm.new_password = ''
  passwordForm.confirm_password = ''
  passwordFormRef.value?.clearValidate()
}

// 获取当前用户信息
const loadCurrentUser = async () => {
  try {
    const res = await api.get('/auth/me')
    currentUser.value = res.data
  } catch (error: any) {
    console.error('加载用户信息失败:', error)
  }
}

// 加载用户列表
const loadUsers = async () => {
  if (!isAdmin.value) return

  loadingUsers.value = true
  try {
    const res = await api.get('/users')
    users.value = res.data.data || []
  } catch (error: any) {
    ElMessage.error('加载用户列表失败：' + error.message)
  } finally {
    loadingUsers.value = false
  }
}

// 显示创建用户对话框
const showCreateUserDialog = () => {
  Object.assign(userForm, {
    id: '',
    username: '',
    nickname: '',
    email: '',
    role: 'user',
    status: 'active',
    password: '',
  })
  userDialogVisible.value = true
}

// 编辑用户
const editUser = (user: any) => {
  Object.assign(userForm, {
    id: user.id,
    username: user.username,
    nickname: user.nickname || '',
    email: user.email || '',
    role: user.role,
    status: user.status,
    password: '',
  })
  userDialogVisible.value = true
}

// 保存用户
const saveUser = async () => {
  if (!userDialogRef.value) return

  await userDialogRef.value.validate(async (valid) => {
    if (!valid) return

    saving.value = true
    try {
      const data = {
        username: userForm.username,
        nickname: userForm.nickname,
        email: userForm.email,
        role: userForm.role,
        status: userForm.status,
      }

      if (userForm.id) {
        // 更新用户
        await api.put(`/users/${userForm.id}`, data)
        ElMessage.success('用户更新成功')
      } else {
        // 创建用户
        await api.post('/users', {
          ...data,
          password: userForm.password,
        })
        ElMessage.success('用户创建成功')
      }

      userDialogVisible.value = false
      loadUsers()
    } catch (error: any) {
      ElMessage.error('操作失败：' + (error.response?.data?.error || error.message))
    } finally {
      saving.value = false
    }
  })
}

// 重置用户密码
const resetUserPassword = async (user: any) => {
  try {
    await ElMessageBox.prompt('请输入新密码（至少8位，包含大小写字母和数字）', '重置密码', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      inputPattern: /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,}$/,
      inputErrorMessage: '密码格式不正确',
      inputType: 'password',
    })

    const { value } = await ElMessageBox.prompt('请再次输入新密码', '确认密码', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      inputType: 'password',
    })

    saving.value = true
    try {
      await api.post(`/users/${user.id}/reset-password`, { password: value })
      ElMessage.success('密码重置成功')
    } catch (error: any) {
      ElMessage.error('重置失败：' + (error.response?.data?.error || error.message))
    } finally {
      saving.value = false
    }
  } catch {
    // 用户取消
  }
}

// 删除用户
const deleteUser = async (userId: string) => {
  saving.value = true
  try {
    await api.delete(`/users/${userId}`)
    ElMessage.success('用户删除成功')
    loadUsers()
  } catch (error: any) {
    ElMessage.error('删除失败：' + (error.response?.data?.error || error.message))
  } finally {
    saving.value = false
  }
}

// 获取状态类型
const getStatusType = (status: string) => {
  const types: Record<string, string> = {
    active: 'success',
    disabled: 'info',
    locked: 'danger',
    require_password_change: 'warning',
  }
  return types[status] || 'info'
}

// 获取状态文本
const getStatusText = (status: string) => {
  const texts: Record<string, string> = {
    active: '正常',
    disabled: '已禁用',
    locked: '已锁定',
    require_password_change: '需修改密码',
  }
  return texts[status] || status
}

onMounted(() => {
  loadProxyConfig()
  loadCurrentUser()
})

// 监听标签切换，加载用户列表
watch(activeTab, (newTab) => {
  if (newTab === 'users' && isAdmin.value) {
    loadUsers()
  }
})
</script>

<style scoped>
.settings-page {
  height: 100%;
  padding-bottom: 20px;
}

/* 页面头部 */
.page-header {
  margin-bottom: 24px;
}

.header-content {
  display: flex;
  align-items: center;
  gap: 20px;
}

.header-icon {
  width: 64px;
  height: 64px;
  background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  box-shadow: 0 8px 24px rgba(79, 70, 229, 0.3);
}

.page-title {
  font-size: 24px;
  font-weight: 700;
  color: #1e293b;
  margin: 0 0 4px 0;
}

.page-desc {
  font-size: 14px;
  color: #64748b;
  margin: 0;
}

/* 设置卡片 */
.settings-card {
  border-radius: 12px;
  min-height: 500px;
}

.settings-tabs {
  padding: 0 20px;
}

.tab-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}

/* 设置部分 */
.setting-section {
  padding: 24px 0;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 32px;
  padding-bottom: 20px;
  border-bottom: 1px solid #f1f5f9;
}

.section-icon {
  width: 56px;
  height: 56px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.section-icon.primary {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.section-icon.success {
  background: linear-gradient(135deg, #4ade80 0%, #22c55e 100%);
}

.section-icon.warning {
  background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%);
}

.section-icon.info {
  background: linear-gradient(135deg, #38bdf8 0%, #0ea5e9 100%);
}

.section-title {
  font-size: 18px;
  font-weight: 600;
  color: #1e293b;
  margin: 0 0 4px 0;
}

.section-desc {
  font-size: 13px;
  color: #64748b;
  margin: 0;
}

/* 表单 */
.settings-form {
  max-width: 700px;
}

.form-group {
  margin-bottom: 32px;
  padding: 24px;
  background: #f8fafc;
  border-radius: 12px;
}

.form-group-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: #334155;
  margin-bottom: 20px;
}

.form-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
  padding-top: 24px;
  border-top: 1px solid #f1f5f9;
}

/* 引擎卡片 */
.engine-card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  margin-bottom: 16px;
  overflow: hidden;
  transition: all 0.3s ease;
}

.engine-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.engine-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px;
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
}

.engine-info {
  display: flex;
  align-items: center;
  gap: 16px;
}

.engine-emoji {
  font-size: 32px;
}

.engine-name {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
  margin: 0 0 2px 0;
}

.engine-desc {
  font-size: 12px;
  color: #94a3b8;
  margin: 0;
}

.engine-body {
  padding: 20px;
  background: #fff;
}

/* 关于部分 */
.about-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 40px 20px;
  text-align: center;
}

.about-logo {
  margin-bottom: 24px;
}

.logo-inner {
  width: 120px;
  height: 120px;
  background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
  border-radius: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  box-shadow: 0 12px 40px rgba(79, 70, 229, 0.3);
}

.about-title {
  font-size: 28px;
  font-weight: 700;
  color: #1e293b;
  margin: 0 0 8px 0;
}

.about-version {
  font-size: 14px;
  color: #64748b;
  margin: 0 0 16px 0;
}

.about-desc {
  font-size: 14px;
  color: #475569;
  max-width: 400px;
  margin: 0 0 32px 0;
  line-height: 1.6;
}

.about-info {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 32px;
}

.info-item {
  display: flex;
  justify-content: center;
  gap: 8px;
}

.info-label {
  font-weight: 600;
  color: #64748b;
}

.info-value {
  color: #1e293b;
}

.about-actions {
  display: flex;
  gap: 12px;
}

/* 响应式 */
@media (max-width: 768px) {
  .header-content {
    flex-direction: column;
    text-align: center;
  }

  .settings-form,
  .form-actions {
    max-width: 100%;
  }

  .form-actions {
    flex-direction: column;
  }

  .form-actions .el-button {
    width: 100%;
  }
}
</style>
