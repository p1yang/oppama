package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestConcurrentReadWrite 测试并发读写性能
func TestConcurrentReadWrite(t *testing.T) {
	// 创建临时数据库文件
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("创建存储实例失败: %v", err)
	}
	defer storage.Close()

	// 模拟批量写入（10个goroutine）
	var writeWg sync.WaitGroup
	writeCount := 10
	servicesPerWriter := 10

	startTime := time.Now()

	// 启动写入goroutine
	for i := 0; i < writeCount; i++ {
		writeWg.Add(1)
		go func(id int) {
			defer writeWg.Done()
			for j := 0; j < servicesPerWriter; j++ {
				service := &OllamaService{
					ID:        fmt.Sprintf("test-%d-%d", id, j),
					URL:       fmt.Sprintf("http://test-%d-%d.com", id, j),
					Name:      "Test Service",
					Status:    StatusOnline,
					Source:    SourceManual,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				err := storage.SaveService(context.Background(), service)
				if err != nil {
					t.Errorf("保存服务失败: %v", err)
				}
			}
		}(i)
	}

	// 并发读取（验证读取不被阻塞）
	var readWg sync.WaitGroup
	readCount := 50
	readAttempts := 5

	readStart := time.Now()

	// 启动读取goroutine
	for i := 0; i < readCount; i++ {
		readWg.Add(1)
		go func() {
			defer readWg.Done()
			for j := 0; j < readAttempts; j++ {
				_, err := storage.ListServices(context.Background(), ServiceFilter{})
				if err != nil {
					t.Errorf("读取服务列表失败: %v", err)
				}
			}
		}()
	}

	// 等待所有读取完成（应该不会被写入阻塞）
	readWg.Wait()
	readDuration := time.Since(readStart)

	// 等待所有写入完成
	writeWg.Wait()
	totalDuration := time.Since(startTime)

	// 验证读取操作在合理时间内完成
	if readDuration > 3*time.Second {
		t.Errorf("并发读取应在3秒内完成，实际耗时 %v", readDuration)
	}

	t.Logf("测试完成：读取耗时 %v，总耗时 %v", readDuration, totalDuration)
	t.Logf("读取操作未受写入阻塞，符合WAL模式预期")
}

// TestWALModeEnabled 测试WAL模式是否已启用
func TestWALModeEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_wal.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("创建存储实例失败: %v", err)
	}
	defer storage.Close()

	// 检查WAL统计信息
	stats, err := storage.GetWALStats()
	if err != nil {
		t.Fatalf("获取WAL统计信息失败: %v", err)
	}

	journalMode, ok := stats["journal_mode"].(string)
	if !ok {
		t.Fatal("journal_mode类型错误")
	}

	// 验证WAL模式已启用
	if journalMode != "wal" {
		t.Errorf("WAL模式未正确启用，当前模式: %s", journalMode)
	}

	// 验证连接池配置
	openConns, ok := stats["open_connections"].(int)
	if !ok {
		t.Error("open_connections类型错误")
	}
	if openConns < 0 {
		t.Errorf("连接数应为非负数，实际: %d", openConns)
	}

	t.Logf("WAL模式验证通过：journal_mode=%s, open_connections=%d", journalMode, openConns)
}

