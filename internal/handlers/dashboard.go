package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"inspector/internal/models"
	"inspector/internal/storage"

	"github.com/gin-gonic/gin"
)

func Dashboard(c *gin.Context) {
	type EndpointStat struct {
		models.Endpoint
		TotalRequests int64
		LastRequest   *time.Time
	}

	type endpointStatRow struct {
		ID              uint
		Name            string
		Slug            string
		Description     string
		ResponseStatus  int
		ResponseHeaders string
		ResponseBody    string
		CreatedAt       time.Time
		TotalRequests   int64
		LastRequest     sql.NullTime
	}

	var rows []endpointStatRow
	storage.DB.Table("endpoints").
		Select("endpoints.id, endpoints.name, endpoints.slug, endpoints.description, endpoints.response_status, endpoints.response_headers, endpoints.response_body, endpoints.created_at, COUNT(request_logs.id) as total_requests, MAX(request_logs.created_at) as last_request").
		Joins("LEFT JOIN request_logs ON request_logs.endpoint_id = endpoints.id").
		Group("endpoints.id").
		Order("endpoints.created_at DESC").
		Scan(&rows)

	stats := make([]EndpointStat, 0, len(rows))
	for _, row := range rows {
		var lastRequest *time.Time
		if row.LastRequest.Valid {
			last := row.LastRequest.Time
			lastRequest = &last
		}

		stats = append(stats, EndpointStat{
			Endpoint: models.Endpoint{
				ID:              row.ID,
				Name:            row.Name,
				Slug:            row.Slug,
				Description:     row.Description,
				ResponseStatus:  row.ResponseStatus,
				ResponseHeaders: row.ResponseHeaders,
				ResponseBody:    row.ResponseBody,
				CreatedAt:       row.CreatedAt,
			},
			TotalRequests: row.TotalRequests,
			LastRequest:   lastRequest,
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
		"publicWSBaseURL": requestWSBaseURL(c),
		"title":           "Dashboard",
	})
}

func ListRequests(c *gin.Context) {
	reqType := strings.TrimSpace(c.Query("type"))
	endpointSlug := strings.TrimSpace(c.Query("endpoint"))
	method := strings.ToUpper(strings.TrimSpace(c.Query("method")))
	searchQuery := strings.TrimSpace(c.Query("q"))
	fromRaw := strings.TrimSpace(c.Query("from"))
	toRaw := strings.TrimSpace(c.Query("to"))

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
	if method != "" {
		query = query.Where("method = ?", method)
	}
	if searchQuery != "" {
		like := "%" + searchQuery + "%"
		query = query.Where("endpoint_slug LIKE ? OR path LIKE ? OR remote_addr LIKE ? OR body LIKE ? OR headers LIKE ?", like, like, like, like, like)
	}
	if fromTime, ok := parseTimeFilter(fromRaw, false); ok {
		query = query.Where("created_at >= ?", fromTime)
	}
	if toTime, ok := parseTimeFilter(toRaw, true); ok {
		query = query.Where("created_at <= ?", toTime)
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

	var methodOptions []string
	storage.DB.Model(&models.RequestLog{}).
		Distinct("method").
		Order("method ASC").
		Pluck("method", &methodOptions)

	filterQuery := buildFilterQuery(map[string]string{
		"type":     reqType,
		"endpoint": endpointSlug,
		"method":   method,
		"q":        searchQuery,
		"from":     fromRaw,
		"to":       toRaw,
	})

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	c.HTML(http.StatusOK, "requests.html", gin.H{
		"ContentTemplate": "requests_content",
		"requests":        requests,
		"endpoints":       endpoints,
		"methodOptions":   methodOptions,
		"currentType":     reqType,
		"currentSlug":     endpointSlug,
		"currentMethod":   method,
		"currentQuery":    searchQuery,
		"currentFrom":     fromRaw,
		"currentTo":       toRaw,
		"filterQuery":     filterQuery,
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

	var suggested models.RequestLog
	var suggestedCompareID uint
	if err := storage.DB.Where("endpoint_slug = ? AND id < ?", req.EndpointSlug, req.ID).Order("id DESC").First(&suggested).Error; err == nil {
		suggestedCompareID = suggested.ID
	}

	c.HTML(http.StatusOK, "request_detail.html", gin.H{
		"ContentTemplate":    "request_detail_content",
		"request":            req,
		"requestTarget":      req.Path + replayQuerySuffix(req.QueryParams),
		"suggestedCompareID": suggestedCompareID,
		"title":              "Request #" + id,
	})
}
