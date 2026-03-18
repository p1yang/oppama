package proxy

import (
	"testing"
	"time"

	"oppama/internal/storage"
)

func TestSelectBestService(t *testing.T) {
	p := &ProxyService{
		modelRoundRobin: make(map[string]int),
	}

	testModel := "llama2"

	t.Run("优先级排序_在线优于离线", func(t *testing.T) {
		services := []struct {
			service *storage.OllamaService
			model   string
		}{
			{
				service: &storage.OllamaService{
					ID:           "offline-service",
					URL:          "http://offline:11434",
					Status:       storage.StatusOffline,
					ResponseTime: 0,
				},
				model: "llama2",
			},
			{
				service: &storage.OllamaService{
					ID:           "online-service",
					URL:          "http://online:11434",
					Status:       storage.StatusOnline,
					ResponseTime: 100 * time.Millisecond,
				},
				model: "llama2",
			},
		}

		selected, modelName := p.selectBestService(services, testModel)
		if selected == nil {
			t.Fatal("未选择到服务")
		}

		if selected.ID != "online-service" {
			t.Errorf("期望选择在线服务，实际：%s", selected.ID)
		}

		if modelName != "llama2" {
			t.Errorf("期望模型名为 llama2，实际：%s", modelName)
		}
	})

	t.Run("响应时间优化_快速优于慢速", func(t *testing.T) {
		services := []struct {
			service *storage.OllamaService
			model   string
		}{
			{
				service: &storage.OllamaService{
					ID:           "slow-service",
					URL:          "http://slow:11434",
					Status:       storage.StatusOnline,
					ResponseTime: 500 * time.Millisecond,
				},
				model: "llama2",
			},
			{
				service: &storage.OllamaService{
					ID:           "fast-service",
					URL:          "http://fast:11434",
					Status:       storage.StatusOnline,
					ResponseTime: 50 * time.Millisecond,
				},
				model: "llama2",
			},
		}

		selected, _ := p.selectBestService(services, testModel)
		if selected == nil {
			t.Fatal("未选择到服务")
		}

		if selected.ID != "fast-service" {
			t.Errorf("期望选择快速服务，实际：%s", selected.ID)
		}
	})

	t.Run("轮询负载均衡", func(t *testing.T) {
		// 重置轮询索引
		p.modelRoundRobin[testModel] = 0

		services := []struct {
			service *storage.OllamaService
			model   string
		}{
			{
				service: &storage.OllamaService{
					ID:           "service-1",
					URL:          "http://service1:11434",
					Status:       storage.StatusOnline,
					ResponseTime: 100 * time.Millisecond,
				},
				model: "llama2",
			},
			{
				service: &storage.OllamaService{
					ID:           "service-2",
					URL:          "http://service2:11434",
					Status:       storage.StatusOnline,
					ResponseTime: 100 * time.Millisecond,
				},
				model: "llama2",
			},
			{
				service: &storage.OllamaService{
					ID:           "service-3",
					URL:          "http://service3:11434",
					Status:       storage.StatusOnline,
					ResponseTime: 100 * time.Millisecond,
				},
				model: "llama2",
			},
		}

		// 第一次选择
		selected1, _ := p.selectBestService(services, testModel)
		if selected1.ID != "service-1" {
			t.Errorf("第一次期望选择 service-1，实际：%s", selected1.ID)
		}

		// 第二次选择（应该轮询到下一个）
		selected2, _ := p.selectBestService(services, testModel)
		if selected2.ID != "service-2" {
			t.Errorf("第二次期望选择 service-2，实际：%s", selected2.ID)
		}

		// 第三次选择
		selected3, _ := p.selectBestService(services, testModel)
		if selected3.ID != "service-3" {
			t.Errorf("第三次期望选择 service-3，实际：%s", selected3.ID)
		}

		// 第四次选择（应该回到第一个）
		selected4, _ := p.selectBestService(services, testModel)
		if selected4.ID != "service-1" {
			t.Errorf("第四次期望选择 service-1，实际：%s", selected4.ID)
		}
	})

	t.Run("优先在线服务即使响应时间较慢", func(t *testing.T) {
		services := []struct {
			service *storage.OllamaService
			model   string
		}{
			{
				service: &storage.OllamaService{
					ID:           "unknown-fast",
					URL:          "http://unknown-fast:11434",
					Status:       storage.StatusUnknown,
					ResponseTime: 10 * time.Millisecond, // 非常快
				},
				model: "llama2",
			},
			{
				service: &storage.OllamaService{
					ID:           "online-slow",
					URL:          "http://online-slow:11434",
					Status:       storage.StatusOnline,
					ResponseTime: 300 * time.Millisecond, // 较慢
				},
				model: "llama2",
			},
		}

		selected, _ := p.selectBestService(services, testModel)
		if selected == nil {
			t.Fatal("未选择到服务")
		}

		// 应该选择在线服务，即使它比较慢
		if selected.ID != "online-slow" {
			t.Errorf("期望选择在线但较慢的服务，实际：%s", selected.ID)
		}
	})

	t.Run("只有离线服务时的选择", func(t *testing.T) {
		services := []struct {
			service *storage.OllamaService
			model   string
		}{
			{
				service: &storage.OllamaService{
					ID:           "offline-1",
					URL:          "http://offline1:11434",
					Status:       storage.StatusOffline,
					ResponseTime: 0,
				},
				model: "llama2",
			},
			{
				service: &storage.OllamaService{
					ID:           "offline-2",
					URL:          "http://offline2:11434",
					Status:       storage.StatusOffline,
					ResponseTime: 0,
				},
				model: "llama2",
			},
		}

		selected, _ := p.selectBestService(services, testModel)
		if selected == nil {
			t.Fatal("未选择到服务")
		}

		// 在都离线的情况下，按顺序选择
		t.Logf("选择了离线服务：%s", selected.ID)
	})
}

