package response

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSON(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		payload        interface{}
		expectedStatus int
		expectedBody   string
		expectedHeader string
	}{
		{
			name:           "success with payload",
			statusCode:     200,
			payload:        map[string]string{"message": "hello"},
			expectedStatus: 200,
			expectedBody:   `{"message":"hello"}`,
			expectedHeader: "application/json",
		},
		{
			name:           "created with payload",
			statusCode:     201,
			payload:        map[string]interface{}{"id": "123", "status": "created"},
			expectedStatus: 201,
			expectedBody:   `{"id":"123","status":"created"}`,
			expectedHeader: "application/json",
		},
		{
			name:           "nil payload",
			statusCode:     200,
			payload:        nil,
			expectedStatus: 200,
			expectedBody:   "",
			expectedHeader: "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			err := JSON(w, tt.statusCode, tt.payload)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedHeader, w.Header().Get("Content-Type"))
			
			body := strings.TrimSpace(w.Body.String())
			assert.Equal(t, tt.expectedBody, body)
		})
	}
}

func TestOK(t *testing.T) {
	w := httptest.NewRecorder()
	payload := map[string]string{"status": "ok"}
	
	err := OK(w, payload)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
}

func TestCreated(t *testing.T) {
	w := httptest.NewRecorder()
	payload := map[string]string{"id": "123"}
	
	err := Created(w, payload)
	require.NoError(t, err)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), `"id":"123"`)
}

func TestAccepted(t *testing.T) {
	w := httptest.NewRecorder()
	
	err := Accepted(w, nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	
	NoContent(w)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestText(t *testing.T) {
	w := httptest.NewRecorder()
	
	Text(w, http.StatusOK, "hello world")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "hello world", w.Body.String())
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()
	
	Error(w, http.StatusBadRequest, "invalid input")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "invalid input", w.Body.String())
}
