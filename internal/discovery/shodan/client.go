package shodan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client Shodan API 客户端
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Config Shodan 配置
type Config struct {
	Key        string
	BaseURL    string
	MaxResults int
}

// Result Shodan 搜索结果
type Result struct {
	IPStr     string   `json:"ip_str"`
	Port      int      `json:"port"`
	Domains   []string `json:"domains"`
	Hostnames []string `json:"hostnames"`
	Title     string   `json:"title"`
	Product   string   `json:"product"`
	Transport string   `json:"transport"`
}

// Response Shodan API 响应
type Response struct {
	Total   int      `json:"total"`
	Matches []Result `json:"matches"`
}

// NewClient 创建 Shodan 客户端
func NewClient(cfg Config) *Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.shodan.io"
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
		return nil, fmt.Errorf("Shodan API Key 未配置")
	}

	searchQuery := query
	if searchQuery == "" {
		searchQuery = `http.title:"Ollama"`
	}

	page := 1
	pageSize := 100
	if limit > 0 && limit < pageSize {
		pageSize = limit
	}

	allURLs := make([]string, 0)

	for {
		select {
		case <-ctx.Done():
			return allURLs, ctx.Err()
		default:
		}

		reqURL := fmt.Sprintf(
			"%s/shodan/host/search?key=%s&query=%s&page=%d",
			c.baseURL,
			c.apiKey,
			url.QueryEscape(searchQuery),
			page,
		)

		resp, err := c.httpClient.Get(reqURL)
		if err != nil {
			return nil, fmt.Errorf("请求 Shodan API 失败：%w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("读取响应失败：%w", err)
		}

		var shodanResp Response
		if err := json.Unmarshal(body, &shodanResp); err != nil {
			return nil, fmt.Errorf("解析响应失败：%w", err)
		}

		// 解析结果
		for _, item := range shodanResp.Matches {
			urlStr := c.buildURL(item)
			if urlStr != "" {
				allURLs = append(allURLs, urlStr)
			}
		}

		// 检查是否还有更多结果
		if len(shodanResp.Matches) < pageSize || (limit > 0 && len(allURLs) >= limit) {
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
func (c *Client) ValidateCredentials(ctx context.Context) error {
	if c.apiKey == "" {
		return fmt.Errorf("API Key 未配置")
	}

	reqURL := fmt.Sprintf("%s/api-info?key=%s", c.baseURL, c.apiKey)

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return fmt.Errorf("验证失败：%w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var infoResp struct {
		Error        string `json:"error"`
		Member       bool   `json:"member"`
		Credits      int    `json:"credits"`
		QueryCredits int    `json:"query_credits"`
	}

	if err := json.Unmarshal(body, &infoResp); err != nil {
		return err
	}

	if infoResp.Error != "" {
		return fmt.Errorf("凭证无效：%s", infoResp.Error)
	}

	return nil
}

// GetQuota 获取配额信息
func (c *Client) GetQuota(ctx context.Context) (int, error) {
	reqURL := fmt.Sprintf("%s/api-info?key=%s", c.baseURL, c.apiKey)

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var infoResp struct {
		QueryCredits int `json:"query_credits"`
	}

	if err := json.Unmarshal(body, &infoResp); err != nil {
		return 0, err
	}

	return infoResp.QueryCredits, nil
}

// buildURL 构建 URL
func (c *Client) buildURL(result Result) string {
	if result.IPStr == "" || result.Port == 0 {
		return ""
	}

	// 跳过非 HTTP 服务
	if result.Transport == "udp" {
		return ""
	}

	scheme := "http"
	if result.Port == 443 || result.Port == 8443 {
		scheme = "https"
	}

	// 优先使用域名
	host := result.IPStr
	if len(result.Hostnames) > 0 {
		host = result.Hostnames[0]
	} else if len(result.Domains) > 0 {
		host = result.Domains[0]
	}

	return fmt.Sprintf("%s://%s:%d", scheme, host, result.Port)
}

// HostInfo 获取主机详细信息
func (c *Client) HostInfo(ctx context.Context, ip string) (*Result, error) {
	reqURL := fmt.Sprintf("%s/shodan/host/%s?key=%s", c.baseURL, ip, c.apiKey)

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result Result
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
