package handlers

import (
	"net/http"
	"strconv"
	"time"

	"inspector/internal/models"
	"inspector/internal/storage"

	"github.com/gin-gonic/gin"
)

type pollRequestEvent struct {
	ID           uint      `json:"id"`
	EndpointSlug string    `json:"endpoint_slug"`
	Type         string    `json:"type"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	RemoteAddr   string    `json:"remote_addr"`
	SizeBytes    int64     `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`
}

type pollSentEvent struct {
	ID             uint      `json:"id"`
	Type           string    `json:"type"`
	Method         string    `json:"method"`
	URL            string    `json:"url"`
	ResponseStatus int       `json:"response_status"`
	Error          string    `json:"error"`
	DurationMs     int64     `json:"duration_ms"`
	CreatedAt      time.Time `json:"created_at"`
}

func EventsPoll(c *gin.Context) {
	sinceRequestID, _ := strconv.ParseUint(c.DefaultQuery("since_request_id", "0"), 10, 64)
	sinceSentRequestID, _ := strconv.ParseUint(c.DefaultQuery("since_sent_request_id", "0"), 10, 64)

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}

	var newRequests []pollRequestEvent
	storage.DB.
		Model(&models.RequestLog{}).
		Select("id, endpoint_slug, type, method, path, remote_addr, size_bytes, created_at").
		Where("id > ?", sinceRequestID).
		Order("id ASC").
		Limit(limit).
		Find(&newRequests)

	var newSentRequests []pollSentEvent
	storage.DB.
		Model(&models.SentRequest{}).
		Select("id, type, method, url, response_status, error, duration_ms, created_at").
		Where("id > ?", sinceSentRequestID).
		Order("id ASC").
		Limit(limit).
		Find(&newSentRequests)

	latestRequestID := uint(sinceRequestID)
	if len(newRequests) > 0 {
		latestRequestID = newRequests[len(newRequests)-1].ID
	} else {
		storage.DB.Model(&models.RequestLog{}).Select("COALESCE(MAX(id), 0)").Scan(&latestRequestID)
	}

	latestSentRequestID := uint(sinceSentRequestID)
	if len(newSentRequests) > 0 {
		latestSentRequestID = newSentRequests[len(newSentRequests)-1].ID
	} else {
		storage.DB.Model(&models.SentRequest{}).Select("COALESCE(MAX(id), 0)").Scan(&latestSentRequestID)
	}

	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	c.JSON(http.StatusOK, gin.H{
		"new_requests":           newRequests,
		"new_sent_requests":      newSentRequests,
		"latest_request_id":      latestRequestID,
		"latest_sent_request_id": latestSentRequestID,
	})
}
