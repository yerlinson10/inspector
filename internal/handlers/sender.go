package handlers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
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
	data := gin.H{
		"ContentTemplate": "sender_content",
		"title":           "Send Request",
		"type":            "http",
	}

	if replayID := strings.TrimSpace(c.Query("replay")); replayID != "" {
		var captured models.RequestLog
		if err := storage.DB.First(&captured, replayID).Error; err != nil {
			data["error"] = "Request para replay no encontrado"
		} else {
			method := strings.ToUpper(strings.TrimSpace(captured.Method))
			if method == "" || method == "WS" {
				method = http.MethodPost
			}

			data["method"] = method
			data["type"] = replayTypeFromCaptured(captured.Type)
			data["headers"] = replayHeadersForSender(captured.Headers)
			data["body"] = captured.Body
			data["url"] = requestBaseURL(c) + captured.Path + replayQuerySuffix(captured.QueryParams)
			data["replay_source_id"] = captured.ID
		}
	}

	c.HTML(http.StatusOK, "sender.html", withViewData(c, data))
}

func SendHTTP(c *gin.Context) {
	method := strings.ToUpper(strings.TrimSpace(c.PostForm("method")))
	url := strings.TrimSpace(c.PostForm("url"))
	headersRaw := strings.TrimSpace(c.PostForm("headers"))
	body := c.PostForm("body")
	reqType := c.DefaultPostForm("type", "http")

	if url == "" {
		c.HTML(http.StatusBadRequest, "sender.html", withViewData(c, gin.H{
			"ContentTemplate": "sender_content",
			"error":           "URL is required",
			"title":           "Send Request",
			"type":            reqType,
		}))
		return
	}

	if err := ValidateHTTPOutboundURL(url); err != nil {
		c.HTML(http.StatusBadRequest, "sender.html", withViewData(c, gin.H{
			"ContentTemplate": "sender_content",
			"error":           "Blocked target URL: " + err.Error(),
			"title":           "Send Request",
			"method":          method,
			"url":             url,
			"headers":         headersRaw,
			"body":            body,
			"type":            reqType,
		}))
		return
	}

	if method == "" {
		method = "GET"
	}
	if !isAllowedHTTPMethod(method) {
		c.HTML(http.StatusBadRequest, "sender.html", withViewData(c, gin.H{
			"ContentTemplate": "sender_content",
			"error":           "Unsupported HTTP method",
			"title":           "Send Request",
			"method":          method,
			"url":             url,
			"headers":         headersRaw,
			"body":            body,
			"type":            reqType,
		}))
		return
	}

	parsedHeaders, headerParseErr := parseSenderHeadersJSON(headersRaw)
	if headerParseErr != nil {
		c.HTML(http.StatusBadRequest, "sender.html", withViewData(c, gin.H{
			"ContentTemplate": "sender_content",
			"error":           headerParseErr.Error(),
			"title":           "Send Request",
			"method":          method,
			"url":             url,
			"headers":         headersRaw,
			"body":            body,
			"type":            reqType,
		}))
		return
	}

	if bodyErr := validateSenderBodyJSON(body); bodyErr != nil {
		c.HTML(http.StatusBadRequest, "sender.html", withViewData(c, gin.H{
			"ContentTemplate": "sender_content",
			"error":           bodyErr.Error(),
			"title":           "Send Request",
			"method":          method,
			"url":             url,
			"headers":         headersRaw,
			"body":            body,
			"type":            reqType,
		}))
		return
	}

	// Build request
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	httpReq, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		saveSentRequest(reqType, method, url, headersRaw, body, 0, "", "", 0, err.Error())
		c.HTML(http.StatusOK, "sender.html", withViewData(c, gin.H{
			"ContentTemplate": "sender_content",
			"error":           "Failed to create request: " + err.Error(),
			"title":           "Send Request",
			"method":          method,
			"url":             url,
			"headers":         headersRaw,
			"body":            body,
			"type":            reqType,
		}))
		return
	}

	// Parse headers
	for k, values := range parsedHeaders {
		if len(values) == 0 {
			httpReq.Header.Set(k, "")
			continue
		}
		for _, value := range values {
			httpReq.Header.Add(k, value)
		}
	}

	client := newOutboundHTTPClient(30 * time.Second)

	start := time.Now()
	resp, err := client.Do(httpReq)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		saveSentRequest(reqType, method, url, headersRaw, body, 0, "", "", duration, err.Error())
		c.HTML(http.StatusOK, "sender.html", withViewData(c, gin.H{
			"ContentTemplate": "sender_content",
			"error":           "Request failed: " + err.Error(),
			"title":           "Send Request",
			"method":          method,
			"url":             url,
			"headers":         headersRaw,
			"body":            body,
			"type":            reqType,
			"duration":        duration,
		}))
		return
	}
	defer resp.Body.Close()

	maxRespBody := maxResponseBodyBytes()
	respReader := io.Reader(resp.Body)
	if maxRespBody > 0 {
		respReader = io.LimitReader(resp.Body, maxRespBody+1)
	}

	respBody, readErr := io.ReadAll(respReader)
	if readErr != nil {
		saveSentRequest(reqType, method, url, headersRaw, body, resp.StatusCode, "", "", duration, "failed to read response body")
		c.HTML(http.StatusOK, "sender.html", withViewData(c, gin.H{
			"ContentTemplate": "sender_content",
			"error":           "Failed to read response body",
			"title":           "Send Request",
			"method":          method,
			"url":             url,
			"headers":         headersRaw,
			"body":            body,
			"type":            reqType,
			"duration":        duration,
		}))
		return
	}

	if maxRespBody > 0 && int64(len(respBody)) > maxRespBody {
		truncated := string(respBody[:maxRespBody])
		respHeaders, _ := json.Marshal(resp.Header)
		errMsg := fmt.Sprintf("response body exceeded %d bytes and was truncated", maxRespBody)
		sent := saveSentRequest(reqType, method, url, headersRaw, body, resp.StatusCode, string(respHeaders), truncated, duration, errMsg)
		c.HTML(http.StatusOK, "sender.html", withViewData(c, gin.H{
			"ContentTemplate": "sender_content",
			"error":           errMsg,
			"title":           "Send Request",
			"method":          method,
			"url":             url,
			"headers":         headersRaw,
			"body":            body,
			"type":            reqType,
			"response_status": resp.StatusCode,
			"response_body":   truncated,
			"duration":        duration,
			"sent_id":         sent.ID,
		}))
		return
	}

	respHeaders, _ := json.Marshal(resp.Header)

	sent := saveSentRequest(reqType, method, url, headersRaw, body, resp.StatusCode, string(respHeaders), string(respBody), duration, "")

	c.HTML(http.StatusOK, "sender.html", withViewData(c, gin.H{
		"ContentTemplate": "sender_content",
		"title":           "Send Request",
		"method":          method,
		"url":             url,
		"headers":         headersRaw,
		"body":            body,
		"type":            reqType,
		"response_status": resp.StatusCode,
		"response_body":   string(respBody),
		"duration":        duration,
		"sent_id":         sent.ID,
		"success":         true,
	}))
}

