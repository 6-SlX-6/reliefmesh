// Package httpx contains HTTP helpers shared by all handlers: JSON decoding
// with strict limits, JSON responses and error mapping.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
)

// ErrorBody is the JSON error envelope returned by every endpoint.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail describes an error in a stable, machine-readable way.
type ErrorDetail struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	Details   map[string]any    `json:"details,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

// JSON writes v with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}

// NoContent writes 204.
func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// RequestIDFunc extracts the request id for error bodies; set by the server.
var RequestIDFunc = func(r *http.Request) string { return "" }

// Error maps err to an HTTP response. Internal errors are logged with their
// cause and reported generically.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	ae, ok := apperr.As(err)
	if !ok {
		ae = apperr.Internal(err)
	}
	status := statusFor(ae.Kind)
	if status >= 500 {
		slog.ErrorContext(r.Context(), "request failed", "error", err.Error(), "request_id", RequestIDFunc(r))
	}
	JSON(w, status, ErrorBody{Error: ErrorDetail{
		Code:      ae.Code,
		Message:   ae.Message,
		Fields:    ae.Fields,
		Details:   ae.Details,
		RequestID: RequestIDFunc(r),
	}})
}

func statusFor(k apperr.Kind) int {
	switch k {
	case apperr.KindBadRequest:
		return http.StatusBadRequest
	case apperr.KindValidation:
		return http.StatusUnprocessableEntity
	case apperr.KindUnauthorized:
		return http.StatusUnauthorized
	case apperr.KindForbidden:
		return http.StatusForbidden
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindConflict:
		return http.StatusConflict
	case apperr.KindTooManyRequests:
		return http.StatusTooManyRequests
	case apperr.KindUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// Decode strictly decodes a JSON body into dst: unknown fields and trailing
// data are rejected, and the body size is limited by the server middleware.
func Decode(r *http.Request, dst any) error {
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		return apperr.BadRequest("unsupported_media_type", "Content-Type must be application/json.")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			return apperr.BadRequest("body_too_large", "Request body is too large.")
		case errors.Is(err, io.EOF):
			return apperr.BadRequest("empty_body", "Request body must not be empty.")
		default:
			var se *json.SyntaxError
			var te *json.UnmarshalTypeError
			if errors.As(err, &te) {
				return apperr.ValidationField(te.Field, "Has the wrong type.")
			}
			if errors.As(err, &se) {
				return apperr.BadRequest("invalid_json", "Request body is not valid JSON.")
			}
			if strings.HasPrefix(err.Error(), "json: unknown field ") {
				return apperr.BadRequest("unknown_field", "Unknown field "+strings.TrimPrefix(err.Error(), "json: unknown field ")+".")
			}
			return apperr.BadRequest("invalid_json", "Request body could not be decoded.")
		}
	}
	if dec.More() {
		return apperr.BadRequest("invalid_json", "Request body must contain a single JSON object.")
	}
	return nil
}

// UUIDParam parses a UUID path parameter.
func UUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, apperr.NotFound("Not found.")
	}
	return id, nil
}

// IntQuery parses an optional bounded integer query parameter.
func IntQuery(r *http.Request, name string, def, min, max int) (int, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < min || n > max {
		return 0, apperr.ValidationField(name, fmt.Sprintf("Must be an integer between %d and %d.", min, max))
	}
	return n, nil
}
