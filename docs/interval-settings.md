# 定时任务间隔设置功能

## 功能概述

Oppama 现在支持通过前端界面自定义设置后台定时任务的时间间隔，包括：

1. **健康检查间隔** (`health_check_interval`) - 定期检查服务存活状态的时间间隔
2. **模型同步间隔** (`model_sync_interval`) - 定期同步在线服务模型列表的时间间隔

## 默认值

- 健康检查间隔：**5 分钟**
- 模型同步间隔：**10 分钟**

## 使用方法

### 通过前端界面设置

1. 登录到 Oppama 管理后台
2. 进入「系统设置」页面
3. 切换到「检测器配置」标签页
4. 在「定时任务间隔」部分设置：
   - 健康检查间隔（1-60 分钟）
   - 模型同步间隔（1-120 分钟）
5. 点击「保存配置」按钮

### 通过 API 设置

```bash
# 获取当前配置
curl -X GET "http://localhost:8080/v1/api/proxy/config" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 更新时间间隔
curl -X PUT "http://localhost:8080/v1/api/proxy/config" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "detector": {
      "health_check_interval": 3,
      "model_sync_interval": 7
    }
  }'
```

## 配置说明

### 健康检查间隔

- **范围**：1-60 分钟
- **功能**：定期对所有已添加的 Ollama 服务进行健康检查
- **检查内容**：
  - 服务是否在线
  - 响应时间
  - 版本号
  - 蜜罐识别
- **影响**：较短的间隔可以更快发现服务故障，但会增加系统负载

### 模型同步间隔

- **范围**：1-120 分钟
- **功能**：定期同步在线服务的模型列表
- **同步内容**：
  - 可用模型名称
  - 模型大小
  - 模型家族
  - 量化级别等
- **影响**：较短的间隔可以保持模型信息最新，但会增加 API 调用频率

## 注意事项

1. **性能考虑**：
   - 如果服务数量较多（>100 个），建议适当增加时间间隔
   - 并发数设置会影响检测速度，默认 10 个并发

2. **资源占用**：
   - 频繁的健康检查会占用网络带宽和 CPU 资源
   - 模型同步会消耗更多的内存存储模型信息

3. **推荐配置**：
   - 小型环境（<20 个服务）：健康检查 3-5 分钟，模型同步 10-15 分钟
   - 中型环境（20-50 个服务）：健康检查 5-10 分钟，模型同步 15-30 分钟
   - 大型环境（>50 个服务）：健康检查 10-15 分钟，模型同步 30-60 分钟

4. **实时生效**：
   - 修改时间间隔后，调度器会立即重启对应的定时任务
   - 无需重启服务器即可应用新配置

## 技术实现

### 后端

- `internal/scheduler/scheduler.go` - 定时任务调度器
  - `SetHealthCheckInterval()` - 设置健康检查间隔
  - `SetModelSyncInterval()` - 设置模型同步间隔
  - `GetIntervals()` - 获取当前间隔设置

- `internal/api/server.go` - API 服务器
  - `SetSchedulerIntervals()` - 设置调度器间隔
  - `GetSchedulerIntervals()` - 获取调度器间隔

- `internal/api/v1/proxy.go` - 代理配置处理器
  - 在 `GetConfig()` 中返回当前间隔设置
  - 在 `UpdateConfig()` 中处理间隔更新

### 前端

- `web/src/views/Settings.vue` - 设置页面
  - 在「检测器配置」标签页添加了两个输入框
  - 使用 `el-input-number` 组件限制输入范围
  - 自动保存和加载配置

## 测试

运行测试脚本验证功能：

```bash
chmod +x test-intervals.sh
./test-intervals.sh
```

## 日志输出

当时间间隔更新时，会在服务器日志中看到类似输出：

```
[Scheduler] 健康检查间隔已更新：5m0s -> 3m0s
[Scheduler] 模型同步间隔已更新：10m0s -> 7m0s
[Server] 已更新 Scheduler 时间间隔：健康检查=3 分钟，模型同步=7 分钟
```

## 版本要求

- Oppama v1.0.0+
- 需要管理员权限才能修改配置