func parseSenderHeadersJSON(raw string) (map[string][]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string][]string{}, nil
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, fmt.Errorf("headers must be a valid JSON object")
	}

	headersObject, ok := parsed.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("headers must be a JSON object")
	}

	if headersObject == nil {
		return map[string][]string{}, nil
	}

	headers := make(map[string][]string, len(headersObject))
	for key, rawValue := range headersObject {
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("headers must not contain empty keys")
		}

		switch value := rawValue.(type) {
		case string:
			headers[key] = []string{value}
		case []interface{}:
			if len(value) == 0 {
				headers[key] = []string{}
				continue
			}

			list := make([]string, 0, len(value))
			for _, item := range value {
				text, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("header %q must contain only text values", key)
				}
				list = append(list, text)
			}
			headers[key] = list
		default:
			return nil, fmt.Errorf("header %q must be text or a list of text values", key)
		}
	}

	return headers, nil
}

func validateSenderBodyJSON(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	var body interface{}
	if err := json.Unmarshal([]byte(trimmed), &body); err != nil {
		return fmt.Errorf("body must be valid JSON")
	}

	return nil
}

func SentHistory(c *gin.Context) {
	reqType := strings.TrimSpace(c.Query("type"))
	method := strings.ToUpper(strings.TrimSpace(c.Query("method")))
	statusFilter := strings.TrimSpace(c.Query("status"))
	searchQuery := strings.TrimSpace(c.Query("q"))
	fromRaw := strings.TrimSpace(c.Query("from"))
	toRaw := strings.TrimSpace(c.Query("to"))

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage := 50

	query := storage.DB.Model(&models.SentRequest{})
	if reqType != "" {
		query = query.Where("type = ?", reqType)
	}
	if method != "" {
		query = query.Where("method = ?", method)
	}
	if statusFilter != "" {
		if strings.EqualFold(statusFilter, "error") {
			query = query.Where("error <> ''")
		} else if parsedStatus, err := strconv.Atoi(statusFilter); err == nil {
			query = query.Where("response_status = ?", parsedStatus)
		}
	}
	if searchQuery != "" {
		like := "%" + searchQuery + "%"
		query = query.Where("url LIKE ? OR headers LIKE ? OR body LIKE ? OR response_body LIKE ? OR error LIKE ?", like, like, like, like, like)
	}
	if fromTime, ok := parseTimeFilter(fromRaw, false); ok {
		query = query.Where("created_at >= ?", fromTime)
	}
	if toTime, ok := parseTimeFilter(toRaw, true); ok {
		query = query.Where("created_at <= ?", toTime)
	}

	var total int64
	query.Count(&total)

	var requests []models.SentRequest
	query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&requests)

	var methodOptions []string
	storage.DB.Model(&models.SentRequest{}).
		Distinct("method").
		Order("method ASC").
		Pluck("method", &methodOptions)

	filterQuery := buildFilterQuery(map[string]string{
		"type":   reqType,
		"method": method,
		"status": statusFilter,
		"q":      searchQuery,
		"from":   fromRaw,
		"to":     toRaw,
	})

	var latestSentID uint
	if len(requests) > 0 {
		latestSentID = requests[0].ID
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	c.HTML(http.StatusOK, "sent_history.html", withViewData(c, gin.H{
		"ContentTemplate": "sent_history_content",
		"requests":        requests,
		"methodOptions":   methodOptions,
		"currentType":     reqType,
		"currentMethod":   method,
		"currentStatus":   statusFilter,
		"currentQuery":    searchQuery,
		"currentFrom":     fromRaw,
		"currentTo":       toRaw,
		"filterQuery":     filterQuery,
		"page":            page,
		"totalPages":      totalPages,
		"total":           total,
		"latestSentID":    latestSentID,
		"title":           "Sent History",
	}))
}