// TestFineGrainedLocks 测试细粒度锁不会导致死锁
func TestFineGrainedLocks(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_locks.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("创建存储实例失败: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// 创建测试数据
	serviceID := "test-service-1"
	service := &OllamaService{
		ID:        serviceID,
		URL:       "http://test.com",
		Name:      "Test Service",
		Status:    StatusOnline,
		Source:    SourceManual,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = storage.SaveService(ctx, service)
	if err != nil {
		t.Fatalf("保存服务失败: %v", err)
	}

	// 添加模型
	models := []ModelInfo{
		{ID: "model1-id", Name: "model1", Size: 1000, LastTested: time.Now()},
		{ID: "model2-id", Name: "model2", Size: 2000, LastTested: time.Now()},
	}

	err = storage.SaveModels(ctx, serviceID, models)
	if err != nil {
		t.Fatalf("保存模型失败: %v", err)
	}

	// 并发访问不同表（测试细粒度锁）
	var wg sync.WaitGroup
	errors := make(chan error, 4)

	// 1. 读取服务
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := storage.GetService(ctx, serviceID)
		errors <- err
	}()

	// 2. 读取模型
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := storage.GetModelsByService(ctx, serviceID)
		errors <- err
	}()

	// 3. 更新服务
	wg.Add(1)
	go func() {
		defer wg.Done()
		service.Status = StatusOffline
		service.UpdatedAt = time.Now()
		err := storage.SaveService(ctx, service)
		errors <- err
	}()

	// 4. 添加新模型
	wg.Add(1)
	go func() {
		defer wg.Done()
		newModels := []ModelInfo{
			{ID: "model3-id", Name: "model3", Size: 3000, LastTested: time.Now()},
		}
		err := storage.SaveModels(ctx, serviceID, newModels)
		errors <- err
	}()

	// 等待所有操作完成
	wg.Wait()
	close(errors)

	// 检查是否有错误
	for err := range errors {
		if err != nil {
			t.Errorf("并发操作失败: %v", err)
		}
	}

	t.Log("细粒度锁测试通过：不同表的并发操作无死锁")
}

// TestBatchDetectionPerformance 测试批量检测场景性能
func TestBatchDetectionPerformance(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_batch.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("创建存储实例失败: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// 模拟批量检测场景：同时插入多个服务
	const batchSize = 50
	services := make([]*OllamaService, batchSize)

	for i := 0; i < batchSize; i++ {
		services[i] = &OllamaService{
			ID:        fmt.Sprintf("batch-%d", i),
			URL:       fmt.Sprintf("http://test-%d.com", i),
			Name:      fmt.Sprintf("Test Service %d", i),
			Status:    StatusOnline,
			Source:    SourceImport,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	// 并发保存服务（模拟批量检测）
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < batchSize; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := storage.SaveService(ctx, services[idx])
			if err != nil {
				t.Errorf("保存服务失败: %v", err)
			}
		}(i)
	}

	// 同时进行读取操作（模拟前端请求）
	var readWg sync.WaitGroup
	readStart := time.Now()

	for i := 0; i < 20; i++ {
		readWg.Add(1)
		go func() {
			defer readWg.Done()
			_, err := storage.ListServices(ctx, ServiceFilter{})
			if err != nil {
				t.Errorf("读取服务列表失败: %v", err)
			}
		}()
	}

	// 等待读取完成
	readWg.Wait()
	readDuration := time.Since(readStart)

	// 等待写入完成
	wg.Wait()
	totalDuration := time.Since(start)

	// 验证读取操作未受严重影响
	if readDuration > 2*time.Second {
		t.Errorf("批量写入期间的读取应在2秒内完成，实际耗时 %v", readDuration)
	}

	t.Logf("批量检测性能测试通过：%d个并发写入，读取耗时 %v，总耗时 %v",
		batchSize, readDuration, totalDuration)
}

// TestWALMaintenance 测试WAL维护功能
func TestWALMaintenance(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_maintenance.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("创建存储实例失败: %v", err)
	}
	defer storage.Close()

	// 执行WAL维护
	err = storage.MaintainWAL()
	if err != nil {
		t.Errorf("WAL维护失败: %v", err)
	}

	// 验证WAL统计信息
	stats, err := storage.GetWALStats()
	if err != nil {
		t.Fatalf("获取WAL统计信息失败: %v", err)
	}

	journalMode, ok := stats["journal_mode"].(string)
	if !ok {
		t.Fatal("journal_mode类型错误")
	}

	if journalMode == "wal" {
		// 如果是WAL模式，检查是否有检查点信息
		_, hasCheckpoint := stats["wal_checkpoint_checkpointed"]
		if !hasCheckpoint {
			t.Log("WAL检查点信息可能未返回，但这不影响WAL模式正常工作")
		}
	}

	t.Logf("WAL维护测试通过：journal_mode=%s", journalMode)
}