package httpapi

import "github.com/gin-gonic/gin"

type errorResponse struct {
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	FieldErrors map[string]string `json:"fieldErrors,omitempty"`
	RequestID   string            `json:"requestId"`
}

func writeError(c *gin.Context, status int, code, message string) {
	writeErrorFields(c, status, code, message, nil)
}

func writeErrorFields(c *gin.Context, status int, code, message string, fields map[string]string) {
	// Several error statuses (including 404) are cacheable by default. A
	// cached missing-post response would hide a later publication, and error
	// payloads also contain request-specific identifiers.
	c.Header("Cache-Control", "no-store")
	c.AbortWithStatusJSON(status, errorResponse{
		Code: code, Message: message, FieldErrors: fields, RequestID: requestIDFromContext(c),
	})
}
