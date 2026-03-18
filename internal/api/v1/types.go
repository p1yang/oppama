package v1

import (
	"time"
)

// Assistant API 相关结构
type Assistant struct {
	ID           string                 `json:"id"`
	Object       string                 `json:"object"`
	CreatedAt    time.Time              `json:"created_at"`
	Name         string                 `json:"name,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Model        string                 `json:"model"`
	Instructions string                 `json:"instructions,omitempty"`
	Tools        []AssistantTool        `json:"tools,omitempty"`
	Metadata     map[string]string      `json:"metadata,omitempty"`
}

type AssistantTool struct {
	Type     string                 `json:"type"`
	Function map[string]interface{} `json:"function,omitempty"`
}

type Thread struct {
	ID        string            `json:"id"`
	Object    string            `json:"object"`
	CreatedAt time.Time         `json:"created_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type ThreadMessage struct {
	ID          string              `json:"id"`
	Object      string              `json:"object"`
	CreatedAt   time.Time           `json:"created_at"`
	ThreadID    string              `json:"thread_id"`
	Role        string              `json:"role"`
	Content     interface{}         `json:"content"`
	Attachments []MessageAttachment `json:"attachments,omitempty"`
	Metadata    map[string]string   `json:"metadata,omitempty"`
}

type MessageAttachment struct {
	FileID string   `json:"file_id"`
	Tools  []string `json:"tools,omitempty"`
}

type Run struct {
	ID            string              `json:"id"`
	Object        string              `json:"object"`
	CreatedAt     time.Time           `json:"created_at"`
	ThreadID      string              `json:"thread_id"`
	AssistantID   string              `json:"assistant_id"`
	Status        string              `json:"status"`
	RequiredAction interface{}         `json:"required_action,omitempty"`
	LastError     interface{}         `json:"last_error,omitempty"`
	ExpiresAt     *time.Time          `json:"expires_at,omitempty"`
	StartedAt     *time.Time          `json:"started_at,omitempty"`
	CompletedAt   *time.Time          `json:"completed_at,omitempty"`
	Model         string              `json:"model"`
	Instructions  string              `json:"instructions,omitempty"`
	Tools         []AssistantTool     `json:"tools,omitempty"`
	Metadata      map[string]string   `json:"metadata,omitempty"`
}

// Images API 相关结构
type ImageGenerationRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
}

type ImageResponse struct {
	Created int         `json:"created"`
	Data    []ImageData `json:"data"`
}

type ImageData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}
