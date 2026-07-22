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
	c.AbortWithStatusJSON(status, errorResponse{
		Code: code, Message: message, FieldErrors: fields, RequestID: requestIDFromContext(c),
	})
}
