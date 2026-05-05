package handlers

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"inspector/internal/broadcaster"
	"inspector/internal/models"
	"inspector/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func SenderPage(c *gin.Context) {
	c.HTML(http.StatusOK, "sender.html", gin.H{
		"ContentTemplate": "sender_content",
		"title":           "Send Request",
	})
}

func SendHTTP(c *gin.Context) {
	method := strings.ToUpper(strings.TrimSpace(c.PostForm("method")))
	url := strings.TrimSpace(c.PostForm("url"))
	headersRaw := strings.TrimSpace(c.PostForm("headers"))
	body := c.PostForm("body")
	reqType := c.DefaultPostForm("type", "http")

	if url == "" {
		c.HTML(http.StatusBadRequest, "sender.html", gin.H{
			"ContentTemplate": "sender_content",
			"error":           "URL is required",
			"title":           "Send Request",
		})
		return
	}

	if method == "" {
		method = "GET"
	}

	// Build request
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	httpReq, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		saveSentRequest(reqType, method, url, headersRaw, body, 0, "", "", 0, err.Error())
		c.HTML(http.StatusOK, "sender.html", gin.H{
			"ContentTemplate": "sender_content",
			"error":           "Failed to create request: " + err.Error(),
			"title":           "Send Request",
			"method":          method,
			"url":             url,
		})
		return
	}

	// Parse headers
	if headersRaw != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(headersRaw), &headers); err == nil {
			for k, v := range headers {
				httpReq.Header.Set(k, v)
			}
		} else {
			// Try line-by-line format: Key: Value
			for _, line := range strings.Split(headersRaw, "\n") {
				parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
				if len(parts) == 2 {
					httpReq.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				}
			}
		}
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
	}

	start := time.Now()
	resp, err := client.Do(httpReq)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		saveSentRequest(reqType, method, url, headersRaw, body, 0, "", "", duration, err.Error())
		c.HTML(http.StatusOK, "sender.html", gin.H{
			"ContentTemplate": "sender_content",
			"error":           "Request failed: " + err.Error(),
			"title":           "Send Request",
			"method":          method,
			"url":             url,
			"duration":        duration,
		})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	respHeaders, _ := json.Marshal(resp.Header)

	sent := saveSentRequest(reqType, method, url, headersRaw, body, resp.StatusCode, string(respHeaders), string(respBody), duration, "")

	c.HTML(http.StatusOK, "sender.html", gin.H{
		"ContentTemplate": "sender_content",
		"title":           "Send Request",
		"method":          method,
		"url":             url,
		"headers":         headersRaw,
		"body":            body,
		"response_status": resp.StatusCode,
		"response_body":   string(respBody),
		"duration":        duration,
		"sent_id":         sent.ID,
		"success":         true,
	})
}

func SentHistory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage := 50

	var total int64
	storage.DB.Model(&models.SentRequest{}).Count(&total)

	var requests []models.SentRequest
	storage.DB.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&requests)

	var latestSentID uint
	if len(requests) > 0 {
		latestSentID = requests[0].ID
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	c.HTML(http.StatusOK, "sent_history.html", gin.H{
		"ContentTemplate": "sent_history_content",
		"requests":        requests,
		"page":            page,
		"totalPages":      totalPages,
		"total":           total,
		"latestSentID":    latestSentID,
		"title":           "Sent History",
	})
}

func SentDetail(c *gin.Context) {
	id := c.Param("id")

	var req models.SentRequest
	if err := storage.DB.First(&req, id).Error; err != nil {
		c.String(http.StatusNotFound, "Request not found")
		return
	}

	c.HTML(http.StatusOK, "sent_detail.html", gin.H{
		"ContentTemplate": "sent_detail_content",
		"request":         req,
		"title":           "Sent Request #" + id,
	})
}

func WSClientPage(c *gin.Context) {
	c.HTML(http.StatusOK, "ws_client.html", gin.H{
		"ContentTemplate": "ws_client_content",
		"title":           "WebSocket Client",
	})
}

func WSProxy(c *gin.Context) {
	targetURL := c.Query("url")
	if targetURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter required"})
		return
	}

	// Upgrade browser connection
	browserConn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer browserConn.Close()

	// Connect to target
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	targetConn, _, err := dialer.Dial(targetURL, nil)
	if err != nil {
		browserConn.WriteMessage(websocket.TextMessage, []byte(`{"error":"Failed to connect to target: `+err.Error()+`"}`))
		return
	}
	defer targetConn.Close()

	// Log connection
	sentReq := models.SentRequest{
		Type:      "websocket",
		Method:    "WS",
		URL:       targetURL,
		CreatedAt: time.Now(),
	}
	storage.DB.Create(&sentReq)

	done := make(chan struct{})

	// Target → Browser
	go func() {
		defer close(done)
		for {
			msgType, message, err := targetConn.ReadMessage()
			if err != nil {
				return
			}
			if err := browserConn.WriteMessage(msgType, message); err != nil {
				return
			}
		}
	}()

	// Browser → Target
	for {
		msgType, message, err := browserConn.ReadMessage()
		if err != nil {
			break
		}
		if err := targetConn.WriteMessage(msgType, message); err != nil {
			break
		}
	}

	<-done
}

func saveSentRequest(reqType, method, url, headers, body string, status int, respHeaders, respBody string, duration int64, errMsg string) models.SentRequest {
	sent := models.SentRequest{
		Type:            reqType,
		Method:          method,
		URL:             url,
		Headers:         headers,
		Body:            body,
		ResponseStatus:  status,
		ResponseHeaders: respHeaders,
		ResponseBody:    respBody,
		DurationMs:      duration,
		Error:           errMsg,
		CreatedAt:       time.Now(),
	}
	storage.DB.Create(&sent)

	broadcaster.DefaultHub.Broadcast(broadcaster.Event{
		Type: "new_sent_request",
		Data: map[string]interface{}{
			"id":              sent.ID,
			"type":            sent.Type,
			"method":          sent.Method,
			"url":             sent.URL,
			"response_status": sent.ResponseStatus,
			"duration_ms":     sent.DurationMs,
			"error":           sent.Error,
			"created_at":      sent.CreatedAt.Format(time.RFC3339),
		},
	})

	return sent
}
