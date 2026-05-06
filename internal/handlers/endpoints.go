package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"inspector/internal/broadcaster"
	"inspector/internal/models"
	"inspector/internal/storage"

	"github.com/gin-gonic/gin"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func ListEndpoints(c *gin.Context) {
	var endpoints []models.Endpoint
	storage.DB.Order("created_at DESC").Find(&endpoints)
	renderEndpointsPage(c, http.StatusOK, endpoints, "")
}

func CreateEndpoint(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	slug := strings.TrimSpace(c.PostForm("slug"))
	description := strings.TrimSpace(c.PostForm("description"))
	responseStatus := 200
	if s := c.PostForm("response_status"); s != "" {
		val, err := parseResponseStatus(s)
		if err != nil {
			var endpoints []models.Endpoint
			storage.DB.Order("created_at DESC").Find(&endpoints)
			renderEndpointsPage(c, http.StatusBadRequest, endpoints, err.Error())
			return
		}
		responseStatus = val
	}
	responseHeaders := strings.TrimSpace(c.PostForm("response_headers"))
	responseBody := strings.TrimSpace(c.PostForm("response_body"))
	if err := validateEndpointResponseConfig(responseHeaders, responseBody); err != nil {
		var endpoints []models.Endpoint
		storage.DB.Order("created_at DESC").Find(&endpoints)
		renderEndpointsPage(c, http.StatusBadRequest, endpoints, err.Error())
		return
	}

	if name == "" || slug == "" {
		var endpoints []models.Endpoint
		storage.DB.Order("created_at DESC").Find(&endpoints)
		renderEndpointsPage(c, http.StatusBadRequest, endpoints, "Name and slug are required")
		return
	}

	if !slugRegex.MatchString(slug) {
		var endpoints []models.Endpoint
		storage.DB.Order("created_at DESC").Find(&endpoints)
		renderEndpointsPage(c, http.StatusBadRequest, endpoints, "Slug must be lowercase alphanumeric with hyphens only")
		return
	}

	endpoint := models.Endpoint{
		Name:            name,
		Slug:            slug,
		Description:     description,
		ResponseStatus:  responseStatus,
		ResponseHeaders: responseHeaders,
		ResponseBody:    responseBody,
	}

	if err := storage.DB.Create(&endpoint).Error; err != nil {
		var endpoints []models.Endpoint
		storage.DB.Order("created_at DESC").Find(&endpoints)
		if strings.Contains(err.Error(), "UNIQUE") {
			renderEndpointsPage(c, http.StatusConflict, endpoints, "An endpoint with this slug already exists")
			return
		}
		renderEndpointsPage(c, http.StatusInternalServerError, endpoints, "Failed to create endpoint: "+err.Error())
		return
	}

	broadcaster.DefaultHub.Broadcast(broadcaster.Event{
		Type: "endpoint_changed",
		Data: map[string]interface{}{
			"action": "created",
			"id":     endpoint.ID,
			"slug":   endpoint.Slug,
		},
	})

	c.Redirect(http.StatusSeeOther, "/endpoints")
}