func SentDetail(c *gin.Context) {
	id := c.Param("id")

	var req models.SentRequest
	if err := storage.DB.First(&req, id).Error; err != nil {
		c.String(http.StatusNotFound, "Request not found")
		return
	}

	viewReq := req
	viewReq.Headers = formatHeadersForDisplay(req.Headers)
	viewReq.Body = formatBodyForDisplay(req.Body, req.Headers)
	viewReq.ResponseHeaders = formatHeadersForDisplay(req.ResponseHeaders)
	viewReq.ResponseBody = formatBodyForDisplay(req.ResponseBody, req.ResponseHeaders)

	c.HTML(http.StatusOK, "sent_detail.html", withViewData(c, gin.H{
		"ContentTemplate": "sent_detail_content",
		"request":         viewReq,
		"title":           "Sent Request #" + id,
	}))
}

func WSClientPage(c *gin.Context) {
	c.HTML(http.StatusOK, "ws_client.html", withViewData(c, gin.H{
		"ContentTemplate": "ws_client_content",
		"title":           "WebSocket Client",
	}))
}

func WSProxy(c *gin.Context) {
	targetURL := c.Query("url")
	if targetURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter required"})
		return
	}
	if err := ValidateWSOutboundURL(targetURL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "blocked target URL: " + err.Error()})
		return
	}

	// Upgrade browser connection
	browserConn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer browserConn.Close()
	if maxBodyBytes := maxRequestBodyBytes(); maxBodyBytes > 0 {
		browserConn.SetReadLimit(maxBodyBytes)
	}

	// Connect to target
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		NetDialContext:   outboundDialContext,
		TLSClientConfig:  &tls.Config{MinVersion: tls.VersionTLS12},
	}
	targetConn, _, err := dialer.Dial(targetURL, nil)
	if err != nil {
		payload, marshalErr := json.Marshal(gin.H{"error": "Failed to connect to target: " + err.Error()})
		if marshalErr != nil {
			payload = []byte(`{"error":"Failed to connect to target"}`)
		}
		_ = browserConn.WriteMessage(websocket.TextMessage, payload)
		return
	}
	defer targetConn.Close()
	if maxBodyBytes := maxResponseBodyBytes(); maxBodyBytes > 0 {
		targetConn.SetReadLimit(maxBodyBytes)
	}

	// Log connection
	sentReq := models.SentRequest{
		Type:      "websocket",
		Method:    "WS",
		URL:       targetURL,
		CreatedAt: time.Now(),
	}
	if err := storage.DB.Create(&sentReq).Error; err != nil {
		log.Printf("failed to persist websocket proxy session: %v", err)
	}

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
	sent = redactSentRequest(sent)

	if err := storage.DB.Create(&sent).Error; err != nil {
		log.Printf("failed to persist sent request: %v", err)
		return sent
	}

	triggerSentRequestAlert(sent)

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

