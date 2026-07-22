package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/xinyuan-js/myblog/apps/api/internal/auth"
)

const adminSessionKey = "admin_session"

type authHandler struct {
	service *auth.Service
	logger  *slog.Logger
}

func (h authHandler) beginGitHub(c *gin.Context) {
	target, cookie, err := h.service.BeginLogin(c.Query("return_to"))
	if errors.Is(err, auth.ErrNotConfigured) {
		writeError(c, http.StatusServiceUnavailable, "OAUTH_NOT_CONFIGURED", "GitHub 登录尚未配置")
		return
	}
	if err != nil {
		h.internalError(c, "begin github oauth", err)
		return
	}
	http.SetCookie(c.Writer, cookie)
	c.Redirect(http.StatusFound, target)
}

func (h authHandler) githubCallback(c *gin.Context) {
	// Set before any redirect writes the response headers.
	http.SetCookie(c.Writer, h.service.ClearStateCookie())
	if c.Query("error") == "access_denied" {
		h.redirectLoginError(c, "access_denied")
		return
	}
	stateCookie, err := c.Request.Cookie("blog_oauth_state")
	if err != nil {
		h.redirectLoginError(c, "state_invalid")
		return
	}
	session, returnTo, err := h.service.CompleteLogin(c.Request.Context(), c.Query("code"), c.Query("state"), stateCookie.Value)
	switch {
	case errors.Is(err, auth.ErrInvalidState):
		h.redirectLoginError(c, "state_invalid")
		return
	case errors.Is(err, auth.ErrForbidden):
		h.logger.Warn("github login rejected", "requestId", requestIDFromContext(c))
		h.redirectLoginError(c, "not_authorized")
		return
	case err != nil:
		h.logger.Error("complete github oauth", "requestId", requestIDFromContext(c), "error", err)
		h.redirectLoginError(c, "oauth_failed")
		return
	}
	cookie, err := h.service.SessionCookie(session)
	if err != nil {
		h.internalError(c, "create session cookie", err)
		return
	}
	http.SetCookie(c.Writer, cookie)
	h.logger.Info("administrator logged in", "requestId", requestIDFromContext(c), "githubId", session.User.GitHubID)
	c.Redirect(http.StatusFound, h.service.AppURL(returnTo))
}

func (h authHandler) me(c *gin.Context) {
	session, ok, err := h.optionalSession(c)
	if err != nil {
		h.internalError(c, "read current session", err)
		return
	}
	c.Header("Cache-Control", "no-store")
	if !ok {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"authenticated": false, "user": nil, "csrfToken": nil}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"authenticated": true, "user": session.User, "csrfToken": session.CSRFToken,
	}})
}

func (h authHandler) logout(c *gin.Context) {
	defer http.SetCookie(c.Writer, h.service.ClearSessionCookie())
	session, ok, err := h.optionalSession(c)
	if err != nil {
		h.internalError(c, "read logout session", err)
		return
	}
	if !ok {
		c.Status(http.StatusNoContent)
		return
	}
	if !h.service.ValidOrigin(c.Request) || !h.service.ValidCSRF(session, c.GetHeader("X-CSRF-Token")) {
		writeError(c, http.StatusForbidden, "CSRF_INVALID", "CSRF 校验失败")
		return
	}
	if err := h.service.Logout(c.Request.Context(), session); err != nil {
		h.internalError(c, "logout administrator", err)
		return
	}
	h.logger.Info("administrator logged out", "requestId", requestIDFromContext(c), "githubId", session.User.GitHubID)
	c.Status(http.StatusNoContent)
}

func (h authHandler) optionalSession(c *gin.Context) (auth.Session, bool, error) {
	cookie, err := c.Request.Cookie(h.service.SessionCookieName())
	if err != nil {
		return auth.Session{}, false, nil
	}
	session, err := h.service.Authenticate(c.Request.Context(), cookie.Value)
	if errors.Is(err, auth.ErrUnauthenticated) {
		http.SetCookie(c.Writer, h.service.ClearSessionCookie())
		return auth.Session{}, false, nil
	}
	return session, err == nil, err
}

func (h authHandler) redirectLoginError(c *gin.Context, code string) {
	target := h.service.AppURL("/admin/login?error=" + url.QueryEscape(code))
	c.Redirect(http.StatusFound, target)
}

func (h authHandler) internalError(c *gin.Context, operation string, err error) {
	h.logger.Error(operation, "requestId", requestIDFromContext(c), "error", err)
	writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
}

func requireAdmin(service *auth.Service, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Request.Cookie(service.SessionCookieName())
		if err != nil {
			writeError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "请先登录管理端")
			return
		}
		session, err := service.Authenticate(c.Request.Context(), cookie.Value)
		if errors.Is(err, auth.ErrUnauthenticated) {
			http.SetCookie(c.Writer, service.ClearSessionCookie())
			writeError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "登录状态已失效")
			return
		}
		if err != nil {
			logger.Error("authenticate administrator", "requestId", requestIDFromContext(c), "error", err)
			writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
			return
		}
		c.Set(adminSessionKey, session)
		c.Next()
	}
}

func requireCSRF(service *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, ok := adminSession(c)
		if !ok || !service.ValidOrigin(c.Request) || !service.ValidCSRF(session, c.GetHeader("X-CSRF-Token")) {
			writeError(c, http.StatusForbidden, "CSRF_INVALID", "CSRF 校验失败")
			return
		}
		c.Next()
	}
}

func adminSession(c *gin.Context) (auth.Session, bool) {
	value, ok := c.Get(adminSessionKey)
	if !ok {
		return auth.Session{}, false
	}
	session, ok := value.(auth.Session)
	return session, ok
}
