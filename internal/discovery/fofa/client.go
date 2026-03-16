package fofa

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client FOFA API 客户端
type Client struct {
	email      string
	key        string
	baseURL    string
	httpClient *http.Client
}

// Config FOFA 配置
type Config struct {
	Email      string
	Key        string
	BaseURL    string // 可选，默认使用官方 API
	MaxResults int
}

// Result FOFA 搜索结果
type Result struct {
	Host   string `json:"host"`
	IP     string `json:"ip"`
	Port   int    `json:"port"`
	Domain string `json:"domain"`
	Title  string `json:"title"`
}

// Response FOFA API 响应
type Response struct {
	Error   bool          `json:"error"`
	ErrMsg  string        `json:"errmsg"`
	Size    int           `json:"size"`
	Results [][]string    `json:"results"`
}

// NewClient 创建 FOFA 客户端
func NewClient(cfg Config) *Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://fofa.info/api/v1"
	}

	return &Client{
		email:   cfg.Email,
		key:     cfg.Key,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Search 搜索 Ollama 服务
func (c *Client) Search(ctx context.Context, query string, limit int) ([]string, error) {
	if c.email == "" || c.key == "" {
		return nil, fmt.Errorf("FOFA API 凭证未配置")
	}

	// 构建搜索查询
	searchQuery := query
	if searchQuery == "" {
		searchQuery = `app="Ollama"`
	}

	// 计算分页
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

		// 构建请求 URL
		reqURL := fmt.Sprintf(
			"%s/search?email=%s&key=%s&base64=true&qbase64=%s&page=%d&size=%d&fields=host,ip,port",
			c.baseURL,
			url.QueryEscape(c.email),
			url.QueryEscape(c.key),
			base64.StdEncoding.EncodeToString([]byte(searchQuery)),
			page,
			pageSize,
		)

		resp, err := c.httpClient.Get(reqURL)
		if err != nil {
			return nil, fmt.Errorf("请求 FOFA API 失败：%w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("读取响应失败：%w", err)
		}

		var fofaResp Response
		if err := json.Unmarshal(body, &fofaResp); err != nil {
			return nil, fmt.Errorf("解析响应失败：%w", err)
		}

		if fofaResp.Error {
			return nil, fmt.Errorf("FOFA API 错误：%s", fofaResp.ErrMsg)
		}

		// 解析结果
		for _, item := range fofaResp.Results {
			if len(item) >= 3 {
				host := item[0]
				ip := item[1]
				port := item[2]

				// 构建 URL
				hostToUse := host
				if hostToUse == "" {
					hostToUse = ip
				}

				url := c.buildURL(hostToUse, port)
				if url != "" {
					allURLs = append(allURLs, url)
				}
			}
		}

		// 检查是否还有更多结果
		if len(fofaResp.Results) < pageSize || (limit > 0 && len(allURLs) >= limit) {
			break
		}

		page++

		// 限制最大页数（避免过度请求）
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
	if c.email == "" || c.key == "" {
		return fmt.Errorf("API 凭证未配置")
	}

	reqURL := fmt.Sprintf(
		"%s/my/info?email=%s&key=%s",
		c.baseURL,
		url.QueryEscape(c.email),
		url.QueryEscape(c.key),
	)

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
		Error   bool   `json:"error"`
		ErrMsg  string `json:"errmsg"`
		Email   string `json:"email"`
		Fcoin   int    `json:"fcoin"`
	}

	if err := json.Unmarshal(body, &infoResp); err != nil {
		return err
	}

	if infoResp.Error {
		return fmt.Errorf("凭证无效：%s", infoResp.ErrMsg)
	}

	return nil
}

// GetQuota 获取配额信息
func (c *Client) GetQuota(ctx context.Context) (int, error) {
	reqURL := fmt.Sprintf(
		"%s/my/info?email=%s&key=%s",
		c.baseURL,
		url.QueryEscape(c.email),
		url.QueryEscape(c.key),
	)

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
		Error bool `json:"error"`
		Fcoin int  `json:"fcoin"`
	}

	if err := json.Unmarshal(body, &infoResp); err != nil {
		return 0, err
	}

	return infoResp.Fcoin, nil
}

// buildURL 构建完整 URL
func (c *Client) buildURL(host string, portStr string) string {
	port := 80
	fmt.Sscanf(portStr, "%d", &port)

	// 跳过常见非 HTTP 端口
	if port == 22 || port == 23 || port == 3389 || port == 445 {
		return ""
	}

	scheme := "http"
	if port == 443 || port == 8443 {
		scheme = "https"
	}

	// 检查是否是 IP 地址
	isIP := strings.Count(host, ".") == 3

	var url string
	if isIP {
		url = fmt.Sprintf("%s://%s:%d", scheme, host, port)
	} else {
		// 优先使用域名
		url = fmt.Sprintf("%s://%s:%d", scheme, host, port)
	}

	return url
}
