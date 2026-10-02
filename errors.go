package mista

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Sentinel errors for use with errors.Is. An *APIError matches the sentinel for
// its HTTP status, e.g. errors.Is(err, mista.ErrNotFound).
var (
	ErrBadRequest   = errors.New("mista: bad request")
	ErrUnauthorized = errors.New("mista: unauthorized")
	ErrForbidden    = errors.New("mista: forbidden")
	ErrNotFound     = errors.New("mista: not found")
	ErrValidation   = errors.New("mista: validation failed")
	ErrRateLimited  = errors.New("mista: rate limited")
	ErrServer       = errors.New("mista: server error")

	// ErrInvalidParams wraps client-side validation failures; no request is sent.
	ErrInvalidParams = errors.New("mista: invalid parameters")

	// ErrMissingToken is returned when no token was given and MISTA_API_TOKEN is unset.
	ErrMissingToken = errors.New("mista: missing API token; pass one to NewClient or set MISTA_API_TOKEN")
)

// FieldError is one field problem reported by the API.
type FieldError struct {
	Field   string
	Message string
	Type    string
}

// APIError is returned when the API answers with an error status, or with a
// 200 whose body says "status": "error" (for example a duplicate contact).
type APIError struct {
	StatusCode int
	Message    string
	Errors     []FieldError
	Body       []byte
	Header     http.Header
}

func (e *APIError) Error() string {
	return fmt.Sprintf("mista: %s (HTTP %d)", e.Message, e.StatusCode)
}

// Is reports whether the error matches one of the sentinel errors.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrBadRequest:
		return e.StatusCode == http.StatusBadRequest
	case ErrUnauthorized:
		return e.StatusCode == http.StatusUnauthorized
	case ErrForbidden:
		return e.StatusCode == http.StatusForbidden
	case ErrNotFound:
		return e.StatusCode == http.StatusNotFound
	case ErrValidation:
		return e.StatusCode == http.StatusUnprocessableEntity
	case ErrRateLimited:
		return e.StatusCode == http.StatusTooManyRequests
	case ErrServer:
		return e.StatusCode >= 500
	}
	return false
}

// RetryAfter returns the Retry-After header in seconds, or 0.
func (e *APIError) RetryAfter() int {
	n, err := strconv.Atoi(strings.TrimSpace(e.Header.Get("Retry-After")))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// ConnectionError wraps a network failure or timeout.
type ConnectionError struct {
	Err error
}

func (e *ConnectionError) Error() string { return "mista: connection error: " + e.Err.Error() }
func (e *ConnectionError) Unwrap() error { return e.Err }

// Timeout reports whether the request timed out.
func (e *ConnectionError) Timeout() bool {
	var t interface{ Timeout() bool }
	return errors.As(e.Err, &t) && t.Timeout()
}

type errorBody struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Detail  json.RawMessage `json:"detail"`
	Errors  json.RawMessage `json:"errors"`
}

func newAPIError(status int, body []byte, header http.Header) *APIError {
	e := &APIError{StatusCode: status, Body: body, Header: header}
	var parsed errorBody
	if json.Unmarshal(body, &parsed) == nil {
		e.Errors = parseFieldErrors(parsed)
		e.Message = parsed.Message
		if e.Message == "" {
			var detail string
			if json.Unmarshal(parsed.Detail, &detail) == nil {
				e.Message = detail
			}
		}
		if e.Message == "" && len(e.Errors) > 0 {
			first := e.Errors[0]
			e.Message = first.Message
			if first.Field != "" {
				e.Message = first.Field + ": " + first.Message
			}
		}
	} else if text := strings.TrimSpace(string(body)); text != "" {
		if len(text) > 300 {
			text = text[:300]
		}
		e.Message = text
	}
	if e.Message == "" {
		e.Message = fmt.Sprintf("request failed with status %d", status)
	}
	return e
}

func parseFieldErrors(body errorBody) []FieldError {
	var detail []struct {
		Type string        `json:"type"`
		Loc  []interface{} `json:"loc"`
		Msg  string        `json:"msg"`
	}
	if json.Unmarshal(body.Detail, &detail) == nil && len(detail) > 0 {
		out := make([]FieldError, 0, len(detail))
		for _, d := range detail {
			fe := FieldError{Message: d.Msg, Type: d.Type}
			if len(d.Loc) > 1 {
				fe.Field = fmt.Sprint(d.Loc[len(d.Loc)-1])
			}
			out = append(out, fe)
		}
		return out
	}
	var laravel map[string][]string
	if json.Unmarshal(body.Errors, &laravel) == nil && len(laravel) > 0 {
		var out []FieldError
		for field, messages := range laravel {
			for _, m := range messages {
				out = append(out, FieldError{Field: field, Message: m})
			}
		}
		return out
	}
	return nil
}
