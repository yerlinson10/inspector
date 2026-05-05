package handlers

import (
	"net/http"
	"strconv"

	"inspector/internal/models"
	"inspector/internal/storage"

	"github.com/gin-gonic/gin"
)

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

	var newRequests []models.RequestLog
	storage.DB.
		Where("id > ?", sinceRequestID).
		Order("id ASC").
		Limit(limit).
		Find(&newRequests)

	var newSentRequests []models.SentRequest
	storage.DB.
		Where("id > ?", sinceSentRequestID).
		Order("id ASC").
		Limit(limit).
		Find(&newSentRequests)

	var latestRequestID uint
	storage.DB.Model(&models.RequestLog{}).Select("COALESCE(MAX(id), 0)").Scan(&latestRequestID)

	var latestSentRequestID uint
	storage.DB.Model(&models.SentRequest{}).Select("COALESCE(MAX(id), 0)").Scan(&latestSentRequestID)

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