func replayTypeFromCaptured(capturedType string) string {
	if strings.EqualFold(strings.TrimSpace(capturedType), "webhook") {
		return "webhook"
	}
	return "http"
}

func replayHeadersForSender(rawHeaders string) string {
	trimmed := strings.TrimSpace(rawHeaders)
	if trimmed == "" {
		return ""
	}

	var multi map[string][]string
	if err := json.Unmarshal([]byte(trimmed), &multi); err == nil {
		normalized := make(map[string]interface{}, len(multi))
		keys := make([]string, 0, len(multi))
		for k := range multi {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			values := multi[key]
			if len(values) == 1 {
				normalized[key] = values[0]
				continue
			}
			if len(values) == 0 {
				normalized[key] = []string{}
				continue
			}
			normalized[key] = values
		}

		encoded, encodeErr := json.MarshalIndent(normalized, "", "  ")
		if encodeErr == nil {
			return string(encoded)
		}
	}

	var single map[string]string
	if err := json.Unmarshal([]byte(trimmed), &single); err == nil {
		if encoded, encodeErr := json.MarshalIndent(single, "", "  "); encodeErr == nil {
			return string(encoded)
		}
	}

	return formatHeadersForDisplay(trimmed)
}

func replayQuerySuffix(rawQuery string) string {
	trimmed := strings.TrimSpace(rawQuery)
	if trimmed == "" {
		return ""
	}

	var queryMap map[string][]string
	if err := json.Unmarshal([]byte(trimmed), &queryMap); err != nil {
		return ""
	}

	values := url.Values{}
	for key, list := range queryMap {
		for _, val := range list {
			values.Add(key, val)
		}
	}

	encoded := values.Encode()
	if encoded == "" {
		return ""
	}

	return "?" + encoded
}

func isAllowedHTTPMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
