package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"inspector/internal/broadcaster"
	"inspector/internal/models"
	"inspector/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func ReceiveRequest(c *gin.Context) {
	slug := c.Param("slug")

	var endpoint models.Endpoint
	if err := storage.DB.Where("slug = ?", slug).First(&endpoint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
		return
	}

	body, _ := io.ReadAll(c.Request.Body)
	headers, _ := json.Marshal(c.Request.Header)
	queryParams, _ := json.Marshal(c.Request.URL.Query())

	reqType := "http"
	if isWebhook(c) {
		reqType = "webhook"
	}

	log := models.RequestLog{
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

	storage.DB.Create(&log)
	go storage.Cleanup(10000)

	broadcaster.DefaultHub.Broadcast(broadcaster.Event{
		Type: "new_request",
		Data: map[string]interface{}{
			"id":            log.ID,
			"endpoint_slug": log.EndpointSlug,
			"type":          log.Type,
			"method":        log.Method,
			"path":          log.Path,
			"remote_addr":   log.RemoteAddr,
			"size_bytes":    log.SizeBytes,
			"created_at":    log.CreatedAt.Format(time.RFC3339),
		},
	})

	// Send configured response
	status := endpoint.ResponseStatus
	if status == 0 {
		status = 200
	}

	if endpoint.ResponseHeaders != "" {
		var respHeaders map[string]string
		if err := json.Unmarshal([]byte(endpoint.ResponseHeaders), &respHeaders); err == nil {
			for k, v := range respHeaders {
				c.Header(k, v)
			}
		}
	}

	respBody := endpoint.ResponseBody
	if respBody == "" {
		respBody = `{"status":"received"}`
	}

	c.Data(status, "application/json", []byte(respBody))
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

	headers, _ := json.Marshal(c.Request.Header)

	for {
		msgType, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		log := models.RequestLog{
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

		storage.DB.Create(&log)

		broadcaster.DefaultHub.Broadcast(broadcaster.Event{
			Type: "new_request",
			Data: map[string]interface{}{
				"id":            log.ID,
				"endpoint_slug": log.EndpointSlug,
				"type":          log.Type,
				"method":        log.Method,
				"path":          log.Path,
				"remote_addr":   log.RemoteAddr,
				"size_bytes":    log.SizeBytes,
				"created_at":    log.CreatedAt.Format(time.RFC3339),
			},
		})

		// Echo back acknowledgment
		ack := []byte(`{"status":"received"}`)
		conn.WriteMessage(msgType, ack)
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
