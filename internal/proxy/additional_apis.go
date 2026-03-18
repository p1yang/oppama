package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Batch API
type BatchRequest struct {
	Requests []BatchRequestItem `json:"requests"`
	Window   int                `json:"window,omitempty"`
}

type BatchRequestItem struct {
	ID     string                 `json:"id"`
	Params map[string]interface{} `json:"params"`
}

type BatchResponse struct {
	ID          string            `json:"id"`
	Status      string            `json:"status"`
	Results     []BatchResult     `json:"results,omitempty"`
	ErrorCounts map[string]int    `json:"error_counts,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	FailedAt    *time.Time        `json:"failed_at,omitempty"`
}

type BatchResult struct {
	ID     string      `json:"id"`
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// Files API
type FileRequest struct {
	File    string `json:"file"`
	Purpose string `json:"purpose"`
}

type File struct {
	ID        string    `json:"id"`
	Object    string    `json:"object"`
	Bytes     int       `json:"bytes"`
	CreatedAt time.Time `json:"created_at"`
	Filename  string    `json:"filename"`
	Purpose   string    `json:"purpose"`
}

// Fine-tuning API
type FineTuningJob struct {
	ID             string                  `json:"id"`
	Status         string                  `json:"status"`
	Model          string                  `json:"model"`
	TrainingFile   string                  `json:"training_file"`
	ValidationFile string                  `json:"validation_file,omitempty"`
	CreatedAt      time.Time               `json:"created_at"`
	FinishedAt     *time.Time              `json:"finished_at,omitempty"`
	Result         string                  `json:"result,omitempty"`
	Error          string                  `json:"error,omitempty"`
	Hyperparams    FineTuningHyperparams   `json:"hyperparameters,omitempty"`
}

type FineTuningHyperparams struct {
	Epochs                   int     `json:"n_epochs,omitempty"`
	LearningRateMultiplier   float64 `json:"learning_rate_multiplier,omitempty"`
	BatchSize                int     `json:"batch_size,omitempty"`
}

type CreateFineTuningJobRequest struct {
	Model          string                 `json:"model"`
	TrainingFile   string                 `json:"training_file"`
	ValidationFile string                 `json:"validation_file,omitempty"`
	Hyperparams    FineTuningHyperparams  `json:"hyperparameters,omitempty"`
}

// CreateBatch 创建批量任务
func (p *ProxyService) CreateBatch(ctx context.Context, req *BatchRequest) (*BatchResponse, error) {
	batchID := fmt.Sprintf("batch_%d", time.Now().Unix())

	results := make([]BatchResult, 0, len(req.Requests))
	errorCounts := make(map[string]int)

	for _, item := range req.Requests {
		var result interface{}
		var err error

		if endpoint, ok := item.Params["endpoint"].(string); ok {
			switch endpoint {
			case "/v1/chat/completions":
				chatReq := &ChatCompletionRequest{}
				if bodyStr, ok := item.Params["body"].(string); ok {
					json.Unmarshal([]byte(bodyStr), chatReq)
				}
				result, err = p.ChatCompletions(ctx, chatReq)
			case "/v1/embeddings":
				embedReq := &EmbeddingRequest{}
				if bodyStr, ok := item.Params["body"].(string); ok {
					json.Unmarshal([]byte(bodyStr), embedReq)
				}
				result, err = p.Embeddings(ctx, embedReq)
			default:
				err = fmt.Errorf("unsupported endpoint: %s", endpoint)
			}
		}

		batchResult := BatchResult{
			ID:     item.ID,
			Status: "succeeded",
		}

		if err != nil {
			batchResult.Status = "failed"
			batchResult.Error = err.Error()
			errorCounts["failed"]++
		} else {
			batchResult.Result = result
		}

		results = append(results, batchResult)
	}

	now := time.Now()
	return &BatchResponse{
		ID:          batchID,
		Status:      "completed",
		Results:     results,
		ErrorCounts: errorCounts,
		CompletedAt: &now,
	}, nil
}
