package apierr

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		apiErr   *APIError
		expected string
	}{
		{
			name: "error without underlying error",
			apiErr: &APIError{
				StatusCode: 400,
				Message:    "bad request",
			},
			expected: "API Error 400: bad request",
		},
		{
			name: "error with underlying error",
			apiErr: &APIError{
				StatusCode: 500,
				Message:    "internal error",
				Err:        errors.New("database connection failed"),
			},
			expected: "API Error 500: internal error (caused by: database connection failed)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.apiErr.Error())
		})
	}
}

func TestAPIError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	apiErr := Wrap(originalErr, 500, "wrapped error")
	
	assert.Equal(t, originalErr, apiErr.Unwrap())
}

func TestNew(t *testing.T) {
	err := New(404, "not found")
	
	assert.Equal(t, 404, err.StatusCode)
	assert.Equal(t, "not found", err.Message)
	assert.Nil(t, err.Err)
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original")
	err := Wrap(originalErr, 500, "wrapped")
	
	assert.Equal(t, 500, err.StatusCode)
	assert.Equal(t, "wrapped", err.Message)
	assert.Equal(t, originalErr, err.Err)
}

func TestWrapf(t *testing.T) {
	originalErr := errors.New("original")
	err := Wrapf(originalErr, 500, "wrapped with id: %d", 123)
	
	assert.Equal(t, 500, err.StatusCode)
	assert.Equal(t, "wrapped with id: 123", err.Message)
	assert.Equal(t, originalErr, err.Err)
}

func TestBadRequest(t *testing.T) {
	err := BadRequest("invalid json")
	
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
	assert.Equal(t, "invalid json", err.Message)
}

func TestBadRequestf(t *testing.T) {
	err := BadRequestf("invalid field: %s", "email")
	
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
	assert.Equal(t, "invalid field: email", err.Message)
}

func TestNotFound(t *testing.T) {
	err := NotFound("user not found")
	
	assert.Equal(t, http.StatusNotFound, err.StatusCode)
	assert.Equal(t, "user not found", err.Message)
}

func TestNotFoundf(t *testing.T) {
	err := NotFoundf("user %s not found", "john")
	
	assert.Equal(t, http.StatusNotFound, err.StatusCode)
	assert.Equal(t, "user john not found", err.Message)
}

func TestConflict(t *testing.T) {
	err := Conflict("resource already exists")
	
	assert.Equal(t, http.StatusConflict, err.StatusCode)
	assert.Equal(t, "resource already exists", err.Message)
}

func TestConflictf(t *testing.T) {
	err := Conflictf("task %s is not running", "123")
	
	assert.Equal(t, http.StatusConflict, err.StatusCode)
	assert.Equal(t, "task 123 is not running", err.Message)
}

func TestInternalError(t *testing.T) {
	err := InternalError("database error")
	
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
	assert.Equal(t, "database error", err.Message)
}

func TestInternalErrorf(t *testing.T) {
	err := InternalErrorf("failed to process request %d", 123)
	
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
	assert.Equal(t, "failed to process request 123", err.Message)
}

func TestWrapInternal(t *testing.T) {
	originalErr := errors.New("db error")
	err := WrapInternal(originalErr, "failed to save")
	
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
	assert.Equal(t, "failed to save", err.Message)
	assert.Equal(t, originalErr, err.Err)
}

func TestWrapInternalf(t *testing.T) {
	originalErr := errors.New("db error")
	err := WrapInternalf(originalErr, "failed to save user %s", "john")
	
	assert.Equal(t, http.StatusInternalServerError, err.StatusCode)
	assert.Equal(t, "failed to save user john", err.Message)
	assert.Equal(t, originalErr, err.Err)
}

func TestIsAPIError(t *testing.T) {
	apiErr := New(400, "bad request")
	regularErr := errors.New("regular error")
	
	assert.True(t, IsAPIError(apiErr))
	assert.False(t, IsAPIError(regularErr))
}

func TestGetStatusCode(t *testing.T) {
	apiErr := New(404, "not found")
	regularErr := errors.New("regular error")
	
	assert.Equal(t, 404, GetStatusCode(apiErr))
	assert.Equal(t, http.StatusInternalServerError, GetStatusCode(regularErr))
}

func TestGetMessage(t *testing.T) {
	apiErr := New(400, "validation failed")
	regularErr := errors.New("connection error")
	
	assert.Equal(t, "validation failed", GetMessage(apiErr))
	assert.Equal(t, "connection error", GetMessage(regularErr))
}
