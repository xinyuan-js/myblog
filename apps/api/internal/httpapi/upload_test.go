package httpapi

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/myblog/apps/api/internal/upload"
	"github.com/gin-gonic/gin"
)

type multipartTestPart struct {
	field    string
	filename string
	body     string
}

func TestDecodeUploadInputRequiresExactlyOneFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		parts     []multipartTestPart
		maximum   int64
		wantBody  string
		wantError error
	}{
		{
			name:     "valid file",
			parts:    []multipartTestPart{{field: "file", filename: "image.png", body: "image"}},
			maximum:  16,
			wantBody: "image",
		},
		{
			name:      "wrong field",
			parts:     []multipartTestPart{{field: "other", filename: "image.png", body: "image"}},
			maximum:   16,
			wantError: errInvalidUploadForm,
		},
		{
			name:      "missing file",
			maximum:   16,
			wantError: errInvalidUploadForm,
		},
		{
			name: "extra value",
			parts: []multipartTestPart{
				{field: "file", filename: "image.png", body: "image"},
				{field: "description", body: "ignored"},
			},
			maximum:   16,
			wantError: errInvalidUploadForm,
		},
		{
			name: "second file",
			parts: []multipartTestPart{
				{field: "file", filename: "image.png", body: "image"},
				{field: "file", filename: "second.png", body: "second"},
			},
			maximum:   16,
			wantError: errInvalidUploadForm,
		},
		{
			name:      "oversized file",
			parts:     []multipartTestPart{{field: "file", filename: "image.png", body: strings.Repeat("a", 17)}},
			maximum:   16,
			wantError: upload.ErrTooLarge,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := multipartRequest(t, test.parts)
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = request

			input, err := decodeUploadInput(context, test.maximum)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("error = %v, want %v", err, test.wantError)
			}
			if test.wantError == nil {
				if input.Filename != "image.png" || string(input.Body) != test.wantBody {
					t.Fatalf("input = %+v", input)
				}
			}
		})
	}
}

func multipartRequest(t *testing.T, parts []multipartTestPart) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, value := range parts {
		var (
			destination io.Writer
			err         error
		)
		if value.filename == "" {
			destination, err = writer.CreateFormField(value.field)
		} else {
			destination, err = writer.CreateFormFile(value.field, value.filename)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := destination.Write([]byte(value.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}
