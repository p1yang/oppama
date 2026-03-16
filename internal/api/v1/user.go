package v1

import (
	"net/http"

	"oppama/internal/api/middleware"
	"oppama/internal/config"
	"oppama/internal/storage"

	"github.com/gin-gonic/gin"
)

// UserHandler 用户管理处理器
type UserHandler struct {
	storage  storage.Storage
	authCfg  config.AuthConfig
}

// NewUserHandler 创建用户管理处理器
func NewUserHandler(store storage.Storage, authCfg config.AuthConfig) *UserHandler {
	return &UserHandler{
		storage: store,
		authCfg: authCfg,
	}
}

// ListUsers 获取用户列表（仅管理员）
// @Summary 获取用户列表
// @Tags 用户管理
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	// 检查管理员权限
	if !middleware.IsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}

	users, err := h.storage.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 清除密码字段
	for _, user := range users {
		user.Password = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  users,
		"total": len(users),
	})
}

// GetUser 获取单个用户信息
// @Summary 获取用户信息
// @Tags 用户管理
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户ID"
// @Success 200 {object} storage.User
// @Router /api/v1/users/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	id := c.Param("id")

	// 检查权限：只能查看自己或管理员
	currentUserID, _ := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)
	if id != currentUserID && !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}

	user, err := h.storage.GetUser(c.Request.Context(), id)
	if err != nil {
		if err == storage.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 清除密码
	user.Password = ""

	c.JSON(http.StatusOK, gin.H{"data": user})
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=8"`
	Nickname string `json:"nickname" binding:"max=100"`
	Email    string `json:"email" binding:"email"`
	Role     string `json:"role" binding:"omitempty,oneof=admin user"`
}

// CreateUser 创建用户（仅管理员）
// @Summary 创建用户
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateUserRequest true "用户信息"
// @Success 201 {object} storage.User
// @Router /api/v1/users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	// 检查管理员权限
	if !middleware.IsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}

	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证密码策略
	if err := h.validatePassword(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查用户是否已存在
	_, err := h.storage.GetUserByUsername(c.Request.Context(), req.Username)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	}

	// 哈希密码
	hash, err := storage.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	// 默认角色
	role := req.Role
	if role == "" {
		role = "user"
	}

	// 创建用户
	user := &storage.User{
		ID:       generateID(),
		Username: req.Username,
		Password: hash,
		Nickname: req.Nickname,
		Email:    req.Email,
		Role:     role,
		Status:   storage.UserStatusActive,
	}

	if err := h.storage.SaveUser(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 清除密码
	user.Password = ""

	c.JSON(http.StatusCreated, gin.H{"data": user})
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Nickname string `json:"nickname" binding:"omitempty,max=100"`
	Email    string `json:"email" binding:"omitempty,email"`
	Role     string `json:"role" binding:"omitempty,oneof=admin user"`
	Status   string `json:"status" binding:"omitempty,oneof=active disabled locked require_password_change"`
}

// UpdateUser 更新用户
// @Summary 更新用户
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户ID"
// @Param request body UpdateUserRequest true "更新信息"
// @Success 200 {object} storage.User
// @Router /api/v1/users/{id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")

	// 检查权限
	currentUserID, _ := middleware.GetUserID(c)
	isAdmin := middleware.IsAdmin(c)

	if id != currentUserID && !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}

	user, err := h.storage.GetUser(c.Request.Context(), id)
	if err != nil {
		if err == storage.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 非管理员只能修改昵称和邮箱
	if !isAdmin {
		if req.Role != "" || req.Status != "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "只有管理员可以修改角色和状态"})
			return
		}
	}

	// 更新字段
	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Role != "" && isAdmin {
		user.Role = req.Role
	}
	if req.Status != "" && isAdmin {
		user.Status = req.Status
	}

	if err := h.storage.SaveUser(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	user.Password = ""
	c.JSON(http.StatusOK, gin.H{"data": user})
}

// DeleteUser 删除用户（仅管理员）
// @Summary 删除用户
// @Tags 用户管理
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户ID"
// @Success 200 {object} map[string]string
// @Router /api/v1/users/{id} [delete]
func (h *UserHandler) DeleteUser(c *gin.Context) {
	// 检查管理员权限
	if !middleware.IsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}

	id := c.Param("id")

	// 不允许删除自己
	currentUserID, _ := middleware.GetUserID(c)
	if id == currentUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除自己的账户"})
		return
	}

	if err := h.storage.DeleteUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=8"`
}

// ResetPassword 重置用户密码（仅管理员）
// @Summary 重置用户密码
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户ID"
// @Param request body ResetPasswordRequest true "新密码"
// @Success 200 {object} map[string]string
// @Router /api/v1/users/{id}/reset-password [post]
func (h *UserHandler) ResetPassword(c *gin.Context) {
	// 检查管理员权限
	if !middleware.IsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}

	id := c.Param("id")

	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证密码策略
	if err := h.validatePassword(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.storage.GetUser(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 哈希新密码
	hash, err := storage.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	user.Password = hash
	user.Status = storage.UserStatusRequirePasswordChange // 要求用户下次登录修改密码

	if err := h.storage.SaveUser(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码重置成功"})
}

// validatePassword 验证密码是否符合策略
func (h *UserHandler) validatePassword(password string) error {
	policy := h.authCfg.PasswordPolicy

	// 检查长度
	if len(password) < policy.MinLength {
		return &ValidationError{
			Field:   "password",
			Message: "密码长度必须大于等于 " + string(rune(policy.MinLength)) + " 位",
		}
	}

	// 检查大写字母
	if policy.RequireUpper {
		hasUpper := false
		for _, c := range password {
			if c >= 'A' && c <= 'Z' {
				hasUpper = true
				break
			}
		}
		if !hasUpper {
			return &ValidationError{
				Field:   "password",
				Message: "密码必须包含至少一个大写字母",
			}
		}
	}

	// 检查小写字母
	if policy.RequireLower {
		hasLower := false
		for _, c := range password {
			if c >= 'a' && c <= 'z' {
				hasLower = true
				break
			}
		}
		if !hasLower {
			return &ValidationError{
				Field:   "password",
				Message: "密码必须包含至少一个小写字母",
			}
		}
	}

	// 检查数字
	if policy.RequireNumbers {
		hasNumber := false
		for _, c := range password {
			if c >= '0' && c <= '9' {
				hasNumber = true
				break
			}
		}
		if !hasNumber {
			return &ValidationError{
				Field:   "password",
				Message: "密码必须包含至少一个数字",
			}
		}
	}

	// 检查特殊字符
	if policy.RequireSpecial {
		specials := "!@#$%^&*()_+-=[]{}|;':\",./<>?`~"
		hasSpecial := false
		for _, c := range password {
			for _, s := range specials {
				if c == s {
					hasSpecial = true
					break
				}
			}
			if hasSpecial {
				break
			}
		}
		if !hasSpecial {
			return &ValidationError{
				Field:   "password",
				Message: "密码必须包含至少一个特殊字符",
			}
		}
	}

	return nil
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
