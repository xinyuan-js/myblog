package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/example/myblog/apps/api/internal/auth"
)

type administratorHandler struct {
	service *auth.Service
	logger  *slog.Logger
}

func (h administratorHandler) list(c *gin.Context) {
	items, err := h.service.ListAdministrators(c.Request.Context())
	if err != nil {
		h.internalError(c, "list administrators", err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h administratorHandler) add(c *gin.Context) {
	var input struct {
		GitHubID uint64 `json:"githubId"`
	}
	if !decodeJSONBody(c, &input, 1024) {
		return
	}
	if input.GitHubID == 0 {
		writeValidationError(c, map[string]string{"githubId": "请输入有效的 GitHub 数字 ID"})
		return
	}
	session, _ := adminSession(c)
	item, err := h.service.AddAdministrator(c.Request.Context(), input.GitHubID, session.User.GitHubID)
	if errors.Is(err, auth.ErrOwnerManagedByConfig) {
		writeError(c, http.StatusConflict, "OWNER_MANAGED_BY_CONFIG", "站点所有者由服务器配置管理，无需重复添加")
		return
	}
	if err != nil {
		h.internalError(c, "add administrator", err)
		return
	}
	c.Header("Location", "/api/admin/administrators/"+strconv.FormatUint(item.GitHubID, 10))
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func (h administratorHandler) remove(c *gin.Context) {
	githubID, err := strconv.ParseUint(c.Param("githubId"), 10, 64)
	if err != nil || githubID == 0 {
		writeError(c, http.StatusBadRequest, "INVALID_ARGUMENT", "GitHub 用户 ID 不正确")
		return
	}
	err = h.service.RemoveAdministrator(c.Request.Context(), githubID)
	switch {
	case errors.Is(err, auth.ErrOwnerManagedByConfig):
		writeError(c, http.StatusConflict, "OWNER_MANAGED_BY_CONFIG", "不能移除服务器配置的站点所有者")
	case errors.Is(err, auth.ErrAdministratorMissing):
		writeError(c, http.StatusNotFound, "ADMINISTRATOR_NOT_FOUND", "管理员不存在")
	case err != nil:
		h.internalError(c, "remove administrator", err)
	default:
		c.Status(http.StatusNoContent)
	}
}

func (h administratorHandler) internalError(c *gin.Context, operation string, err error) {
	h.logger.Error(operation, "requestId", requestIDFromContext(c), "error", err)
	writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
}
