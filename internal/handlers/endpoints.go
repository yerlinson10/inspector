package handlers

import (
	"net/http"
	"regexp"
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

	c.HTML(http.StatusOK, "endpoints.html", gin.H{
		"ContentTemplate": "endpoints_content",
		"endpoints":       endpoints,
		"title":           "Endpoints",
	})
}

func CreateEndpoint(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	slug := strings.TrimSpace(c.PostForm("slug"))
	description := strings.TrimSpace(c.PostForm("description"))
	responseStatus := 200
	if s := c.PostForm("response_status"); s != "" {
		if val, err := parseInt(s); err == nil && val > 0 {
			responseStatus = val
		}
	}
	responseHeaders := strings.TrimSpace(c.PostForm("response_headers"))
	responseBody := strings.TrimSpace(c.PostForm("response_body"))

	if name == "" || slug == "" {
		var endpoints []models.Endpoint
		storage.DB.Order("created_at DESC").Find(&endpoints)
		c.HTML(http.StatusBadRequest, "endpoints.html", gin.H{
			"ContentTemplate": "endpoints_content",
			"endpoints":       endpoints,
			"error":           "Name and slug are required",
			"title":           "Endpoints",
		})
		return
	}

	if !slugRegex.MatchString(slug) {
		var endpoints []models.Endpoint
		storage.DB.Order("created_at DESC").Find(&endpoints)
		c.HTML(http.StatusBadRequest, "endpoints.html", gin.H{
			"ContentTemplate": "endpoints_content",
			"endpoints":       endpoints,
			"error":           "Slug must be lowercase alphanumeric with hyphens only",
			"title":           "Endpoints",
		})
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
			c.HTML(http.StatusConflict, "endpoints.html", gin.H{
				"ContentTemplate": "endpoints_content",
				"endpoints":       endpoints,
				"error":           "An endpoint with this slug already exists",
				"title":           "Endpoints",
			})
			return
		}
		c.HTML(http.StatusInternalServerError, "endpoints.html", gin.H{
			"ContentTemplate": "endpoints_content",
			"endpoints":       endpoints,
			"error":           "Failed to create endpoint: " + err.Error(),
			"title":           "Endpoints",
		})
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

	var endpoint models.Endpoint
	if err := storage.DB.First(&endpoint, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	responseStatus := 200
	if s := c.PostForm("response_status"); s != "" {
		if val, err := parseInt(s); err == nil && val > 0 {
			responseStatus = val
		}
	}
	responseHeaders := strings.TrimSpace(c.PostForm("response_headers"))
	responseBody := strings.TrimSpace(c.PostForm("response_body"))

	if name != "" {
		endpoint.Name = name
	}
	endpoint.Description = description
	endpoint.ResponseStatus = responseStatus
	endpoint.ResponseHeaders = responseHeaders
	endpoint.ResponseBody = responseBody

	storage.DB.Save(&endpoint)
	broadcaster.DefaultHub.Broadcast(broadcaster.Event{
		Type: "endpoint_changed",
		Data: map[string]interface{}{
			"action": "updated",
			"id":     endpoint.ID,
			"slug":   endpoint.Slug,
		},
	})
	c.Redirect(http.StatusSeeOther, "/endpoints")
}

func DeleteEndpoint(c *gin.Context) {
	id := c.Param("id")

	var endpoint models.Endpoint
	if err := storage.DB.First(&endpoint, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	storage.DB.Where("endpoint_id = ?", endpoint.ID).Delete(&models.RequestLog{})
	storage.DB.Delete(&endpoint)

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

	storage.DB.Where("endpoint_id = ?", endpoint.ID).Delete(&models.RequestLog{})
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

func parseInt(s string) (int, error) {
	var val int
	_, err := func() (int, error) {
		for _, c := range s {
			if c < '0' || c > '9' {
				return 0, &parseError{}
			}
			val = val*10 + int(c-'0')
		}
		return val, nil
	}()
	return val, err
}

type parseError struct{}

func (e *parseError) Error() string { return "invalid number" }
