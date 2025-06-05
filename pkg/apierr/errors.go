package apierr

import (
	"fmt"
	"net/http"
)

// APIError represents an API error with HTTP status code and message
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Err        error  `json:"-"` // Don't serialize the underlying error
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("API Error %d: %s (caused by: %v)", e.StatusCode, e.Message, e.Err)
	}
	return fmt.Sprintf("API Error %d: %s", e.StatusCode, e.Message)
}

// Unwrap returns the underlying error for error wrapping
func (e *APIError) Unwrap() error {
	return e.Err
}

// New creates a new API error
func New(statusCode int, message string) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Message:    message,
	}
}

// Wrap wraps an existing error with API error information
func Wrap(err error, statusCode int, message string) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Message:    message,
		Err:        err,
	}
}

// Wrapf wraps an error with a formatted message
func Wrapf(err error, statusCode int, format string, args ...interface{}) *APIError {
	return Wrap(err, statusCode, fmt.Sprintf(format, args...))
}

// Common error constructors
func BadRequest(message string) *APIError {
	return New(http.StatusBadRequest, message)
}

func BadRequestf(format string, args ...interface{}) *APIError {
	return New(http.StatusBadRequest, fmt.Sprintf(format, args...))
}

func NotFound(message string) *APIError {
	return New(http.StatusNotFound, message)
}

func NotFoundf(format string, args ...interface{}) *APIError {
	return New(http.StatusNotFound, fmt.Sprintf(format, args...))
}

func Conflict(message string) *APIError {
	return New(http.StatusConflict, message)
}

func Conflictf(format string, args ...interface{}) *APIError {
	return New(http.StatusConflict, fmt.Sprintf(format, args...))
}

func InternalError(message string) *APIError {
	return New(http.StatusInternalServerError, message)
}

func InternalErrorf(format string, args ...interface{}) *APIError {
	return New(http.StatusInternalServerError, fmt.Sprintf(format, args...))
}

func WrapInternal(err error, message string) *APIError {
	return Wrap(err, http.StatusInternalServerError, message)
}

func WrapInternalf(err error, format string, args ...interface{}) *APIError {
	return Wrapf(err, http.StatusInternalServerError, format, args...)
}

// IsAPIError checks if an error is an APIError
func IsAPIError(err error) bool {
	_, ok := err.(*APIError)
	return ok
}

// GetStatusCode extracts the status code from an error, defaulting to 500
func GetStatusCode(err error) int {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode
	}
	return http.StatusInternalServerError
}

// GetMessage extracts the message from an error
func GetMessage(err error) string {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Message
	}
	return err.Error()
}
