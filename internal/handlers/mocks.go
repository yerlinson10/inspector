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

type mockRuleInput struct {
	Name            string
	Priority        int
	IsActive        bool
	Method          string
	PathMode        string
	PathValue       string
	QueryMode       string
	QueryJSON       string
	HeadersMode     string
	HeadersJSON     string
	BodyMode        string
	BodyPattern     string
	ResponseStatus  int
	ResponseHeaders string
	ResponseBody    string
	DelayMs         int
}

func ListMockRules(c *gin.Context) {
	endpoint, ok := getEndpointByIDParam(c)
	if !ok {
		return
	}

	var rules []models.MockRule
	storage.DB.Where("endpoint_id = ?", endpoint.ID).Order("priority ASC, id ASC").Find(&rules)
	c.JSON(http.StatusOK, gin.H{"endpoint_id": endpoint.ID, "items": rules})
}

func CreateMockRule(c *gin.Context) {
	endpoint, ok := getEndpointByIDParam(c)
	if !ok {
		return
	}

	input, err := parseMockRuleInput(c)
	if err != nil {
		renderMockInputError(c, err.Error())
		return
	}

	rule := models.MockRule{
		EndpointID:      endpoint.ID,
		Name:            input.Name,
		Priority:        input.Priority,
		IsActive:        input.IsActive,
		Method:          input.Method,
		PathMode:        input.PathMode,
		PathValue:       input.PathValue,
		QueryMode:       input.QueryMode,
		QueryJSON:       input.QueryJSON,
		HeadersMode:     input.HeadersMode,
		HeadersJSON:     input.HeadersJSON,
		BodyMode:        input.BodyMode,
		BodyPattern:     input.BodyPattern,
		ResponseStatus:  input.ResponseStatus,
		ResponseHeaders: input.ResponseHeaders,
		ResponseBody:    input.ResponseBody,
		DelayMs:         input.DelayMs,
	}

	if err := storage.DB.Create(&rule).Error; err != nil {
		renderMockInputError(c, "failed to create mock rule")
		return
	}

	broadcastMockChange("created", endpoint, rule)
	if wantsJSON(c) {
		c.JSON(http.StatusCreated, gin.H{"status": "created", "id": rule.ID})
		return
	}
	c.Redirect(http.StatusSeeOther, "/endpoints")
}

func UpdateMockRule(c *gin.Context) {
	endpoint, ok := getEndpointByIDParam(c)
	if !ok {
		return
	}

	mockID := strings.TrimSpace(c.Param("mockId"))
	var rule models.MockRule
	if err := storage.DB.Where("id = ? AND endpoint_id = ?", mockID, endpoint.ID).First(&rule).Error; err != nil {
		if wantsJSON(c) {
			c.JSON(http.StatusNotFound, gin.H{"error": "mock rule not found"})
			return
		}
		renderMockInputError(c, "Mock rule not found")
		return
	}

	input, err := parseMockRuleInput(c)
	if err != nil {
		renderMockInputError(c, err.Error())
		return
	}

	rule.Name = input.Name
	rule.Priority = input.Priority
	rule.IsActive = input.IsActive
	rule.Method = input.Method
	rule.PathMode = input.PathMode
	rule.PathValue = input.PathValue
	rule.QueryMode = input.QueryMode
	rule.QueryJSON = input.QueryJSON
	rule.HeadersMode = input.HeadersMode
	rule.HeadersJSON = input.HeadersJSON
	rule.BodyMode = input.BodyMode
	rule.BodyPattern = input.BodyPattern
	rule.ResponseStatus = input.ResponseStatus
	rule.ResponseHeaders = input.ResponseHeaders
	rule.ResponseBody = input.ResponseBody
	rule.DelayMs = input.DelayMs

	if err := storage.DB.Save(&rule).Error; err != nil {
		renderMockInputError(c, "failed to update mock rule")
		return
	}

	broadcastMockChange("updated", endpoint, rule)
	if wantsJSON(c) {
		c.JSON(http.StatusOK, gin.H{"status": "updated", "id": rule.ID})
		return
	}
	c.Redirect(http.StatusSeeOther, "/endpoints")
}