func TestModelRoundRobin(t *testing.T) {
	p := &ProxyService{
		modelRoundRobin: make(map[string]int),
	}

	model1 := "llama2"
	model2 := "qwen2"

	// 准备两个服务的列表
	services := []struct {
		service *storage.OllamaService
		model   string
	}{
		{
			service: &storage.OllamaService{
				ID:     "service-a",
				URL:    "http://service-a:11434",
				Status: storage.StatusOnline,
			},
			model: "common-model",
		},
		{
			service: &storage.OllamaService{
				ID:     "service-b",
				URL:    "http://service-b:11434",
				Status: storage.StatusOnline,
			},
			model: "common-model",
		},
	}

	// 不同模型的轮询应该是独立的
	selected1, _ := p.selectBestService(services, model1)
	selected2, _ := p.selectBestService(services, model2)

	// 两个模型都应该从索引 0 开始
	if selected1.ID != "service-a" {
		t.Errorf("模型 1 期望选择 service-a，实际：%s", selected1.ID)
	}

	if selected2.ID != "service-a" {
		t.Errorf("模型 2 期望选择 service-a，实际：%s", selected2.ID)
	}

	t.Logf("模型 %s 的轮询索引：%d", model1, p.modelRoundRobin[model1])
	t.Logf("模型 %s 的轮询索引：%d", model2, p.modelRoundRobin[model2])
}

