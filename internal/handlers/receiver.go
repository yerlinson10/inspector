package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"inspector/internal/broadcaster"
	"inspector/internal/models"
	"inspector/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: isAllowedWebSocketOrigin,
}

func ReceiveRequest(c *gin.Context) {
	slug := c.Param("slug")

	var endpoint models.Endpoint
	if err := storage.DB.Where("slug = ?", slug).First(&endpoint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
		return
	}

	maxBodyBytes := maxRequestBodyBytes()
	if maxBodyBytes > 0 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}
	headers, _ := json.Marshal(c.Request.Header)
	queryParams, _ := json.Marshal(c.Request.URL.Query())

	reqType := "http"
	if isWebhook(c) {
		reqType = "webhook"
	}

	requestLog := models.RequestLog{
		EndpointID:   endpoint.ID,
		EndpointSlug: slug,
		Type:         reqType,
		Method:       c.Request.Method,
		Path:         c.Request.URL.Path,
		Headers:      string(headers),
		QueryParams:  string(queryParams),
		Body:         string(body),
		RemoteAddr:   c.ClientIP(),
		SizeBytes:    int64(len(body)),
		CreatedAt:    time.Now(),
	}
	requestLog = redactRequestLog(requestLog)

	if err := storage.DB.Create(&requestLog).Error; err != nil {
		log.Printf("failed to persist request log for slug %s: %v", slug, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist request"})
		return
	}

	broadcaster.DefaultHub.Broadcast(broadcaster.Event{
		Type: "new_request",
		Data: map[string]interface{}{
			"id":            requestLog.ID,
			"endpoint_slug": requestLog.EndpointSlug,
			"type":          requestLog.Type,
			"method":        requestLog.Method,
			"path":          requestLog.Path,
			"remote_addr":   requestLog.RemoteAddr,
			"size_bytes":    requestLog.SizeBytes,
			"created_at":    requestLog.CreatedAt.Format(time.RFC3339),
		},
	})

	var rules []models.MockRule
	storage.DB.Where("endpoint_id = ? AND is_active = ?", endpoint.ID, true).Find(&rules)
	resolved := resolveResponse(endpoint, rules, c.Request, body)

	for k, v := range resolved.headers {
		c.Header(k, v)
	}

	if resolved.delayMs > 0 {
		time.Sleep(time.Duration(resolved.delayMs) * time.Millisecond)
	}

	if resolved.ruleID != 0 {
		storage.DB.Model(&models.MockRule{}).
			Where("id = ?", resolved.ruleID).
			UpdateColumn("hit_count", gorm.Expr("hit_count + 1"))
	}

	c.Data(resolved.status, resolved.contentType, []byte(resolved.body))
}

func ReceiveWebSocket(c *gin.Context) {
	slug := c.Param("slug")

	var endpoint models.Endpoint
	if err := storage.DB.Where("slug = ?", slug).First(&endpoint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
		return
	}

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	if maxBodyBytes := maxRequestBodyBytes(); maxBodyBytes > 0 {
		conn.SetReadLimit(maxBodyBytes)
	}

	headers, _ := json.Marshal(c.Request.Header)

	for {
		msgType, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		wsLog := models.RequestLog{
			EndpointID:   endpoint.ID,
			EndpointSlug: slug,
			Type:         "websocket",
			Method:       "WS",
			Path:         c.Request.URL.Path,
			Headers:      string(headers),
			Body:         string(message),
			RemoteAddr:   c.ClientIP(),
			SizeBytes:    int64(len(message)),
			CreatedAt:    time.Now(),
		}
		wsLog = redactRequestLog(wsLog)

		if err := storage.DB.Create(&wsLog).Error; err != nil {
			log.Printf("failed to persist websocket log for slug %s: %v", slug, err)
			break
		}

		broadcaster.DefaultHub.Broadcast(broadcaster.Event{
			Type: "new_request",
			Data: map[string]interface{}{
				"id":            wsLog.ID,
				"endpoint_slug": wsLog.EndpointSlug,
				"type":          wsLog.Type,
				"method":        wsLog.Method,
				"path":          wsLog.Path,
				"remote_addr":   wsLog.RemoteAddr,
				"size_bytes":    wsLog.SizeBytes,
				"created_at":    wsLog.CreatedAt.Format(time.RFC3339),
			},
		})

		// Echo back acknowledgment
		ack := []byte(`{"status":"received"}`)
		if err := conn.WriteMessage(msgType, ack); err != nil {
			break
		}
	}
}

func isWebhook(c *gin.Context) bool {
	webhookHeaders := []string{
		"X-Webhook-Id",
		"X-GitHub-Event",
		"X-Gitlab-Event",
		"X-Stripe-Signature",
		"X-Hub-Signature",
		"X-Hook-UUID",
	}
	for _, h := range webhookHeaders {
		if c.GetHeader(h) != "" {
			return true
		}
	}
	if c.Request.Method == "POST" && c.ContentType() == "application/json" {
		return true
	}
	return false
}
