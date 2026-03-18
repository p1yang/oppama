package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"oppama/internal/utils/logger"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage SQLite 存储实现
type SQLiteStorage struct {
	db *sql.DB
	// 细粒度表级锁替代全局锁
	serviceMu sync.RWMutex
	modelMu   sync.RWMutex
	taskMu    sync.RWMutex
	userMu    sync.RWMutex
	tokenMu   sync.RWMutex
}

// NewSQLiteStorage 创建 SQLite 存储实例
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	return NewSQLiteStorageWithPool(dbPath, 25, 5, 30*time.Minute, 5*time.Minute)
}

// NewSQLiteStorageWithPool 创建带自定义连接池配置的 SQLite 存储实例
func NewSQLiteStorageWithPool(dbPath string, maxOpenConns, maxIdleConns int, connMaxLifetime, connMaxIdleTime time.Duration) (*SQLiteStorage, error) {
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败：%w", err)
	}

	// 添加WAL模式连接参数
	dsn := dbPath + "?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败：%w", err)
	}

	// 验证WAL模式已启用
	var journalMode string
	err = db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("查询journal_mode失败：%w", err)
	}
	if journalMode != "wal" {
		db.Close()
		// 回退到默认模式并记录警告
		db, err = sql.Open("sqlite3", dbPath)
		if err != nil {
			return nil, fmt.Errorf("打开数据库失败：%w", err)
		}
		logger.Storage().Warnf("警告：WAL模式启用失败，使用默认journal模式：%s", journalMode)
	} else {
		logger.Storage().Printf("SQLite WAL模式已启用")
	}

	// 设置连接池优化（使用配置的参数）
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	logger.Storage().Printf("数据库连接池配置：max_open=%d, max_idle=%d, max_lifetime=%v, max_idle=%v",
		maxOpenConns, maxIdleConns, connMaxLifetime, connMaxIdleTime)

	s := &SQLiteStorage{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化数据库 schema 失败：%w", err)
	}

	return s, nil
}

