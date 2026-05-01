// Package utils provides HTTP response helpers, the ApiResponse envelope,
// environment utilities, and common type conversions used throughout the application.
package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ── ApiResponse ───────────────────────────────────────────────────────────────

// ApiResponse is the single response envelope returned by every service method
// and sent by every handler. Services build it; handlers call SendResponse().
//
// JSON shape:
//
//	{ "success": true, "message": "...", "data": {...}, "status_code": 200, "request_id": "uuid" }
//	{ "success": false, "message": "...", "errors": {...}, "status_code": 4xx, "request_id": "uuid" }
type ApiResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	Data       any    `json:"data,omitempty"`
	Errors     any    `json:"errors,omitempty"`
	StatusCode int    `json:"status_code"`
	RequestID  string `json:"request_id,omitempty"`
}

// ApiSuccess builds a successful ApiResponse.
func ApiSuccess(message string, data any, statusCode int) ApiResponse {
	return ApiResponse{Success: true, Message: message, Data: data, StatusCode: statusCode}
}

// ApiError builds an error ApiResponse. errs may be nil, a string, or any serialisable value.
func ApiError(message string, errs any, statusCode int) ApiResponse {
	return ApiResponse{Success: false, Message: message, Errors: errs, StatusCode: statusCode}
}

// SendResponse writes an ApiResponse to the http.ResponseWriter, injecting the
// X-Request-ID header value into the response body for traceability.
func SendResponse(w http.ResponseWriter, r ApiResponse) {
	r.RequestID = w.Header().Get("X-Request-ID")
	w.Header().Set(HeaderContentTypeName, HeaderContentTypeJSON)
	w.WriteHeader(r.StatusCode)
	json.NewEncoder(w).Encode(r) //nolint:errcheck
}

// Common Content-Type header values.
const (
	HeaderContentTypeName = "Content-Type"
	HeaderContentTypeJSON = "application/json"
	HeaderContentTypeText = "text/plain; charset=utf-8"
)

// M is a shorthand for map[string]any — used for ad-hoc JSON response bodies.
type M map[string]any

// ResponseWriter wraps http.ResponseWriter to capture the status code written by handlers.
// This is required by the logger middleware which needs to log the final status code.
type ResponseWriter struct {
	http.ResponseWriter
	StatusCode int
}

// WriteHeader captures the status code before delegating to the underlying writer.
func (rw *ResponseWriter) WriteHeader(code int) {
	rw.StatusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// CookieParams groups all standard http.Cookie fields for convenient cookie creation.
type CookieParams struct {
	Name     string
	Value    string
	MaxAge   int
	Path     string
	Domain   string
	Secure   bool
	HTTPOnly bool
	SameSite http.SameSite
}

// HttpWriter wraps http.ResponseWriter and *http.Request to provide a fluent, chainable
// API for writing JSON, text, and error responses.
type HttpWriter struct {
	W          http.ResponseWriter
	R          *http.Request
	StatusCode int
}

// NewHttpWriter creates an HttpWriter with a default 200 OK status.
func NewHttpWriter(w http.ResponseWriter, r *http.Request) *HttpWriter {
	return &HttpWriter{
		W:          w,
		R:          r,
		StatusCode: http.StatusOK,
	}
}

// Status sets the HTTP status code and returns the HttpWriter for chaining.
func (hw *HttpWriter) Status(code int) *HttpWriter {
	hw.StatusCode = code
	return hw
}

// JSON serialises data to JSON, injects the X-Request-ID into the payload, and writes
// the response. On marshal failure it falls back to a 500 plain-text response.
func (hw *HttpWriter) JSON(data M) {
	// Inject request ID for traceability
	if rid := hw.W.Header().Get("X-Request-ID"); rid != "" {
		data["request_id"] = rid
	} else {
		data["request_id"] = "unknown"
	}

	b, err := json.Marshal(data)
	if err != nil {
		hw.W.Header().Set(HeaderContentTypeName, HeaderContentTypeText)
		hw.W.WriteHeader(http.StatusInternalServerError)
		hw.W.Write([]byte("failed to marshal response")) //nolint:errcheck
		return
	}

	hw.W.Header().Set(HeaderContentTypeName, HeaderContentTypeJSON)
	hw.W.WriteHeader(hw.StatusCode)
	hw.W.Write(b) //nolint:errcheck
}

// Text writes a plain-text response, appending the request ID for traceability.
func (hw *HttpWriter) Text(text string) {
	hw.W.Header().Set(HeaderContentTypeName, HeaderContentTypeText)
	hw.W.WriteHeader(hw.StatusCode)

	rid := hw.W.Header().Get("X-Request-ID")
	hw.W.Write([]byte(strings.Join([]string{text, "RequestId=" + rid}, ";"))) //nolint:errcheck
}

// Error writes an error response. If no explicit status code has been set via Status(),
// it defaults to 500 Internal Server Error.
func (hw *HttpWriter) Error(err error, statusCode ...int) {
	if hw.StatusCode == http.StatusOK {
		if len(statusCode) > 0 && statusCode[0] >= 400 {
			hw.StatusCode = statusCode[0]
		} else {
			hw.StatusCode = http.StatusInternalServerError
		}
	}

	hw.W.Header().Set(HeaderContentTypeName, HeaderContentTypeText)
	hw.W.WriteHeader(hw.StatusCode)

	rid := hw.W.Header().Get("X-Request-ID")
	hw.W.Write([]byte(err.Error() + ";RequestId=" + rid)) //nolint:errcheck
}

// ParseBody decodes the JSON request body into body (must be a pointer).
// Returns an error if the body is absent, the Content-Type is wrong, or decoding fails.
func (hw *HttpWriter) ParseBody(body any) error {
	return ParseBody(hw.R, body)
}

// ParseBody is a package-level helper that decodes the JSON request body into dest.
// Use this in module route handlers instead of going through HttpWriter.
func ParseBody(r *http.Request, dest any) error {
	if r.Body == nil {
		return errors.New("request has no body")
	}
	ct := r.Header.Get(HeaderContentTypeName)
	if ct == "" || !strings.Contains(ct, "application/json") {
		return errors.New("Content-Type must be application/json")
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		return fmt.Errorf("failed to decode JSON body: %w", err)
	}
	return nil
}

// SetCookie writes a Set-Cookie header using the provided parameters.
func (hw *HttpWriter) SetCookie(p CookieParams) {
	http.SetCookie(hw.W, &http.Cookie{
		Name:     p.Name,
		Value:    p.Value,
		MaxAge:   p.MaxAge,
		Path:     p.Path,
		Domain:   p.Domain,
		Secure:   p.Secure,
		HttpOnly: p.HTTPOnly,
		SameSite: p.SameSite,
	})
}
