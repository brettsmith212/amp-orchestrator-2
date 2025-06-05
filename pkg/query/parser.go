package query

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/brettsmith212/amp-orchestrator-2/pkg/apierr"
)

// TaskQuery represents query parameters for task listing
type TaskQuery struct {
	// Pagination
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor,omitempty"`

	// Filtering
	Status    []string   `json:"status,omitempty"`
	StartedBefore *time.Time `json:"started_before,omitempty"`
	StartedAfter  *time.Time `json:"started_after,omitempty"`

	// Sorting
	SortBy    string `json:"sort_by"`
	SortOrder string `json:"sort_order"`
}

// ParseTaskQuery parses URL query parameters into a TaskQuery struct
func ParseTaskQuery(values url.Values) (*TaskQuery, error) {
	query := &TaskQuery{
		Limit:     50, // Default limit
		SortBy:    "started",
		SortOrder: "desc",
	}

	// Parse limit
	if limitStr := values.Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return nil, apierr.BadRequest("Invalid limit parameter")
		}
		if limit < 1 {
			return nil, apierr.BadRequest("Limit must be greater than 0")
		}
		if limit > 100 {
			return nil, apierr.BadRequest("Limit cannot exceed 100")
		}
		query.Limit = limit
	}

	// Parse cursor
	if cursor := values.Get("cursor"); cursor != "" {
		query.Cursor = cursor
	}

	// Parse status filter
	if statusStr := values.Get("status"); statusStr != "" {
		rawStatuses := strings.Split(statusStr, ",")
		var statuses []string
		for _, status := range rawStatuses {
			status = strings.TrimSpace(status)
			if status != "running" && status != "stopped" {
				return nil, apierr.BadRequestf("Invalid status filter: %s", status)
			}
			statuses = append(statuses, status)
		}
		query.Status = statuses
	}

	// Parse started_before
	if beforeStr := values.Get("started_before"); beforeStr != "" {
		before, err := time.Parse(time.RFC3339, beforeStr)
		if err != nil {
			return nil, apierr.BadRequest("Invalid started_before format, use RFC3339")
		}
		query.StartedBefore = &before
	}

	// Parse started_after
	if afterStr := values.Get("started_after"); afterStr != "" {
		after, err := time.Parse(time.RFC3339, afterStr)
		if err != nil {
			return nil, apierr.BadRequest("Invalid started_after format, use RFC3339")
		}
		query.StartedAfter = &after
	}

	// Parse sort_by
	if sortBy := values.Get("sort_by"); sortBy != "" {
		if sortBy != "started" && sortBy != "status" && sortBy != "id" {
			return nil, apierr.BadRequestf("Invalid sort_by parameter: %s", sortBy)
		}
		query.SortBy = sortBy
	}

	// Parse sort_order
	if sortOrder := values.Get("sort_order"); sortOrder != "" {
		if sortOrder != "asc" && sortOrder != "desc" {
			return nil, apierr.BadRequestf("Invalid sort_order parameter: %s", sortOrder)
		}
		query.SortOrder = sortOrder
	}

	return query, nil
}

// GenerateCursor creates a cursor string for pagination
func GenerateCursor(id string, started time.Time) string {
	// Simple cursor format: timestamp_id
	return fmt.Sprintf("%d_%s", started.Unix(), id)
}

// ParseCursor extracts timestamp and ID from cursor string
func ParseCursor(cursor string) (time.Time, string, error) {
	parts := strings.SplitN(cursor, "_", 2)
	if len(parts) != 2 {
		return time.Time{}, "", apierr.BadRequest("Invalid cursor format")
	}

	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, "", apierr.BadRequest("Invalid cursor timestamp")
	}

	return time.Unix(timestamp, 0), parts[1], nil
}