func DeleteMockRule(c *gin.Context) {
	endpoint, ok := getEndpointByIDParam(c)
	if !ok {
		return
	}
	mockID := strings.TrimSpace(c.Param("mockId"))

	var rule models.MockRule
	if err := storage.DB.Where("id = ? AND endpoint_id = ?", mockID, endpoint.ID).First(&rule).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mock rule not found"})
		return
	}
	if err := storage.DB.Delete(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete mock rule"})
		return
	}
	broadcastMockChange("deleted", endpoint, rule)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func ToggleMockRule(c *gin.Context) {
	endpoint, ok := getEndpointByIDParam(c)
	if !ok {
		return
	}
	mockID := strings.TrimSpace(c.Param("mockId"))

	var rule models.MockRule
	if err := storage.DB.Where("id = ? AND endpoint_id = ?", mockID, endpoint.ID).First(&rule).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mock rule not found"})
		return
	}

	rule.IsActive = !rule.IsActive
	if err := storage.DB.Save(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to toggle mock rule"})
		return
	}

	broadcastMockChange("toggled", endpoint, rule)
	if wantsJSON(c) {
		c.JSON(http.StatusOK, gin.H{"status": "updated", "id": rule.ID, "is_active": rule.IsActive})
		return
	}
	c.Redirect(http.StatusSeeOther, "/endpoints")
}

func parseMockRuleInput(c *gin.Context) (mockRuleInput, error) {
	in := mockRuleInput{
		Name:            strings.TrimSpace(c.PostForm("name")),
		Method:          strings.TrimSpace(c.PostForm("method")),
		PathMode:        strings.ToLower(strings.TrimSpace(c.PostForm("path_mode"))),
		PathValue:       strings.TrimSpace(c.PostForm("path_value")),
		QueryMode:       strings.ToLower(strings.TrimSpace(c.PostForm("query_mode"))),
		QueryJSON:       strings.TrimSpace(c.PostForm("query_json")),
		HeadersMode:     strings.ToLower(strings.TrimSpace(c.PostForm("headers_mode"))),
		HeadersJSON:     strings.TrimSpace(c.PostForm("headers_json")),
		BodyMode:        strings.ToLower(strings.TrimSpace(c.PostForm("body_mode"))),
		BodyPattern:     strings.TrimSpace(c.PostForm("body_pattern")),
		ResponseHeaders: strings.TrimSpace(c.PostForm("response_headers")),
		ResponseBody:    strings.TrimSpace(c.PostForm("response_body")),
	}

	if in.Name == "" {
		in.Name = "Mock Rule"
	}
	if in.PathMode == "" {
		in.PathMode = "any"
	}
	if in.QueryMode == "" {
		in.QueryMode = "any"
	}
	if in.HeadersMode == "" {
		in.HeadersMode = "any"
	}
	if in.BodyMode == "" {
		in.BodyMode = "any"
	}

	in.IsActive = c.PostForm("is_active") == "on" || c.PostForm("is_active") == "true" || c.PostForm("is_active") == "1"
	if !c.Request.PostForm.Has("is_active") {
		in.IsActive = true
	}

	priorityRaw := strings.TrimSpace(c.PostForm("priority"))
	if priorityRaw == "" {
		in.Priority = 100
	} else {
		val, err := strconv.Atoi(priorityRaw)
		if err != nil {
			return in, &parseError{msg: "priority must be a valid integer"}
		}
		in.Priority = val
	}

	statusRaw := strings.TrimSpace(c.PostForm("response_status"))
	if statusRaw == "" {
		in.ResponseStatus = 200
	} else {
		val, err := parseResponseStatus(statusRaw)
		if err != nil {
			return in, err
		}
		in.ResponseStatus = val
	}

	delayRaw := strings.TrimSpace(c.PostForm("delay_ms"))
	if delayRaw == "" {
		in.DelayMs = 0
	} else {
		val, err := strconv.Atoi(delayRaw)
		if err != nil || val < 0 {
			return in, &parseError{msg: "delay_ms must be a non-negative integer"}
		}
		in.DelayMs = val
	}

	if err := validateMockRuleModes(in.PathMode, in.QueryMode, in.HeadersMode, in.BodyMode); err != nil {
		return in, err
	}
	if err := validateMockRulePatterns(in); err != nil {
		return in, err
	}
	if err := validateEndpointResponseConfig(in.ResponseHeaders, in.ResponseBody); err != nil {
		return in, err
	}

	in.Method = strings.ToUpper(in.Method)
	if in.Method != "" && in.Method != "*" && in.Method != "ANY" {
		if ok, _ := regexp.MatchString(`^[A-Z]+$`, in.Method); !ok {
			return in, &parseError{msg: "method must be HTTP verb, ANY or *"}
		}
	}

	return in, nil
}

