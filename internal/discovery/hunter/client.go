package hunter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"oppama/internal/utils/logger"
)

// Client Hunter API 客户端
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Config Hunter 配置
type Config struct {
	Key        string
	BaseURL    string
	MaxResults int
}

// Result Hunter 搜索结果
type Result struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Domain   string `json:"domain"`
	Title    string `json:"web_title"`
	Protocol string `json:"protocol"`
	URL      string `json:"url"`
}

// Response Hunter API 响应
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Total        int         `json:"total"`
		Time         int         `json:"time"`
		Arr          []Result    `json:"arr"`
		ConsumeQuota interface{} `json:"consume_quota"` // 可能是字符串或数字
		RestQuota    interface{} `json:"rest_quota"`    // 可能是字符串或数字
		AccountType  string      `json:"account_type"`
	} `json:"data"`
}

// NewClient 创建 Hunter 客户端
func NewClient(cfg Config) *Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://hunter.qianxin.com/openApi"
	}

	return &Client{
		apiKey:  cfg.Key,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Search 搜索 Ollama 服务
func (c *Client) Search(ctx context.Context, query string, limit int) ([]string, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("Hunter API Key 未配置")
	}

	searchQuery := query
	if searchQuery == "" {
		searchQuery = `port="11434"` // 使用 port 查询，而不是 app.name
	}

	page := 1
	pageSize := 10
	if limit > 0 {
		pageSize = limit
		if pageSize > 100 {
			pageSize = 100
		}
	}

	allURLs := make([]string, 0)

	for {
		select {
		case <-ctx.Done():
			return allURLs, ctx.Err()
		default:
		}

		// Hunter API 要求查询语句 base64 编码
		encodedQuery := base64.StdEncoding.EncodeToString([]byte(searchQuery))

		reqURL := fmt.Sprintf(
			"%s/search?api-key=%s&search=%s&page=%d&page_size=%d&is_web=3",
			c.baseURL,
			c.apiKey,
			encodedQuery,
			page,
			pageSize,
		)

		logger.Hunter().Printf("请求 URL: %s", reqURL)
		logger.Hunter().Printf("查询语句：%s", searchQuery)

		resp, err := c.httpClient.Get(reqURL)
		if err != nil {
			return nil, fmt.Errorf("请求 Hunter API 失败：%w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("读取响应失败：%w", err)
		}

		// 打印原始响应用于调试
		logger.Hunter().Printf("原始响应 (page %d): %s", page, string(body))

		var hunterResp Response
		if err := json.Unmarshal(body, &hunterResp); err != nil {
			return nil, fmt.Errorf("解析响应失败：%w, 响应内容: %s", err, string(body))
		}

		if hunterResp.Code != 200 {
			return nil, fmt.Errorf("Hunter API 错误 [%d]: %s, 查询：%s", hunterResp.Code, hunterResp.Message, searchQuery)
		}

		// 处理 arr 为 null 的情况
		if hunterResp.Data.Arr == nil {
			logger.Hunter().Printf("查询无结果：total=%d, 查询语句：%s", hunterResp.Data.Total, searchQuery)
			return allURLs, nil
		}

		// 调试日志：显示查询结果
		logger.Hunter().Printf("查询：%s, 第%d页，返回 %d 条结果，总计：%d",
			searchQuery, page, len(hunterResp.Data.Arr), hunterResp.Data.Total)

		if len(hunterResp.Data.Arr) == 0 {
			// 返回空结果但不报错，让调用者知道没有数据
			return allURLs, nil
		}

		// 解析结果
		for _, item := range hunterResp.Data.Arr {
			urlStr := c.buildURL(item)
			logger.Hunter().Printf("构建 URL: IP=%s, Port=%d, URL=%s => %s", item.IP, item.Port, item.URL, urlStr)
			if urlStr != "" {
				allURLs = append(allURLs, urlStr)
			}
		}

		logger.Hunter().Printf("本页累计收集到 %d 个 URL", len(allURLs))

		// 检查是否还有更多结果
		if len(hunterResp.Data.Arr) < pageSize || (limit > 0 && len(allURLs) >= limit) {
			break
		}

		page++

		// 限制最大页数
		if page > 10 {
			break
		}
	}

	// 应用限制
	if limit > 0 && len(allURLs) > limit {
		allURLs = allURLs[:limit]
	}

	return allURLs, nil
}

// ValidateCredentials 验证 API 凭证
// 使用 /openApi/userInfo 端点验证 API Key
func (c *Client) ValidateCredentials(ctx context.Context) error {
	if c.apiKey == "" {
		return fmt.Errorf("API Key 未配置")
	}

	// 使用官方的 userInfo 接口验证凭证
	reqURL := fmt.Sprintf("%s/userInfo?api-key=%s", c.baseURL, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败：%w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("验证失败：%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败：%w", err)
	}

	var apiResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		// 返回更详细的错误信息，包含实际响应内容
		bodyStr := string(body)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "..."
		}
		return fmt.Errorf("解析响应失败，API 返回非 JSON 格式。响应内容: %s, 错误: %w", bodyStr, err)
	}

	// Hunter API 成功返回 code=200
	if apiResp.Code != 200 {
		if apiResp.Message == "" {
			apiResp.Message = "未知错误"
		}
		return fmt.Errorf("凭证无效：%s", apiResp.Message)
	}

	return nil
}

// GetQuota 获取配额信息
// 使用 /openApi/userInfo 端点获取用户配额信息
func (c *Client) GetQuota(ctx context.Context) (int, error) {
	reqURL := fmt.Sprintf("%s/userInfo?api-key=%s", c.baseURL, c.apiKey)

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// 根据官方文档定义响应结构
	var apiResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Type            string `json:"type"`              // 账号类型
			RestEquityPoint int    `json:"rest_equity_point"` // 剩余权益积分
			RestFreePoint   int    `json:"rest_free_point"`   // 当日剩余免费积分
			RestExportQuota int    `json:"rest_export_quota"` // 当日剩余导出额度
			DayFreePoint    int    `json:"day_free_point"`    // 当日免费积分上限
			DayExportQuota  int    `json:"day_export_quota"`  // 当日导出额度上限
			OnceExportQuota int    `json:"once_export_quota"` // 单次导出额度上限
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		// 返回更详细的错误信息
		bodyStr := string(body)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "..."
		}
		return 0, fmt.Errorf("解析响应失败。响应内容: %s, 错误: %w", bodyStr, err)
	}

	// 返回当日剩余免费积分
	return apiResp.Data.RestFreePoint, nil
}

// buildURL 构建 URL
func (c *Client) buildURL(result Result) string {
	if result.URL != "" {
		// 确保协议正确
		if !strings.HasPrefix(result.URL, "http://") && !strings.HasPrefix(result.URL, "https://") {
			scheme := "http"
			if result.Protocol == "https" || result.Port == 443 {
				scheme = "https"
			}
			return fmt.Sprintf("%s://%s", scheme, result.URL)
		}
		return result.URL
	}

	// 手动构建 URL
	scheme := "http"
	if result.Protocol == "https" || result.Port == 443 || result.Port == 8443 {
		scheme = "https"
	}

	host := result.IP
	if result.Domain != "" {
		host = result.Domain
	}

	return fmt.Sprintf("%s://%s:%d", scheme, host, result.Port)
}
