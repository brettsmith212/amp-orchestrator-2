package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/brettsmith212/amp-orchestrator-2/internal/worker"
	"github.com/brettsmith212/amp-orchestrator-2/pkg/response"
)

// GetTaskThread returns the thread messages for a specific task
func GetTaskThread(wm *worker.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskID := chi.URLParam(r, "id")
		if taskID == "" {
			response.Error(w, http.StatusBadRequest, "task ID is required")
			return
		}

		// Parse pagination parameters
		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")

		limit := 50 // Default limit
		if limitStr != "" {
			if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
				limit = parsedLimit
				if limit > 100 {
					limit = 100 // Cap at 100
				}
			}
		}

		offset := 0 // Default offset
		if offsetStr != "" {
			if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
				offset = parsedOffset
			}
		}

		// Get total count first
		total, err := wm.CountThreadMessages(taskID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to count thread messages")
			return
		}

		// Get messages
		messages, err := wm.GetThreadMessages(taskID, limit, offset)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to retrieve thread messages")
			return
		}

		// Convert to DTOs
		messageDTOs := make([]ThreadMessageDTO, len(messages))
		for i, msg := range messages {
			messageDTOs[i] = ThreadMessageDTO{
				ID:        msg.ID,
				Type:      string(msg.Type),
				Content:   msg.Content,
				Timestamp: msg.Timestamp,
				Metadata:  msg.Metadata,
			}
		}

		// Calculate has_more
		hasMore := offset+len(messages) < total

		responseData := PaginatedThreadResponse{
			Messages: messageDTOs,
			HasMore:  hasMore,
			Total:    total,
		}

		response.JSON(w, http.StatusOK, responseData)
	}
}
