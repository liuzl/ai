package ai

import (
	"fmt"
	"time"
)

// ErrorWithStatus is the base interface for all AI client errors.
// It provides access to the HTTP status code and allows for error type assertions.
type ErrorWithStatus interface {
	error
	StatusCode() int
	Provider() string
	Unwrap() error
}

// baseError is the common implementation for all errors.
type baseError struct {
	statusCode int
	provider   string
	message    string
	err        error
}

func (e *baseError) Error() string {
	if e.provider != "" {
		return fmt.Sprintf("[%s] %s", e.provider, e.message)
	}
	return e.message
}

func (e *baseError) StatusCode() int {
	return e.statusCode
}

func (e *baseError) Provider() string {
	return e.provider
}

func (e *baseError) Unwrap() error {
	return e.err
}

// AuthenticationError represents authentication failures (401, 403).
type AuthenticationError struct {
	baseError
}

// NewAuthenticationError creates a new authentication error.
func NewAuthenticationError(provider string, statusCode int, message string, err error) *AuthenticationError {
	return &AuthenticationError{
		baseError: baseError{
			statusCode: statusCode,
			provider:   provider,
			message:    fmt.Sprintf("authentication failed: %s", message),
			err:        err,
		},
	}
}

// RateLimitError represents rate limiting errors (429).
type RateLimitError struct {
	baseError
	RetryAfter time.Duration
}

// NewRateLimitError creates a new rate limit error.
func NewRateLimitError(provider string, message string, retryAfter time.Duration, err error) *RateLimitError {
	msg := fmt.Sprintf("rate limit exceeded: %s", message)
	if retryAfter > 0 {
		msg = fmt.Sprintf("%s (retry after %v)", msg, retryAfter)
	}
	return &RateLimitError{
		baseError: baseError{
			statusCode: 429,
			provider:   provider,
			message:    msg,
			err:        err,
		},
		RetryAfter: retryAfter,
	}
}

// InvalidRequestError represents invalid request errors (400).
type InvalidRequestError struct {
	baseError
	Details string
}

// NewInvalidRequestError creates a new invalid request error.
func NewInvalidRequestError(provider string, message string, details string, err error) *InvalidRequestError {
	msg := fmt.Sprintf("invalid request: %s", message)
	if details != "" {
		msg = fmt.Sprintf("%s - %s", msg, details)
	}
	return &InvalidRequestError{
		baseError: baseError{
			statusCode: 400,
			provider:   provider,
			message:    msg,
			err:        err,
		},
		Details: details,
	}
}

// ServerError represents server-side errors (5xx).
type ServerError struct {
	baseError
}

// NewServerError creates a new server error.
func NewServerError(provider string, statusCode int, message string, err error) *ServerError {
	return &ServerError{
		baseError: baseError{
			statusCode: statusCode,
			provider:   provider,
			message:    fmt.Sprintf("server error: %s", message),
			err:        err,
		},
	}
}

// NetworkError represents network-level failures (connection refused, DNS, etc.).
type NetworkError struct {
	baseError
}

// NewNetworkError creates a new network error.
func NewNetworkError(provider string, message string, err error) *NetworkError {
	return &NetworkError{
		baseError: baseError{
			statusCode: 0, // No HTTP status for network errors
			provider:   provider,
			message:    fmt.Sprintf("network error: %s", message),
			err:        err,
		},
	}
}

// TimeoutError represents timeout errors (context deadline exceeded).
type TimeoutError struct {
	baseError
	Duration time.Duration
}

// NewTimeoutError creates a new timeout error.
func NewTimeoutError(provider string, duration time.Duration, err error) *TimeoutError {
	return &TimeoutError{
		baseError: baseError{
			statusCode: 0, // No HTTP status for timeout errors
			provider:   provider,
			message:    fmt.Sprintf("request timeout after %v", duration),
			err:        err,
		},
		Duration: duration,
	}
}

// UnknownError represents unexpected errors that don't fit other categories.
type UnknownError struct {
	baseError
}

// NewUnknownError creates a new unknown error.
func NewUnknownError(provider string, statusCode int, message string, err error) *UnknownError {
	return &UnknownError{
		baseError: baseError{
			statusCode: statusCode,
			provider:   provider,
			message:    fmt.Sprintf("unknown error: %s", message),
			err:        err,
		},
	}
}