func UpdateEndpoint(c *gin.Context) {
	id := c.Param("id")
	isHTMLForm := c.Request.Method == http.MethodPost

	var endpoint models.Endpoint
	if err := storage.DB.First(&endpoint, id).Error; err != nil {
		if isHTMLForm {
			var endpoints []models.Endpoint
			storage.DB.Order("created_at DESC").Find(&endpoints)
			renderEndpointsPage(c, http.StatusNotFound, endpoints, "Endpoint not found")
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	responseStatus := endpoint.ResponseStatus
	if s := c.PostForm("response_status"); s != "" {
		val, err := parseResponseStatus(s)
		if err != nil {
			if isHTMLForm {
				var endpoints []models.Endpoint
				storage.DB.Order("created_at DESC").Find(&endpoints)
				renderEndpointsPage(c, http.StatusBadRequest, endpoints, err.Error())
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		responseStatus = val
	}
	responseHeaders := strings.TrimSpace(c.PostForm("response_headers"))
	responseBody := strings.TrimSpace(c.PostForm("response_body"))
	if err := validateEndpointResponseConfig(responseHeaders, responseBody); err != nil {
		if isHTMLForm {
			var endpoints []models.Endpoint
			storage.DB.Order("created_at DESC").Find(&endpoints)
			renderEndpointsPage(c, http.StatusBadRequest, endpoints, err.Error())
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if name != "" {
		endpoint.Name = name
	}
	endpoint.Description = description
	endpoint.ResponseStatus = responseStatus
	endpoint.ResponseHeaders = responseHeaders
	endpoint.ResponseBody = responseBody

	if err := storage.DB.Save(&endpoint).Error; err != nil {
		if isHTMLForm {
			var endpoints []models.Endpoint
			storage.DB.Order("created_at DESC").Find(&endpoints)
			renderEndpointsPage(c, http.StatusInternalServerError, endpoints, "failed to update endpoint")
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update endpoint"})
		return
	}
	broadcaster.DefaultHub.Broadcast(broadcaster.Event{
		Type: "endpoint_changed",
		Data: map[string]interface{}{
			"action": "updated",
			"id":     endpoint.ID,
			"slug":   endpoint.Slug,
		},
	})
	if !isHTMLForm {
		c.JSON(http.StatusOK, gin.H{"status": "updated", "id": endpoint.ID})
		return
	}
	c.Redirect(http.StatusSeeOther, "/endpoints")
}

func DeleteEndpoint(c *gin.Context) {
	id := c.Param("id")

	var endpoint models.Endpoint
	if err := storage.DB.First(&endpoint, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	if err := storage.DB.Where("endpoint_id = ?", endpoint.ID).Delete(&models.RequestLog{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete endpoint logs"})
		return
	}
	if err := storage.DB.Delete(&endpoint).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete endpoint"})
		return
	}

	broadcaster.DefaultHub.Broadcast(broadcaster.Event{
		Type: "endpoint_changed",
		Data: map[string]interface{}{
			"action": "deleted",
			"id":     endpoint.ID,
			"slug":   endpoint.Slug,
		},
	})

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func ClearEndpointLogs(c *gin.Context) {
	id := c.Param("id")

	var endpoint models.Endpoint
	if err := storage.DB.First(&endpoint, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	if err := storage.DB.Where("endpoint_id = ?", endpoint.ID).Delete(&models.RequestLog{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear logs"})
		return
	}
	broadcaster.DefaultHub.Broadcast(broadcaster.Event{
		Type: "endpoint_changed",
		Data: map[string]interface{}{
			"action": "logs_cleared",
			"id":     endpoint.ID,
			"slug":   endpoint.Slug,
		},
	})
	c.JSON(http.StatusOK, gin.H{"status": "cleared"})
}

func parseResponseStatus(s string) (int, error) {
	val, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, &parseError{msg: "response_status must be a valid integer"}
	}
	if val < 100 || val > 599 {
		return 0, &parseError{msg: "response_status must be between 100 and 599"}
	}
	return val, nil
}

func renderEndpointsPage(c *gin.Context, status int, endpoints []models.Endpoint, errMsg string) {
	data := gin.H{
		"ContentTemplate": "endpoints_content",
		"endpoints":       endpoints,
		"title":           "Endpoints",
		"publicWSBaseURL": requestWSBaseURL(c),
	}
	if token, ok := c.Get("csrfToken"); ok {
		if tokenStr, ok := token.(string); ok {
			data["csrfToken"] = tokenStr
		}
	}
	if errMsg != "" {
		data["error"] = errMsg
	}
	c.HTML(status, "endpoints.html", data)
}

func validateEndpointResponseConfig(responseHeaders, responseBody string) error {
	headersRaw := strings.TrimSpace(responseHeaders)
	if headersRaw != "" {
		var headers map[string]interface{}
		if err := json.Unmarshal([]byte(headersRaw), &headers); err != nil {
			return &parseError{msg: "response_headers must be valid JSON object"}
		}
	}

	bodyRaw := strings.TrimSpace(responseBody)
	if bodyRaw != "" {
		var body interface{}
		if err := json.Unmarshal([]byte(bodyRaw), &body); err != nil {
			return &parseError{msg: "response_body must be valid JSON"}
		}
	}

	return nil
}

type parseError struct {
	msg string
}

func (e *parseError) Error() string { return e.msg }
