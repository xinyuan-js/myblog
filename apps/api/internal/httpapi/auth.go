package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/example/myblog/apps/api/internal/auth"
	"github.com/gin-gonic/gin"
)

const adminSessionKey = "admin_session"

type authHandler struct {
	service *auth.Service
	logger  *slog.Logger
}

func (h authHandler) beginGitHub(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
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
	c.Header("Cache-Control", "no-store")
	// Set before any redirect writes the response headers.
	http.SetCookie(c.Writer, h.service.ClearStateCookie())
	http.SetCookie(c.Writer, h.service.ClearLegacySessionCookie())
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
	h.logger.Info("user logged in", "requestId", requestIDFromContext(c), "githubId", session.User.GitHubID, "isAdmin", session.User.IsAdmin)
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
	// Migrate sessions created before the comment policy gateway changed the
	// cookie path from /api to /. The legacy cookie is deleted in the same
	// response so duplicate cookie names cannot confuse later authentication.
	if cookie, cookieErr := h.service.SessionCookie(session); cookieErr == nil {
		http.SetCookie(c.Writer, h.service.ClearLegacySessionCookie())
		http.SetCookie(c.Writer, cookie)
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"authenticated": true, "user": session.User, "csrfToken": session.CSRFToken,
	}})
}

func (h authHandler) logout(c *gin.Context) {
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
		h.internalError(c, "logout user", err)
		return
	}
	http.SetCookie(c.Writer, h.service.ClearSessionCookie())
	http.SetCookie(c.Writer, h.service.ClearLegacySessionCookie())
	h.logger.Info("user logged out", "requestId", requestIDFromContext(c), "githubId", session.User.GitHubID)
	c.Status(http.StatusNoContent)
}

func (h authHandler) artalkSession(c *gin.Context) {
	session, ok, err := h.optionalSession(c)
	if err != nil {
		h.internalError(c, "read Artalk session", err)
		return
	}
	if !ok {
		writeError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "请先登录")
		return
	}
	if !h.service.ValidOrigin(c.Request) || !h.service.ValidCSRF(session, c.GetHeader("X-CSRF-Token")) {
		writeError(c, http.StatusForbidden, "CSRF_INVALID", "CSRF 校验失败")
		return
	}
	artalkSession, err := h.service.CreateArtalkSession(c.Request.Context(), session)
	if writeCommentPolicyError(c, err) {
		return
	}
	if err != nil {
		h.internalError(c, "create Artalk session", err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"data": artalkSession})
}

func (h authHandler) artalkUserInfo(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	token := c.GetHeader("Authorization")
	if !strings.HasPrefix(token, "Bearer ") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}
	user, err := h.service.VerifyArtalkToken(strings.TrimPrefix(token, "Bearer "))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h authHandler) optionalSession(c *gin.Context) (auth.Session, bool, error) {
	cookie, err := c.Request.Cookie(h.service.SessionCookieName())
	if err != nil {
		return auth.Session{}, false, nil
	}
	session, err := h.service.Authenticate(c.Request.Context(), cookie.Value)
	if errors.Is(err, auth.ErrUnauthenticated) {
		http.SetCookie(c.Writer, h.service.ClearSessionCookie())
		http.SetCookie(c.Writer, h.service.ClearLegacySessionCookie())
		return auth.Session{}, false, nil
	}
	return session, err == nil, err
}

func (h authHandler) redirectLoginError(c *gin.Context, code string) {
	// A failed OAuth attempt must leave the browser logged out of this
	// application, even if it arrived with an older blog session cookie.
	http.SetCookie(c.Writer, h.service.ClearSessionCookie())
	http.SetCookie(c.Writer, h.service.ClearLegacySessionCookie())
	target := h.service.AppURL("/login?error=" + url.QueryEscape(code))
	c.Redirect(http.StatusFound, target)
}

func (h authHandler) internalError(c *gin.Context, operation string, err error) {
	h.logger.Error(operation, "requestId", requestIDFromContext(c), "error", err)
	writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
}

func requireAdmin(service *auth.Service, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		cookie, err := c.Request.Cookie(service.SessionCookieName())
		if err != nil {
			writeError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "请先登录管理端")
			return
		}
		session, err := service.Authenticate(c.Request.Context(), cookie.Value)
		if errors.Is(err, auth.ErrUnauthenticated) {
			http.SetCookie(c.Writer, service.ClearSessionCookie())
			http.SetCookie(c.Writer, service.ClearLegacySessionCookie())
			writeError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "登录状态已失效")
			return
		}
		if err != nil {
			logger.Error("authenticate administrator", "requestId", requestIDFromContext(c), "error", err)
			writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
			return
		}
		if !session.User.IsAdmin {
			writeError(c, http.StatusForbidden, "ADMIN_REQUIRED", "当前账号没有管理权限")
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

func requireOwner() gin.HandlerFunc {
	return func(c *gin.Context) {
		session, ok := adminSession(c)
		if !ok || !session.User.IsOwner {
			writeError(c, http.StatusForbidden, "OWNER_REQUIRED", "只有站点所有者可以管理管理员权限")
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