func validateMockRuleModes(pathMode, queryMode, headersMode, bodyMode string) error {
	if !isOneOf(pathMode, "any", "exact", "prefix", "regex") {
		return &parseError{msg: "path_mode must be any|exact|prefix|regex"}
	}
	if !isOneOf(queryMode, "any", "contains", "exact") {
		return &parseError{msg: "query_mode must be any|contains|exact"}
	}
	if !isOneOf(headersMode, "any", "contains", "exact") {
		return &parseError{msg: "headers_mode must be any|contains|exact"}
	}
	if !isOneOf(bodyMode, "any", "exact", "contains", "regex", "json") {
		return &parseError{msg: "body_mode must be any|exact|contains|regex|json"}
	}
	return nil
}

func validateMockRulePatterns(in mockRuleInput) error {
	if in.PathMode == "regex" {
		if _, err := regexp.Compile(in.PathValue); err != nil {
			return &parseError{msg: "path_value must be a valid regex when path_mode=regex"}
		}
	}
	if in.QueryMode != "any" {
		if _, ok := parseStringMap(in.QueryJSON); !ok {
			return &parseError{msg: "query_json must be a valid JSON object of string values"}
		}
	}
	if in.HeadersMode != "any" {
		if _, ok := parseStringMap(in.HeadersJSON); !ok {
			return &parseError{msg: "headers_json must be a valid JSON object of string values"}
		}
	}
	if in.BodyMode == "regex" {
		if _, err := regexp.Compile(in.BodyPattern); err != nil {
			return &parseError{msg: "body_pattern must be a valid regex when body_mode=regex"}
		}
	}
	if in.BodyMode == "json" {
		var value map[string]interface{}
		if err := json.Unmarshal([]byte(in.BodyPattern), &value); err != nil {
			return &parseError{msg: "body_pattern must be a valid JSON object when body_mode=json"}
		}
	}
	return nil
}

func isOneOf(v string, options ...string) bool {
	for _, opt := range options {
		if v == opt {
			return true
		}
	}
	return false
}

func getEndpointByIDParam(c *gin.Context) (models.Endpoint, bool) {
	endpointID := strings.TrimSpace(c.Param("id"))
	var endpoint models.Endpoint
	if err := storage.DB.First(&endpoint, endpointID).Error; err != nil {
		if wantsJSON(c) {
			c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
		} else {
			var endpoints []models.Endpoint
			loadEndpointsForPage(&endpoints)
			renderEndpointsPage(c, http.StatusNotFound, endpoints, "Endpoint not found")
		}
		return models.Endpoint{}, false
	}
	return endpoint, true
}

func renderMockInputError(c *gin.Context, msg string) {
	if wantsJSON(c) {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	var endpoints []models.Endpoint
	loadEndpointsForPage(&endpoints)
	renderEndpointsPage(c, http.StatusBadRequest, endpoints, msg)
}

func broadcastMockChange(action string, endpoint models.Endpoint, rule models.MockRule) {
	broadcaster.DefaultHub.Broadcast(broadcaster.Event{
		Type: "mock_changed",
		Data: map[string]interface{}{
			"action":      action,
			"id":          rule.ID,
			"endpoint_id": endpoint.ID,
			"slug":        endpoint.Slug,
			"is_active":   rule.IsActive,
		},
	})
}

func wantsJSON(c *gin.Context) bool {
	return strings.Contains(strings.ToLower(c.GetHeader("Accept")), "application/json") || strings.Contains(strings.ToLower(c.ContentType()), "application/json")
}
