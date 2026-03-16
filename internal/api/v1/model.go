package v1

import (
	"net/http"
	"sort"

	"oppama/internal/storage"

	"github.com/gin-gonic/gin"
)

// ModelHandler 模型管理处理器
type ModelHandler struct {
	storage storage.Storage
}

// NewModelHandler 创建模型处理器
func NewModelHandler(storage storage.Storage) *ModelHandler {
	return &ModelHandler{
		storage: storage,
	}
}

// ListModels 获取所有可用模型
func (h *ModelHandler) ListModels(c *gin.Context) {
	var filter storage.ModelFilter

	// 解析查询参数
	if family := c.Query("family"); family != "" {
		filter.Family = family
	}
	if available := c.Query("available"); available == "true" {
		filter.AvailableOnly = true
	}
	if serviceID := c.Query("service_id"); serviceID != "" {
		filter.ServiceID = serviceID
	}

	models, err := h.storage.ListModels(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  models,
		"total": len(models),
	})
}

// RecommendModels 获取推荐模型
func (h *ModelHandler) RecommendModels(c *gin.Context) {
	// 获取所有在线服务
	services, err := h.storage.ListServices(c.Request.Context(), storage.ServiceFilter{
		Status: func() *storage.ServiceStatus {
			s := storage.StatusOnline
			return &s
		}(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 统计模型出现次数和可用性
	modelStats := make(map[string]*struct {
		Name         string
		ServiceCount int
		TotalSize    int64
		Family       string
		Services     []string
	})

	for _, service := range services {
		if service.IsHoneypot || service.Status != storage.StatusOnline {
			continue
		}

		for _, model := range service.Models {
			if !model.IsAvailable {
				continue
			}

			if stats, ok := modelStats[model.Name]; ok {
				stats.ServiceCount++
				stats.TotalSize += model.Size
				stats.Services = append(stats.Services, service.URL)
			} else {
				modelStats[model.Name] = &struct {
					Name         string
					ServiceCount int
					TotalSize    int64
					Family       string
					Services     []string
				}{
					Name:         model.Name,
					ServiceCount: 1,
					TotalSize:    model.Size,
					Family:       model.Family,
					Services:     []string{service.URL},
				}
			}
		}
	}

	// 转换为列表并排序
	type RecommendedModel struct {
		Name         string   `json:"name"`
		Family       string   `json:"family"`
		ServiceCount int      `json:"service_count"`
		AvgSize      int64    `json:"avg_size"`
		Services     []string `json:"services"`
		Score        int      `json:"score"` // 推荐分数
	}

	recommendations := make([]RecommendedModel, 0)
	for _, stats := range modelStats {
		rec := RecommendedModel{
			Name:         stats.Name,
			Family:       stats.Family,
			ServiceCount: stats.ServiceCount,
			AvgSize:      stats.TotalSize / int64(stats.ServiceCount),
			Services:     stats.Services,
			Score:        stats.ServiceCount * 10, // 简单评分：服务数量越多分数越高
		}
		recommendations = append(recommendations, rec)
	}

	// 按分数排序
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Score > recommendations[j].Score
	})

	// 限制返回数量
	limit := 20
	if len(recommendations) > limit {
		recommendations = recommendations[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"data": recommendations,
	})
}
