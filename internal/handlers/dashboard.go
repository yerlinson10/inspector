package handlers

import (
	"net/http"
	"strconv"
	"time"

	"inspector/internal/models"
	"inspector/internal/storage"

	"github.com/gin-gonic/gin"
)

func Dashboard(c *gin.Context) {
	var endpoints []models.Endpoint
	storage.DB.Order("created_at DESC").Find(&endpoints)

	type EndpointStat struct {
		models.Endpoint
		TotalRequests int64
		LastRequest   *time.Time
	}

	var stats []EndpointStat
	for _, ep := range endpoints {
		var count int64
		storage.DB.Model(&models.RequestLog{}).Where("endpoint_id = ?", ep.ID).Count(&count)

		var lastReq models.RequestLog
		var lastTime *time.Time
		if err := storage.DB.Where("endpoint_id = ?", ep.ID).Order("created_at DESC").First(&lastReq).Error; err == nil {
			lastTime = &lastReq.CreatedAt
		}

		stats = append(stats, EndpointStat{
			Endpoint:      ep,
			TotalRequests: count,
			LastRequest:   lastTime,
		})
	}

	var recentRequests []models.RequestLog
	storage.DB.Order("created_at DESC").Limit(20).Find(&recentRequests)

	var latestRequestID uint
	if len(recentRequests) > 0 {
		latestRequestID = recentRequests[0].ID
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"ContentTemplate": "dashboard_content",
		"endpoints":       stats,
		"recentRequests":  recentRequests,
		"latestRequestID": latestRequestID,
		"title":           "Dashboard",
	})
}

func ListRequests(c *gin.Context) {
	reqType := c.Query("type")
	endpointSlug := c.Query("endpoint")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage := 50

	query := storage.DB.Model(&models.RequestLog{})

	if reqType != "" {
		query = query.Where("type = ?", reqType)
	}
	if endpointSlug != "" {
		query = query.Where("endpoint_slug = ?", endpointSlug)
	}

	var total int64
	query.Count(&total)

	var requests []models.RequestLog
	query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&requests)

	var latestRequestID uint
	if len(requests) > 0 {
		latestRequestID = requests[0].ID
	}

	var endpoints []models.Endpoint
	storage.DB.Order("name ASC").Find(&endpoints)

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	c.HTML(http.StatusOK, "requests.html", gin.H{
		"ContentTemplate": "requests_content",
		"requests":        requests,
		"endpoints":       endpoints,
		"currentType":     reqType,
		"currentSlug":     endpointSlug,
		"page":            page,
		"totalPages":      totalPages,
		"total":           total,
		"latestRequestID": latestRequestID,
		"title":           "Requests",
	})
}

func RequestDetail(c *gin.Context) {
	id := c.Param("id")

	var req models.RequestLog
	if err := storage.DB.First(&req, id).Error; err != nil {
		c.String(http.StatusNotFound, "Request not found")
		return
	}

	c.HTML(http.StatusOK, "request_detail.html", gin.H{
		"ContentTemplate": "request_detail_content",
		"request":         req,
		"title":           "Request #" + id,
	})
}