// initSchema 初始化数据库表结构
func (s *SQLiteStorage) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS services (
		id TEXT PRIMARY KEY,
		url TEXT UNIQUE NOT NULL,
		name TEXT,
		status TEXT DEFAULT 'unknown',
		version TEXT,
		response_time INTEGER DEFAULT 0,
		is_honeypot BOOLEAN DEFAULT FALSE,
		requires_auth BOOLEAN DEFAULT FALSE,
		country TEXT,
		region TEXT,
		city TEXT,
		isp TEXT,
		source TEXT DEFAULT 'manual',
		metadata TEXT,
		last_checked DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS models (
		id TEXT PRIMARY KEY,
		service_id TEXT NOT NULL,
		name TEXT NOT NULL,
		size INTEGER DEFAULT 0,
		digest TEXT,
		family TEXT,
		format TEXT,
		parameter_size TEXT,
		quantization_level TEXT,
		is_available BOOLEAN DEFAULT TRUE,
		last_tested DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		engines TEXT,
		query TEXT,
		max_results INTEGER DEFAULT 100,
		status TEXT DEFAULT 'pending',
		progress INTEGER DEFAULT 0,
		total INTEGER DEFAULT 0,
		found_count INTEGER DEFAULT 0,
		started_at DATETIME,
		completed_at DATETIME,
		results TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS universal_tasks (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		title TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		progress INTEGER DEFAULT 0,
		total INTEGER DEFAULT 0,
		result TEXT,
		error TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_services_status ON services(status);
	CREATE INDEX IF NOT EXISTS idx_services_source ON services(source);
	CREATE INDEX IF NOT EXISTS idx_models_service_id ON models(service_id);
	CREATE INDEX IF NOT EXISTS idx_models_family ON models(family);
	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_universal_tasks_status ON universal_tasks(status);
	CREATE INDEX IF NOT EXISTS idx_universal_tasks_type ON universal_tasks(type);

	-- 用户表
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		nickname TEXT,
		email TEXT,
		role TEXT DEFAULT 'user',
		status TEXT DEFAULT 'active',
		last_login_at DATETIME,
		failed_logins INTEGER DEFAULT 0,
		locked_until DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
	CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);

	-- Token 黑名单表
	CREATE TABLE IF NOT EXISTS token_blacklist (
		token TEXT PRIMARY KEY,
		expires_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_token_blacklist_expires ON token_blacklist(expires_at);

	-- 活动日志表
	CREATE TABLE IF NOT EXISTS activity_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		action TEXT NOT NULL,
		target TEXT,
		user_id TEXT,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_activity_logs_type ON activity_logs(type);
	CREATE INDEX IF NOT EXISTS idx_activity_logs_created_at ON activity_logs(created_at);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	// 更新现有用户表结构（添加新字段）
	return s.migrateUserSchema()
}

// migrateUserSchema 迁移用户表结构
func (s *SQLiteStorage) migrateUserSchema() error {
	// 检查并添加新字段
	migrations := []string{
		"ALTER TABLE users ADD COLUMN status TEXT DEFAULT 'active'",
		"ALTER TABLE users ADD COLUMN last_login_at DATETIME",
		"ALTER TABLE users ADD COLUMN failed_logins INTEGER DEFAULT 0",
		"ALTER TABLE users ADD COLUMN locked_until DATETIME",
	}

	for _, migration := range migrations {
		// 尝试执行，如果字段已存在会报错，忽略即可
		s.db.Exec(migration)
	}

	return nil
}

// SaveService 保存服务
func (s *SQLiteStorage) SaveService(ctx context.Context, svc *OllamaService) error {
	s.serviceMu.Lock()
	defer s.serviceMu.Unlock()

	now := time.Now()
	svc.UpdatedAt = now
	if svc.CreatedAt.IsZero() {
		svc.CreatedAt = now
	}

	var metadataJSON string
	if svc.Metadata != nil {
		data, _ := json.Marshal(svc.Metadata)
		metadataJSON = string(data)
	}

	query := `
	INSERT INTO services (
		id, url, name, status, version, response_time, is_honeypot,
		requires_auth, country, region, city, isp, source, metadata,
		last_checked, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		url = excluded.url,
		name = excluded.name,
		status = excluded.status,
		version = excluded.version,
		response_time = excluded.response_time,
		is_honeypot = excluded.is_honeypot,
		requires_auth = excluded.requires_auth,
		country = excluded.country,
		region = excluded.region,
		city = excluded.city,
		isp = excluded.isp,
		source = excluded.source,
		metadata = excluded.metadata,
		last_checked = excluded.last_checked,
		updated_at = excluded.updated_at
	`

	_, err := s.db.ExecContext(ctx, query,
		svc.ID, svc.URL, svc.Name, svc.Status, svc.Version,
		svc.ResponseTime.Milliseconds(), svc.IsHoneypot, svc.RequiresAuth,
		svc.Country, svc.Region, svc.City, svc.ISP, svc.Source,
		metadataJSON, svc.LastChecked, svc.CreatedAt, svc.UpdatedAt,
	)

	return err
}

// GetService 获取服务（优化版：解决 N+1 查询问题）
func (s *SQLiteStorage) GetService(ctx context.Context, id string) (*OllamaService, error) {
	s.serviceMu.RLock()
	defer s.serviceMu.RUnlock()

	query := `SELECT * FROM services WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	svc := &OllamaService{}
	var metadataJSON sql.NullString
	var responseTimeMs int64

	err := row.Scan(
		&svc.ID, &svc.URL, &svc.Name, &svc.Status, &svc.Version,
		&responseTimeMs, &svc.IsHoneypot, &svc.RequiresAuth,
		&svc.Country, &svc.Region, &svc.City, &svc.ISP, &svc.Source,
		&metadataJSON, &svc.LastChecked, &svc.CreatedAt, &svc.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	svc.ResponseTime = time.Duration(responseTimeMs) * time.Millisecond

	if metadataJSON.Valid && metadataJSON.String != "" {
		json.Unmarshal([]byte(metadataJSON.String), &svc.Metadata)
	}

	// 加载模型（使用批量查询方法）
	s.modelMu.RLock()
	models, err := s.getModelsByServiceIDs(ctx, []string{svc.ID})
	s.modelMu.RUnlock()

	if err != nil {
		return nil, err
	}
	svc.Models = models

	return svc, nil
}

// ListServices 列出服务（优化版：解决 N+1 查询问题）
func (s *SQLiteStorage) ListServices(ctx context.Context, filter ServiceFilter) ([]*OllamaService, error) {
	s.serviceMu.RLock()
	defer s.serviceMu.RUnlock()

	// 构建查询条件
	whereClause := "WHERE 1=1"
	args := []interface{}{}

	if filter.Status != nil {
		whereClause += ` AND s.status = ?`
		args = append(args, *filter.Status)
	}
	if filter.Source != nil {
		whereClause += ` AND s.source = ?`
		args = append(args, *filter.Source)
	}
	if filter.IsHoneypot != nil {
		whereClause += ` AND s.is_honeypot = ?`
		args = append(args, *filter.IsHoneypot)
	}
	if filter.Search != "" {
		whereClause += ` AND (s.name LIKE ? OR s.url LIKE ? OR s.version LIKE ?)`
		searchTerm := "%" + filter.Search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	// 先获取服务列表（带分页）
	query := `
		SELECT s.id, s.url, s.name, s.status, s.version, s.response_time,
		       s.is_honeypot, s.requires_auth, s.country, s.region, s.city,
		       s.isp, s.source, s.metadata, s.last_checked, s.created_at, s.updated_at
		FROM services s
		` + whereClause + `
		ORDER BY s.created_at DESC
	`

	if filter.PageSize > 0 {
		offset := 0
		if filter.Page > 0 {
			offset = (filter.Page - 1) * filter.PageSize
		}
		query += ` LIMIT ? OFFSET ?`
		args = append(args, filter.PageSize, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 收集所有服务 ID
	serviceIDs := make([]string, 0)
	servicesMap := make(map[string]*OllamaService)

	for rows.Next() {
		svc := &OllamaService{}
		var metadataJSON sql.NullString
		var responseTimeMs int64

		err := rows.Scan(
			&svc.ID, &svc.URL, &svc.Name, &svc.Status, &svc.Version,
			&responseTimeMs, &svc.IsHoneypot, &svc.RequiresAuth,
			&svc.Country, &svc.Region, &svc.City, &svc.ISP, &svc.Source,
			&metadataJSON, &svc.LastChecked, &svc.CreatedAt, &svc.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		svc.ResponseTime = time.Duration(responseTimeMs) * time.Millisecond

		if metadataJSON.Valid && metadataJSON.String != "" {
			json.Unmarshal([]byte(metadataJSON.String), &svc.Metadata)
		}

		svc.Models = make([]ModelInfo, 0) // 初始化模型列表
		serviceIDs = append(serviceIDs, svc.ID)
		servicesMap[svc.ID] = svc
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 一次性查询所有服务的模型
	if len(serviceIDs) > 0 {
		s.modelMu.RLock()
		models, err := s.getModelsByServiceIDs(ctx, serviceIDs)
		s.modelMu.RUnlock()

		if err != nil {
			return nil, err
		}

		// 将模型分配到对应的服务
		for _, model := range models {
			if svc, exists := servicesMap[model.ServiceID]; exists {
				svc.Models = append(svc.Models, model)
			}
		}
	}

	// 转换为切片并保持原始顺序
	services := make([]*OllamaService, 0, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		services = append(services, servicesMap[serviceID])
	}

	return services, nil
}

// DeleteService 删除服务
func (s *SQLiteStorage) DeleteService(ctx context.Context, id string) error {
	s.serviceMu.Lock()
	defer s.serviceMu.Unlock()

	_, err := s.db.ExecContext(ctx, `DELETE FROM services WHERE id = ?`, id)
	return err
}

// UpdateServiceStatus 更新服务状态
func (s *SQLiteStorage) UpdateServiceStatus(ctx context.Context, id string, status ServiceStatus) error {
	s.serviceMu.Lock()
	defer s.serviceMu.Unlock()

	_, err := s.db.ExecContext(ctx,
		`UPDATE services SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, id,
	)
	return err
}

// SaveModels 保存模型列表
func (s *SQLiteStorage) SaveModels(ctx context.Context, serviceID string, models []ModelInfo) error {
	s.modelMu.Lock()
	defer s.modelMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 先删除旧模型
	_, err = tx.ExecContext(ctx, `DELETE FROM models WHERE service_id = ?`, serviceID)
	if err != nil {
		return err
	}

	// 插入新模型
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO models (
			id, service_id, name, size, digest, family, format,
			parameter_size, quantization_level, is_available, last_tested
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, model := range models {
		_, err := stmt.ExecContext(ctx,
			model.ID, serviceID, model.Name, model.Size, model.Digest,
			model.Family, model.Format, model.ParameterSize, model.QuantLevel,
			model.IsAvailable, model.LastTested,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// getModelsByServiceIDs 批量获取多个服务的模型（内部方法，不对外暴露）
// 用于优化 ListServices 的 N+1 查询问题
func (s *SQLiteStorage) getModelsByServiceIDs(ctx context.Context, serviceIDs []string) ([]ModelInfo, error) {
	if len(serviceIDs) == 0 {
		return []ModelInfo{}, nil
	}

	// 构建 IN 查询
	placeholders := make([]string, len(serviceIDs))
	args := make([]interface{}, len(serviceIDs))
	for i, id := range serviceIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `
		SELECT id, service_id, name, size, digest, family, format,
		       parameter_size, quantization_level, is_available, last_tested
		FROM models
		WHERE service_id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY service_id, name
	`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := make([]ModelInfo, 0)
	for rows.Next() {
		var m ModelInfo
		var lastTested sql.NullTime

		err := rows.Scan(
			&m.ID, &m.ServiceID, &m.Name, &m.Size, &m.Digest, &m.Family,
			&m.Format, &m.ParameterSize, &m.QuantLevel, &m.IsAvailable, &lastTested,
		)
		if err != nil {
			return nil, err
		}

		if lastTested.Valid {
			m.LastTested = lastTested.Time
		}

		models = append(models, m)
	}

	return models, rows.Err()
}

// GetModelsByService 获取服务的模型列表
func (s *SQLiteStorage) GetModelsByService(ctx context.Context, serviceID string) ([]ModelInfo, error) {
	s.modelMu.RLock()
	defer s.modelMu.RUnlock()

	query := `SELECT * FROM models WHERE service_id = ? ORDER BY name`
	rows, err := s.db.QueryContext(ctx, query, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := make([]ModelInfo, 0)
	for rows.Next() {
		var m ModelInfo
		var lastTested sql.NullTime

		err := rows.Scan(
			&m.ID, &m.ServiceID, &m.Name, &m.Size, &m.Digest, &m.Family,
			&m.Format, &m.ParameterSize, &m.QuantLevel, &m.IsAvailable, &lastTested,
		)
		if err != nil {
			return nil, err
		}

		if lastTested.Valid {
			m.LastTested = lastTested.Time
		}

		models = append(models, m)
	}

	return models, rows.Err()
}

// ListModels 列出所有模型
func (s *SQLiteStorage) ListModels(ctx context.Context, filter ModelFilter) ([]ModelInfo, error) {
	s.modelMu.RLock()
	defer s.modelMu.RUnlock()

	query := `SELECT m.* FROM models m WHERE 1=1`
	args := []interface{}{}

	if filter.Family != "" {
		query += ` AND m.family = ?`
		args = append(args, filter.Family)
	}
	if filter.MinSize > 0 {
		query += ` AND m.size >= ?`
		args = append(args, filter.MinSize)
	}
	if filter.AvailableOnly {
		query += ` AND m.is_available = 1`
	}
	if filter.ServiceID != "" {
		query += ` AND m.service_id = ?`
		args = append(args, filter.ServiceID)
	}

	query += ` ORDER BY m.name`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := make([]ModelInfo, 0)
	for rows.Next() {
		var m ModelInfo
		var lastTested sql.NullTime

		err := rows.Scan(
			&m.ID, &m.ServiceID, &m.Name, &m.Size, &m.Digest, &m.Family,
			&m.Format, &m.ParameterSize, &m.QuantLevel, &m.IsAvailable, &lastTested,
		)
		if err != nil {
			return nil, err
		}

		if lastTested.Valid {
			m.LastTested = lastTested.Time
		}

		models = append(models, m)
	}

	return models, rows.Err()
}

// SaveTask 保存任务
func (s *SQLiteStorage) SaveTask(ctx context.Context, task *DiscoveryTask) error {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()

	enginesJSON, _ := json.Marshal(task.Engines)
	resultsJSON, _ := json.Marshal(task.Results)

	query := `
	INSERT INTO tasks (
		id, engines, query, max_results, status, progress, total,
		found_count, started_at, completed_at, results
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		engines = excluded.engines,
		query = excluded.query,
		max_results = excluded.max_results,
		status = excluded.status,
		progress = excluded.progress,
		total = excluded.total,
		found_count = excluded.found_count,
		started_at = excluded.started_at,
		completed_at = excluded.completed_at,
		results = excluded.results
	`

	_, err := s.db.ExecContext(ctx, query,
		task.ID, string(enginesJSON), task.Query, task.MaxResults,
		task.Status, task.Progress, task.Total, task.FoundCount,
		task.StartedAt, task.CompletedAt, resultsJSON,
	)

	return err
}

// GetTask 获取任务
func (s *SQLiteStorage) GetTask(ctx context.Context, id string) (*DiscoveryTask, error) {
	s.taskMu.RLock()
	defer s.taskMu.RUnlock()

	query := `SELECT * FROM tasks WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	task := &DiscoveryTask{}
	var enginesJSON, resultsJSON sql.NullString
	var startedAt, completedAt, createdAt sql.NullTime

	err := row.Scan(
		&task.ID, &enginesJSON, &task.Query, &task.MaxResults, &task.Status,
		&task.Progress, &task.Total, &task.FoundCount, &startedAt, &completedAt, &resultsJSON, &createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if enginesJSON.Valid && enginesJSON.String != "" {
		json.Unmarshal([]byte(enginesJSON.String), &task.Engines)
	}
	if resultsJSON.Valid && resultsJSON.String != "" {
		json.Unmarshal([]byte(resultsJSON.String), &task.Results)
	}
	if startedAt.Valid {
		task.StartedAt = startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = completedAt.Time
	}
	if createdAt.Valid {
		task.CreatedAt = createdAt.Time
	}

	return task, nil
}

// UpdateTask 更新任务
func (s *SQLiteStorage) UpdateTask(ctx context.Context, task *DiscoveryTask) error {
	return s.SaveTask(ctx, task)
}

// GetStats 获取统计数据
func (s *SQLiteStorage) GetStats(ctx context.Context) (*Stats, error) {
	// 按照固定顺序获取读锁：service -> model
	s.serviceMu.RLock()
	s.modelMu.RLock()
	// 按相反顺序释放
	defer s.modelMu.RUnlock()
	defer s.serviceMu.RUnlock()

	stats := &Stats{
		Timestamp: time.Now(),
	}

	// 服务统计
	err := s.db.QueryRowContext(ctx, `
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN status = 'online' THEN 1 ELSE 0 END) as online,
			SUM(CASE WHEN status = 'offline' THEN 1 ELSE 0 END) as offline,
			SUM(CASE WHEN is_honeypot = 1 THEN 1 ELSE 0 END) as honeypot
		FROM services
	`).Scan(&stats.TotalServices, &stats.OnlineServices, &stats.OfflineServices, &stats.HoneypotServices)
	if err != nil {
		return nil, err
	}

	// 模型统计
	err = s.db.QueryRowContext(ctx, `
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN is_available = 1 THEN 1 ELSE 0 END) as available
		FROM models
	`).Scan(&stats.TotalModels, &stats.AvailableModels)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return stats, nil
}

// Ping 健康检查
func (s *SQLiteStorage) Ping(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.db.PingContext(ctx)
}

// Close 关闭连接
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// ========== 通用任务存储方法 ==========

// SaveUniversalTask 保存通用任务
func (s *SQLiteStorage) SaveUniversalTask(ctx context.Context, task *Task) error {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()

	query := `
	INSERT INTO universal_tasks (
		id, type, title, status, progress, total, result, error,
		created_at, updated_at, completed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		type = excluded.type,
		title = excluded.title,
		status = excluded.status,
		progress = excluded.progress,
		total = excluded.total,
		result = excluded.result,
		error = excluded.error,
		updated_at = excluded.updated_at,
		completed_at = excluded.completed_at
	`

	_, err := s.db.ExecContext(ctx, query,
		task.ID, task.Type, task.Title, task.Status,
		task.Progress, task.Total, task.Result, task.Error,
		task.CreatedAt, task.UpdatedAt, task.CompletedAt,
	)

	return err
}

// GetUniversalTask 获取通用任务
func (s *SQLiteStorage) GetUniversalTask(ctx context.Context, id string) (*Task, error) {
	s.taskMu.RLock()
	defer s.taskMu.RUnlock()

	query := `SELECT * FROM universal_tasks WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	task := &Task{}
	var completedAt sql.NullTime
	var resultSQL, errorSQL sql.NullString

	err := row.Scan(
		&task.ID, &task.Type, &task.Title, &task.Status,
		&task.Progress, &task.Total, &resultSQL, &errorSQL,
		&task.CreatedAt, &task.UpdatedAt, &completedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}
	if resultSQL.Valid {
		task.Result = resultSQL.String
	}
	if errorSQL.Valid {
		task.Error = errorSQL.String
	}

	return task, nil
}

// ListUniversalTasks 列出通用任务
func (s *SQLiteStorage) ListUniversalTasks(ctx context.Context, filter TaskFilter) ([]*Task, error) {
	s.taskMu.RLock()
	defer s.taskMu.RUnlock()

	query := `SELECT * FROM universal_tasks WHERE 1=1`
	args := []interface{}{}

	if filter.Type != "" {
		query += ` AND type = ?`
		args = append(args, filter.Type)
	}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}

	query += ` ORDER BY created_at DESC`

	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]*Task, 0)
	for rows.Next() {
		task := &Task{}
		var completedAt sql.NullTime
		var resultSQL, errorSQL sql.NullString

		err := rows.Scan(
			&task.ID, &task.Type, &task.Title, &task.Status,
			&task.Progress, &task.Total, &resultSQL, &errorSQL,
			&task.CreatedAt, &task.UpdatedAt, &completedAt,
		)
		if err != nil {
			return nil, err
		}

		if completedAt.Valid {
			task.CompletedAt = &completedAt.Time
		}
		if resultSQL.Valid {
			task.Result = resultSQL.String
		}
		if errorSQL.Valid {
			task.Error = errorSQL.String
		}

		tasks = append(tasks, task)
	}

	return tasks, rows.Err()
}

// DeleteUniversalTask 删除通用任务
func (s *SQLiteStorage) DeleteUniversalTask(ctx context.Context, id string) error {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()

	_, err := s.db.ExecContext(ctx, `DELETE FROM universal_tasks WHERE id = ?`, id)
	return err
}

// SaveUser 保存用户
func (s *SQLiteStorage) SaveUser(ctx context.Context, user *User) error {
	s.userMu.Lock()
	defer s.userMu.Unlock()

	now := time.Now()
	user.UpdatedAt = now
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}

	query := `
	INSERT INTO users (id, username, password, nickname, email, role, status, last_login_at, failed_logins, locked_until, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		username = excluded.username,
		password = excluded.password,
		nickname = excluded.nickname,
		email = excluded.email,
		role = excluded.role,
		status = excluded.status,
		last_login_at = excluded.last_login_at,
		failed_logins = excluded.failed_logins,
		locked_until = excluded.locked_until,
		updated_at = excluded.updated_at
	`

	_, err := s.db.ExecContext(ctx, query,
		user.ID, user.Username, user.Password, user.Nickname,
		user.Email, user.Role, user.Status, user.LastLoginAt,
		user.FailedLogins, user.LockedUntil, user.CreatedAt, user.UpdatedAt,
	)

	return err
}

// GetUserByUsername 根据用户名获取用户
func (s *SQLiteStorage) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	s.userMu.RLock()
	defer s.userMu.RUnlock()

	query := `SELECT * FROM users WHERE username = ?`
	row := s.db.QueryRowContext(ctx, query, username)

	user := &User{}
	var lastLoginAt, lockedUntil sql.NullTime
	var nicknameSQL, emailSQL sql.NullString

	err := row.Scan(
		&user.ID, &user.Username, &user.Password, &nicknameSQL,
		&emailSQL, &user.Role, &user.Status, &lastLoginAt,
		&user.FailedLogins, &lockedUntil, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	if lockedUntil.Valid {
		user.LockedUntil = &lockedUntil.Time
	}
	if nicknameSQL.Valid {
		user.Nickname = nicknameSQL.String
	}
	if emailSQL.Valid {
		user.Email = emailSQL.String
	}

	return user, nil
}

// GetUser 根据 ID 获取用户
func (s *SQLiteStorage) GetUser(ctx context.Context, id string) (*User, error) {
	s.userMu.RLock()
	defer s.userMu.RUnlock()

	query := `SELECT * FROM users WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	user := &User{}
	var lastLoginAt, lockedUntil sql.NullTime
	var nicknameSQL, emailSQL sql.NullString

	err := row.Scan(
		&user.ID, &user.Username, &user.Password, &nicknameSQL,
		&emailSQL, &user.Role, &user.Status, &lastLoginAt,
		&user.FailedLogins, &lockedUntil, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	if lockedUntil.Valid {
		user.LockedUntil = &lockedUntil.Time
	}
	if nicknameSQL.Valid {
		user.Nickname = nicknameSQL.String
	}
	if emailSQL.Valid {
		user.Email = emailSQL.String
	}

	return user, nil
}

// ListUsers 获取所有用户
func (s *SQLiteStorage) ListUsers(ctx context.Context) ([]*User, error) {
	s.userMu.RLock()
	defer s.userMu.RUnlock()

	query := `SELECT * FROM users ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]*User, 0)
	for rows.Next() {
		user := &User{}
		var lastLoginAt, lockedUntil sql.NullTime
		var nicknameSQL, emailSQL sql.NullString
		err := rows.Scan(
			&user.ID, &user.Username, &user.Password, &nicknameSQL,
			&emailSQL, &user.Role, &user.Status, &lastLoginAt,
			&user.FailedLogins, &lockedUntil, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if lastLoginAt.Valid {
			user.LastLoginAt = &lastLoginAt.Time
		}
		if lockedUntil.Valid {
			user.LockedUntil = &lockedUntil.Time
		}
		if nicknameSQL.Valid {
			user.Nickname = nicknameSQL.String
		}
		if emailSQL.Valid {
			user.Email = emailSQL.String
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

// DeleteUser 删除用户
func (s *SQLiteStorage) DeleteUser(ctx context.Context, id string) error {
	s.userMu.Lock()
	defer s.userMu.Unlock()

	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}

// UpdateUser 更新用户
func (s *SQLiteStorage) UpdateUser(ctx context.Context, user *User) error {
	return s.SaveUser(ctx, user)
}

// ========== Token 黑名单方法 ==========

// AddToken 添加 Token 到黑名单
func (s *SQLiteStorage) AddToken(ctx context.Context, token string, expiresAt time.Time) error {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	query := `INSERT INTO token_blacklist (token, expires_at) VALUES (?, ?)`
	_, err := s.db.ExecContext(ctx, query, token, expiresAt)
	return err
}

// IsTokenBlacklisted 检查 Token 是否在黑名单中
func (s *SQLiteStorage) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	s.tokenMu.RLock()

	query := `SELECT expires_at FROM token_blacklist WHERE token = ?`
	var expiresAt time.Time
	err := s.db.QueryRowContext(ctx, query, token).Scan(&expiresAt)

	if err != nil {
		s.tokenMu.RUnlock()
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	// 检查是否过期
	now := time.Now()
	if now.After(expiresAt) {
		// 已过期，需要删除
		s.tokenMu.RUnlock() // 释放读锁
		s.tokenMu.Lock()    // 获取写锁
		defer s.tokenMu.Unlock()

		// 再次检查（因为在释放读锁后，其他goroutine可能已经修改了）
		var checkExpiresAt time.Time
		err := s.db.QueryRowContext(ctx, query, token).Scan(&checkExpiresAt)
		if err != nil {
			if err == sql.ErrNoRows {
				// 已经被其他goroutine删除
				return false, nil
			}
			return false, err
		}

		if now.After(checkExpiresAt) {
			// 仍然过期，删除
			_, err := s.db.ExecContext(ctx, `DELETE FROM token_blacklist WHERE token = ?`, token)
			if err != nil {
				return false, err
			}
		}
		// 如果现在没有过期（其他goroutine更新了过期时间），返回true
		return false, nil
	}

	s.tokenMu.RUnlock()
	return true, nil
}

// DeleteToken 从黑名单删除 Token
func (s *SQLiteStorage) DeleteToken(ctx context.Context, token string) error {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	_, err := s.db.ExecContext(ctx, `DELETE FROM token_blacklist WHERE token = ?`, token)
	return err
}

// CleanExpiredTokens 清理过期的 Token
func (s *SQLiteStorage) CleanExpiredTokens(ctx context.Context) error {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	_, err := s.db.ExecContext(ctx, `DELETE FROM token_blacklist WHERE expires_at < ?`, time.Now())
	return err
}

// GetWALStats 获取WAL模式统计信息
func (s *SQLiteStorage) GetWALStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 检查journal模式
	var journalMode string
	err := s.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		return nil, fmt.Errorf("查询journal_mode失败：%w", err)
	}
	stats["journal_mode"] = journalMode

	// 获取WAL检查点信息
	var busy, log, checkpointed int
	err = s.db.QueryRow("PRAGMA wal_checkpoint(PASSIVE)").Scan(&busy, &log, &checkpointed)
	if err == nil {
		stats["wal_checkpoint_busy"] = busy
		stats["wal_checkpoint_log"] = log
		stats["wal_checkpoint_checkpointed"] = checkpointed
	}

	// 获取数据库连接统计
	dbStats := s.db.Stats()
	stats["open_connections"] = dbStats.OpenConnections
	stats["in_use_connections"] = dbStats.InUse
	stats["idle_connections"] = dbStats.Idle
	stats["wait_count"] = dbStats.WaitCount
	stats["wait_duration"] = dbStats.WaitDuration.String()

	// 获取WAL文件信息（如果可能）
	if journalMode == "wal" {
		var pageSize, walAutoCheckpoint int
		s.db.QueryRow("PRAGMA page_size").Scan(&pageSize)
		s.db.QueryRow("PRAGMA wal_autocheckpoint").Scan(&walAutoCheckpoint)
		stats["page_size"] = pageSize
		stats["wal_autocheckpoint"] = walAutoCheckpoint
	}

	return stats, nil
}

// SaveActivityLog 保存活动日志
func (s *SQLiteStorage) SaveActivityLog(ctx context.Context, log *ActivityLog) error {
	query := `
	INSERT INTO activity_logs (type, action, target, user_id, metadata, created_at)
	VALUES (?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	if log.CreatedAt.IsZero() {
		log.CreatedAt = now
	}

	_, err := s.db.ExecContext(ctx, query,
		log.Type,
		log.Action,
		log.Target,
		log.UserID,
		log.Metadata,
		log.CreatedAt,
	)

	return err
}

// ListRecentActivities 查询最近活动日志
func (s *SQLiteStorage) ListRecentActivities(ctx context.Context, limit int) ([]*ActivityLog, error) {
	query := `
	SELECT id, type, action, target, user_id, metadata, created_at
	FROM activity_logs
	ORDER BY created_at DESC
	LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []*ActivityLog
	for rows.Next() {
		var log ActivityLog
		var metadata sql.NullString

		err := rows.Scan(
			&log.ID,
			&log.Type,
			&log.Action,
			&log.Target,
			&log.UserID,
			&metadata,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if metadata.Valid {
			log.Metadata = metadata.String
		}

		activities = append(activities, &log)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return activities, nil
}

// ListActivitiesByService 按服务 ID 查询活动日志
func (s *SQLiteStorage) ListActivitiesByService(ctx context.Context, serviceID string, limit int) ([]*ActivityLog, error) {
	query := `
	SELECT id, type, action, target, user_id, metadata, created_at
	FROM activity_logs
	WHERE metadata LIKE ?
	ORDER BY created_at DESC
	LIMIT ?
	`

	// 使用 LIKE 查询包含 service_id 的记录
	likePattern := fmt.Sprintf(`%%"service_id":"%s"%%`, serviceID)

	rows, err := s.db.QueryContext(ctx, query, likePattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []*ActivityLog
	for rows.Next() {
		var log ActivityLog
		var metadata sql.NullString

		err := rows.Scan(
			&log.ID,
			&log.Type,
			&log.Action,
			&log.Target,
			&log.UserID,
			&metadata,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if metadata.Valid {
			log.Metadata = metadata.String
		}

		activities = append(activities, &log)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return activities, nil
}

// MaintainWAL 维护 WAL模式
func (s *SQLiteStorage) MaintainWAL() error {
	// 检查当前journal模式
	var journalMode string
	err := s.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		return fmt.Errorf("查询journal_mode失败：%w", err)
	}

	if journalMode != "wal" {
		// 不是WAL模式，无需维护
		return nil
	}

	// 执行WAL检查点（TRUNCATE会尝试截断WAL文件）
	_, err = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		return fmt.Errorf("执行WAL检查点失败：%w", err)
	}

	logger.Storage().Printf("WAL 维护完成：已执行检查点")
	return nil
}
