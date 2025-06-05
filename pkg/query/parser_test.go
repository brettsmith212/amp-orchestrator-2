package query

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/brettsmith212/amp-orchestrator-2/pkg/apierr"
)

func TestParseTaskQuery_Defaults(t *testing.T) {
	values := url.Values{}
	query, err := ParseTaskQuery(values)
	require.NoError(t, err)

	assert.Equal(t, 50, query.Limit)
	assert.Equal(t, "", query.Cursor)
	assert.Empty(t, query.Status)
	assert.Nil(t, query.StartedBefore)
	assert.Nil(t, query.StartedAfter)
	assert.Equal(t, "started", query.SortBy)
	assert.Equal(t, "desc", query.SortOrder)
}

func TestParseTaskQuery_Limit(t *testing.T) {
	tests := []struct {
		name        string
		limit       string
		expected    int
		expectError bool
	}{
		{"valid limit", "25", 25, false},
		{"max limit", "100", 100, false},
		{"min limit", "1", 1, false},
		{"invalid limit", "abc", 0, true},
		{"zero limit", "0", 0, true},
		{"negative limit", "-5", 0, true},
		{"over max limit", "101", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := url.Values{"limit": {tt.limit}}
			query, err := ParseTaskQuery(values)

			if tt.expectError {
				assert.Error(t, err)
				assert.True(t, apierr.IsAPIError(err))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, query.Limit)
			}
		})
	}
}

func TestParseTaskQuery_Cursor(t *testing.T) {
	values := url.Values{"cursor": {"test_cursor_123"}}
	query, err := ParseTaskQuery(values)
	require.NoError(t, err)

	assert.Equal(t, "test_cursor_123", query.Cursor)
}

func TestParseTaskQuery_Status(t *testing.T) {
	tests := []struct {
		name        string
		status      string
		expected    []string
		expectError bool
	}{
		{"single status", "running", []string{"running"}, false},
		{"multiple statuses", "running,stopped", []string{"running", "stopped"}, false},
		{"with spaces", "running, stopped", []string{"running", "stopped"}, false},
		{"invalid status", "invalid", nil, true},
		{"mixed valid/invalid", "running,invalid", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := url.Values{"status": {tt.status}}
			query, err := ParseTaskQuery(values)

			if tt.expectError {
				assert.Error(t, err)
				assert.True(t, apierr.IsAPIError(err))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, query.Status)
			}
		})
	}
}

func TestParseTaskQuery_TimeFilters(t *testing.T) {
	validTime := "2025-06-04T16:00:00Z"
	invalidTime := "not-a-time"

	t.Run("valid started_before", func(t *testing.T) {
		values := url.Values{"started_before": {validTime}}
		query, err := ParseTaskQuery(values)
		require.NoError(t, err)

		expected, _ := time.Parse(time.RFC3339, validTime)
		assert.Equal(t, expected, *query.StartedBefore)
	})

	t.Run("valid started_after", func(t *testing.T) {
		values := url.Values{"started_after": {validTime}}
		query, err := ParseTaskQuery(values)
		require.NoError(t, err)

		expected, _ := time.Parse(time.RFC3339, validTime)
		assert.Equal(t, expected, *query.StartedAfter)
	})

	t.Run("invalid started_before", func(t *testing.T) {
		values := url.Values{"started_before": {invalidTime}}
		_, err := ParseTaskQuery(values)
		assert.Error(t, err)
		assert.True(t, apierr.IsAPIError(err))
	})

	t.Run("invalid started_after", func(t *testing.T) {
		values := url.Values{"started_after": {invalidTime}}
		_, err := ParseTaskQuery(values)
		assert.Error(t, err)
		assert.True(t, apierr.IsAPIError(err))
	})
}

func TestParseTaskQuery_Sorting(t *testing.T) {
	tests := []struct {
		name         string
		sortBy       string
		sortOrder    string
		expectedBy   string
		expectedOrder string
		expectError  bool
	}{
		{"valid sort by started", "started", "asc", "started", "asc", false},
		{"valid sort by status", "status", "desc", "status", "desc", false},
		{"valid sort by id", "id", "asc", "id", "asc", false},
		{"invalid sort by", "invalid", "asc", "", "", true},
		{"invalid sort order", "started", "invalid", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := url.Values{}
			if tt.sortBy != "" {
				values.Set("sort_by", tt.sortBy)
			}
			if tt.sortOrder != "" {
				values.Set("sort_order", tt.sortOrder)
			}

			query, err := ParseTaskQuery(values)

			if tt.expectError {
				assert.Error(t, err)
				assert.True(t, apierr.IsAPIError(err))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedBy, query.SortBy)
				assert.Equal(t, tt.expectedOrder, query.SortOrder)
			}
		})
	}
}

func TestGenerateCursor(t *testing.T) {
	testTime := time.Unix(1672531200, 0) // 2023-01-01 00:00:00 UTC
	testID := "abc123"

	cursor := GenerateCursor(testID, testTime)
	assert.Equal(t, "1672531200_abc123", cursor)
}

func TestParseCursor(t *testing.T) {
	t.Run("valid cursor", func(t *testing.T) {
		cursor := "1672531200_abc123"
		timestamp, id, err := ParseCursor(cursor)
		require.NoError(t, err)

		expectedTime := time.Unix(1672531200, 0)
		assert.Equal(t, expectedTime, timestamp)
		assert.Equal(t, "abc123", id)
	})

	t.Run("invalid cursor format", func(t *testing.T) {
		cursor := "invalid_cursor_format_too_many_parts"
		_, _, err := ParseCursor(cursor)
		assert.Error(t, err)
		assert.True(t, apierr.IsAPIError(err))
	})

	t.Run("invalid timestamp", func(t *testing.T) {
		cursor := "not_a_number_abc123"
		_, _, err := ParseCursor(cursor)
		assert.Error(t, err)
		assert.True(t, apierr.IsAPIError(err))
	})

	t.Run("missing parts", func(t *testing.T) {
		cursor := "1672531200"
		_, _, err := ParseCursor(cursor)
		assert.Error(t, err)
		assert.True(t, apierr.IsAPIError(err))
	})
}
