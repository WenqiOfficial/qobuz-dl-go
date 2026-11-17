package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/imroc/req/v3"
)

var (
	ErrInvalidSecret   = errors.New("invalid app secret")
	ErrAppIDRejected   = errors.New("app id rejected")
	ErrTokenInvalid    = errors.New("user token is invalid or expired")
	ErrBadCredentials  = errors.New("invalid email or password")
	ErrForbidden       = errors.New("qobuz forbidden the request")
	ErrNotFound        = errors.New("qobuz resource not found")
	ErrRateLimited     = errors.New("qobuz rate limited the request")
	ErrTransient       = errors.New("temporary qobuz api failure")
	ErrInvalidResponse = errors.New("invalid qobuz api response")
)

type ErrorKind string

const (
	ErrorKindUnknown         ErrorKind = ""
	ErrorKindInvalidSecret   ErrorKind = "invalid_secret"
	ErrorKindAppIDRejected   ErrorKind = "app_id_rejected"
	ErrorKindTokenInvalid    ErrorKind = "token_invalid"
	ErrorKindBadCredentials  ErrorKind = "bad_credentials"
	ErrorKindForbidden       ErrorKind = "forbidden"
	ErrorKindNotFound        ErrorKind = "not_found"
	ErrorKindRateLimited     ErrorKind = "rate_limited"
	ErrorKindTransient       ErrorKind = "transient"
	ErrorKindInvalidResponse ErrorKind = "invalid_response"
)

type StatusError struct {
	Code     int
	Message  string
	Endpoint string
	Kind     ErrorKind
}

func (e *StatusError) Error() string {
	message := e.Message
	if message == "" {
		message = http.StatusText(e.Code)
	}
	if e.Endpoint == "" {
		return fmt.Sprintf("qobuz api returned %d: %s", e.Code, message)
	}
	return fmt.Sprintf("qobuz api %s returned %d: %s", e.Endpoint, e.Code, message)
}

func (e *StatusError) Unwrap() error {
	switch e.Kind {
	case ErrorKindInvalidSecret:
		return ErrInvalidSecret
	case ErrorKindAppIDRejected:
		return ErrAppIDRejected
	case ErrorKindTokenInvalid:
		return ErrTokenInvalid
	case ErrorKindBadCredentials:
		return ErrBadCredentials
	case ErrorKindForbidden:
		return ErrForbidden
	case ErrorKindNotFound:
		return ErrNotFound
	case ErrorKindRateLimited:
		return ErrRateLimited
	case ErrorKindTransient:
		return ErrTransient
	case ErrorKindInvalidResponse:
		return ErrInvalidResponse
	default:
		return nil
	}
}

func responseError(resp *req.Response, endpoint string) error {
	message := responseMessage(resp)
	return &StatusError{
		Code:     resp.StatusCode,
		Message:  message,
		Endpoint: endpoint,
		Kind:     classifyStatus(resp.StatusCode, message, endpoint),
	}
}

func responseMessage(resp *req.Response) string {
	var payload struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal([]byte(resp.String()), &payload); err == nil {
		if message := strings.TrimSpace(payload.Message); message != "" {
			return message
		}
		if message := strings.TrimSpace(payload.Error); message != "" {
			return message
		}
	}
	return http.StatusText(resp.StatusCode)
}

func classifyStatus(code int, message, endpoint string) ErrorKind {
	normalized := strings.ToLower(message)
	mentionsAppID := strings.Contains(normalized, "app id") ||
		strings.Contains(normalized, "app_id") ||
		strings.Contains(normalized, "application id")

	switch {
	case code == http.StatusBadRequest && strings.Contains(normalized, "invalid") && strings.Contains(normalized, "signature"):
		return ErrorKindInvalidSecret
	case (code == http.StatusBadRequest || code == http.StatusUnauthorized) && mentionsAppID:
		return ErrorKindAppIDRejected
	case code == http.StatusUnauthorized && endpoint == "user/login":
		if strings.Contains(normalized, "token") ||
			strings.Contains(normalized, "authentication is required") ||
			strings.Contains(normalized, "invalid app") {
			return ErrorKindTokenInvalid
		}
		return ErrorKindBadCredentials
	case code == http.StatusUnauthorized:
		return ErrorKindTokenInvalid
	case code == http.StatusForbidden:
		return ErrorKindForbidden
	case code == http.StatusNotFound:
		return ErrorKindNotFound
	case code == http.StatusTooManyRequests:
		return ErrorKindRateLimited
	case code >= http.StatusInternalServerError:
		return ErrorKindTransient
	default:
		return ErrorKindUnknown
	}
}

func invalidResponseError(endpoint, message string) error {
	return &StatusError{
		Code:     http.StatusOK,
		Message:  message,
		Endpoint: endpoint,
		Kind:     ErrorKindInvalidResponse,
	}
}

func isTransportOrServerError(err error) bool {
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		return true
	}
	return statusErr.Code >= http.StatusInternalServerError
}

func preventsQualityFallback(err error) bool {
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	if statusErr.Kind == ErrorKindInvalidSecret || statusErr.Kind == ErrorKindAppIDRejected {
		return true
	}
	return statusErr.Code == http.StatusUnauthorized ||
		statusErr.Code == http.StatusForbidden ||
		statusErr.Code == http.StatusTooManyRequests
}