func TestSessionBinding(t *testing.T) {
	p := &ProxyService{
		modelRoundRobin: make(map[string]int),
		sessionBindings: make(map[string]*SessionBinding),
		sessionTTL:      5 * time.Minute,
	}

	// 准备测试服务
	service1 := &storage.OllamaService{
		ID:           "service-1",
		URL:          "http://service1:11434",
		Status:       storage.StatusOnline,
		ResponseTime: 100 * time.Millisecond,
		Models: []storage.ModelInfo{
			{Name: "llama2", ServiceID: "service-1"},
		},
	}

	service2 := &storage.OllamaService{
		ID:           "service-2",
		URL:          "http://service2:11434",
		Status:       storage.StatusOnline,
		ResponseTime: 150 * time.Millisecond,
		Models: []storage.ModelInfo{
			{Name: "llama2", ServiceID: "service-2"},
		},
	}

	p.currentServices = []*storage.OllamaService{service1, service2}

	t.Run("首次请求创建会话绑定", func(t *testing.T) {
		sessionID := "test-session-1"

		// 第一次请求，应该选择 service-1（响应时间更快）
		svc, model, err := p.selectServiceAndModel("llama2", sessionID)
		if err != nil {
			t.Fatalf("选择服务失败：%v", err)
		}

		if svc.ID != "service-1" {
			t.Errorf("期望选择 service-1，实际：%s", svc.ID)
		}

		if model != "llama2" {
			t.Errorf("期望模型 llama2，实际：%s", model)
		}

		// 检查是否创建了会话绑定
		binding, exists := p.GetSessionBinding(sessionID)
		if !exists {
			t.Fatal("未创建会话绑定")
		}

		if binding.ServiceID != "service-1" {
			t.Errorf("会话绑定的服务 ID 错误：%s", binding.ServiceID)
		}

		if binding.ModelName != "llama2" {
			t.Errorf("会话绑定的模型名称错误：%s", binding.ModelName)
		}

		t.Logf("会话绑定：session=%s, service=%s, model=%s",
			sessionID, binding.ServiceID, binding.ModelName)
	})

	t.Run("后续请求使用相同服务", func(t *testing.T) {
		sessionID := "test-session-2"

		// 第一次请求
		svc1, _, err := p.selectServiceAndModel("llama2", sessionID)
		if err != nil {
			t.Fatalf("第一次选择失败：%v", err)
		}

		// 模拟多次请求
		for i := 0; i < 5; i++ {
			svc2, _, err := p.selectServiceAndModel("llama2", sessionID)
			if err != nil {
				t.Fatalf("第 %d 次选择失败：%v", i+2, err)
			}

			if svc1.ID != svc2.ID {
				t.Errorf("第 %d 次请求服务不一致：%s != %s", i+2, svc1.ID, svc2.ID)
			}
		}

		binding, _ := p.GetSessionBinding(sessionID)
		t.Logf("会话请求次数：%d", binding.RequestCount)
		if binding.RequestCount != 6 {
			t.Errorf("期望请求次数为 6，实际：%d", binding.RequestCount)
		}
	})

	t.Run("不同会话独立绑定", func(t *testing.T) {
		session1 := "session-A"
		session2 := "session-B"

		// 两个不同的会话
		svc1, _, _ := p.selectServiceAndModel("llama2", session1)
		svc2, _, _ := p.selectServiceAndModel("llama2", session2)

		// 由于轮询，可能会分配到不同的服务
		binding1, _ := p.GetSessionBinding(session1)
		binding2, _ := p.GetSessionBinding(session2)

		t.Logf("会话 A: service=%s", binding1.ServiceID)
		t.Logf("会话 B: service=%s", binding2.ServiceID)

		// 验证每个会话都正确绑定到各自的服务
		if binding1.ServiceID != svc1.ID {
			t.Errorf("会话 A 绑定错误")
		}
		if binding2.ServiceID != svc2.ID {
			t.Errorf("会话 B 绑定错误")
		}
	})

	t.Run("会话过期清理", func(t *testing.T) {
		sessionID := "test-expired-session"

		// 创建会话绑定
		p.selectServiceAndModel("llama2", sessionID)

		// 验证绑定存在
		if _, exists := p.GetSessionBinding(sessionID); !exists {
			t.Fatal("未创建会话绑定")
		}

		// 手动设置过期时间（模拟过期）
		p.sessionMu.Lock()
		if binding, ok := p.sessionBindings[sessionID]; ok {
			binding.LastUsedAt = time.Now().Add(-10 * time.Minute) // 10 分钟前
		}
		p.sessionMu.Unlock()

		// 验证已过期
		if _, exists := p.GetSessionBinding(sessionID); exists {
			t.Error("会话已过期但仍存在")
		}

		// 执行清理
		p.CleanupExpiredSessions()

		// 验证已删除
		if _, exists := p.GetSessionBinding(sessionID); exists {
			t.Error("清理后会话仍存在")
		}

		t.Log("会话过期清理成功")
	})

	t.Run("无 session_id 时使用轮询", func(t *testing.T) {
		// 重置轮询索引
		p.modelRoundRobin["llama2"] = 0

		// 多次请求不使用 session_id
		var services []string
		for i := 0; i < 4; i++ {
			svc, _, _ := p.selectServiceAndModel("llama2", "")
			services = append(services, svc.ID)
		}

		// 验证轮询效果
		t.Logf("轮询结果：%v", services)

		// 应该交替使用 service-1 和 service-2
		hasService1 := false
		hasService2 := false
		for _, sid := range services {
			if sid == "service-1" {
				hasService1 = true
			}
			if sid == "service-2" {
				hasService2 = true
			}
		}

		if !hasService1 || !hasService2 {
			t.Error("轮询未正常工作")
		}
	})
}

func TestModelMatches(t *testing.T) {
	p := &ProxyService{}

	tests := []struct {
		name     string
		requested string
		available string
		want     bool
	}{
		// 精确匹配
		{
			name:     "精确匹配_完全相同",
			requested: "llama2:latest",
			available: "llama2:latest",
			want:     true,
		},

		// 基础名称匹配
		{
			name:     "基础名称匹配_请求无标签",
			requested: "llama2",
			available: "llama2:latest",
			want:     true,
		},
		{
			name:     "基础名称匹配_请求无标签匹配任何标签",
			requested: "qwen2",
			available: "qwen2:72b",
			want:     true,
		},
		{
			name:     "基础名称不同_不匹配",
			requested: "llama2",
			available: "qwen2:latest",
			want:     false,
		},

		// 标签精确匹配
		{
			name:     "标签精确匹配",
			requested: "llama2:latest",
			available: "llama2:latest",
			want:     true,
		},

		// 量化版本匹配
		{
			name:     "量化版本匹配_latest到latest-q4",
			requested: "llama2:latest",
			available: "llama2:latest-q4_K_M",
			want:     true,
		},
		{
			name:     "量化版本匹配_7b到7b-q8",
			requested: "qwen2:7b",
			available: "qwen2:7b-q8_0",
			want:     true,
		},

		// 防止错误匹配（重要）
		{
			name:     "防止错误匹配_7b不匹配70b",
			requested: "qwen2:7b",
			available: "qwen2:70b",
			want:     false,
		},
		{
			name:     "防止错误匹配_70b不匹配7b",
			requested: "qwen2:70b",
			available: "qwen2:7b",
			want:     false,
		},
		{
			name:     "防止错误匹配_7b不匹配70b量化版",
			requested: "qwen2:7b",
			available: "qwen2:70b-q4_K_M",
			want:     false,
		},
		{
			name:     "防止错误匹配_13b不匹配70b",
			requested: "qwen2:13b",
			available: "qwen2:70b",
			want:     false,
		},

		// 边界情况
		{
			name:     "边界_空字符串",
			requested: "",
			available: "llama2:latest",
			want:     false,
		},
		{
			name:     "边界_只有冒号",
			requested: "llama2:",
			available: "llama2:latest",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.modelMatches(tt.requested, tt.available)
			if got != tt.want {
				t.Errorf("modelMatches(%q, %q) = %v, want %v",
					tt.requested, tt.available, got, tt.want)
			}
		})
	}
}
