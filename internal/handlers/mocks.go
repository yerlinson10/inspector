package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"inspector/internal/broadcaster"
	"inspector/internal/models"
	"inspector/internal/storage"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type mockRuleInput struct {
	Scope               string
	EndpointID          *uint
	Name                string
	ExcludedEndpointIDs string
	Priority            int
	IsActive            bool
	Method              string
	PathMode            string
	PathValue           string
	QueryMode           string
	QueryJSON           string
	HeadersMode         string
	HeadersJSON         string
	BodyMode            string
	BodyPattern         string
	ResponseStatus      int
	ResponseHeaders     string
	ResponseBody        string
	DelayMs             int
}

type mockRuleView struct {
	Rule                   models.MockRule
	EndpointName           string
	EndpointSlug           string
	ExcludedEndpointSet    map[uint]bool
	ExcludedEndpointTooltip string
}

func MockRulesPage(c *gin.Context) {
	var endpoints []models.Endpoint
	storage.DB.Order("name ASC").Find(&endpoints)

	var rules []models.MockRule
	storage.DB.Order("priority ASC, id ASC").Find(&rules)

	endpointByID := map[uint]models.Endpoint{}
	for _, ep := range endpoints {
		endpointByID[ep.ID] = ep
	}

	var globalRules []mockRuleView
	var endpointRules []mockRuleView
	for _, rule := range rules {
		view := mockRuleView{Rule: rule}
		view.ExcludedEndpointSet = parseExcludedEndpointSet(rule.ExcludedEndpointIDs)
		if normalizeMockScope(rule.Scope) == models.MockScopeGlobal && len(view.ExcludedEndpointSet) > 0 {
			names := make([]string, 0, len(view.ExcludedEndpointSet))
			for id := range view.ExcludedEndpointSet {
				if ep, ok := endpointByID[id]; ok {
					names = append(names, ep.Name+" (/in/"+ep.Slug+")")
					continue
				}
				names = append(names, "ID "+strconv.Itoa(int(id)))
			}
			view.ExcludedEndpointTooltip = strings.Join(names, "\n")
		}
		if rule.EndpointID != nil {
			if ep, ok := endpointByID[*rule.EndpointID]; ok {
				view.EndpointName = ep.Name
				view.EndpointSlug = ep.Slug
			}
		}
		if normalizeMockScope(rule.Scope) == models.MockScopeGlobal {
			globalRules = append(globalRules, view)
		} else {
			endpointRules = append(endpointRules, view)
		}
	}

	data := gin.H{
		"ContentTemplate": "mocks_content",
		"title":           "Mock Rules",
		"endpoints":       endpoints,
		"globalRules":     globalRules,
		"endpointRules":   endpointRules,
	}
	if errMsg := strings.TrimSpace(c.Query("error")); errMsg != "" {
		data["error"] = errMsg
	}
	if successMsg := strings.TrimSpace(c.Query("success")); successMsg != "" {
		data["success"] = successMsg
	}
	c.HTML(http.StatusOK, "mocks.html", withViewData(c, data))
}

func parseExcludedEndpointSet(raw string) map[uint]bool {
	set := map[uint]bool{}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return set
	}
	var ids []uint
	if err := json.Unmarshal([]byte(trimmed), &ids); err != nil {
		return set
	}
	for _, id := range ids {
		set[id] = true
	}
	return set
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

func ListGlobalMockRules(c *gin.Context) {
	var rules []models.MockRule
	storage.DB.Where("scope = ?", models.MockScopeGlobal).Order("priority ASC, id ASC").Find(&rules)
	c.JSON(http.StatusOK, gin.H{"scope": models.MockScopeGlobal, "items": rules})
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

	input.Scope = models.MockScopeEndpoint
	input.EndpointID = &endpoint.ID

	rule := buildRuleFromInput(input)
	if err := storage.DB.Create(&rule).Error; err != nil {
		renderMockInputError(c, "failed to create mock rule")
		return
	}

	broadcastMockChange("created", &endpoint, rule)
	if wantsJSON(c) {
		c.JSON(http.StatusCreated, gin.H{"status": "created", "id": rule.ID})
		return
	}
	c.Redirect(http.StatusSeeOther, "/endpoints")
}

func CreateManagedMockRule(c *gin.Context) {
	input, err := parseMockRuleInput(c)
	if err != nil {
		renderMockInputErrorInMocksPage(c, err.Error())
		return
	}
	scope := normalizeMockScope(input.Scope)

	if scope == models.MockScopeGlobal {
		input.Scope = models.MockScopeGlobal
		input.EndpointID = nil
		if err := validateScopeAssignment(input.Scope, input.EndpointID); err != nil {
			renderMockInputErrorInMocksPage(c, err.Error())
			return
		}

		rule := buildRuleFromInput(input)
		if err := storage.DB.Create(&rule).Error; err != nil {
			renderMockInputErrorInMocksPage(c, "failed to create mock rule")
			return
		}
		broadcastMockChange("created", nil, rule)
		if wantsJSON(c) {
			c.JSON(http.StatusCreated, gin.H{"status": "created", "id": rule.ID, "created_count": 1})
			return
		}
		c.Redirect(http.StatusSeeOther, "/mocks?success="+url.QueryEscape("Rule created successfully"))
		return
	}

	endpointIDs, err := parseManagedEndpointIDs(c)
	if err != nil {
		renderMockInputErrorInMocksPage(c, err.Error())
		return
	}
	if len(endpointIDs) == 0 && input.EndpointID != nil {
		endpointIDs = append(endpointIDs, *input.EndpointID)
	}
	if len(endpointIDs) == 0 {
		renderMockInputErrorInMocksPage(c, "select at least one endpoint for scope=endpoint rules")
		return
	}

	createdIDs := make([]uint, 0, len(endpointIDs))
	for _, endpointID := range endpointIDs {
		id := endpointID
		input.Scope = models.MockScopeEndpoint
		input.EndpointID = &id
		if err := validateScopeAssignment(input.Scope, input.EndpointID); err != nil {
			renderMockInputErrorInMocksPage(c, err.Error())
			return
		}

		rule := buildRuleFromInput(input)
		if err := storage.DB.Create(&rule).Error; err != nil {
			renderMockInputErrorInMocksPage(c, "failed to create mock rule")
			return
		}
		createdIDs = append(createdIDs, rule.ID)
		endpoint := findEndpointByPointer(rule.EndpointID)
		broadcastMockChange("created", endpoint, rule)
	}

	if wantsJSON(c) {
		c.JSON(http.StatusCreated, gin.H{"status": "created", "ids": createdIDs, "created_count": len(createdIDs)})
		return
	}
	msg := "Rules created successfully"
	if len(createdIDs) == 1 {
		msg = "Rule created successfully"
	}
	c.Redirect(http.StatusSeeOther, "/mocks?success="+url.QueryEscape(msg))
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

	input.Scope = models.MockScopeEndpoint
	input.EndpointID = &endpoint.ID
	updateRuleFromInput(&rule, input)

	if err := storage.DB.Save(&rule).Error; err != nil {
		renderMockInputError(c, "failed to update mock rule")
		return
	}

	broadcastMockChange("updated", &endpoint, rule)
	if wantsJSON(c) {
		c.JSON(http.StatusOK, gin.H{"status": "updated", "id": rule.ID})
		return
	}
	c.Redirect(http.StatusSeeOther, "/endpoints")
}

func UpdateManagedMockRule(c *gin.Context) {
	mockID := strings.TrimSpace(c.Param("mockId"))

	var rule models.MockRule
	if err := storage.DB.First(&rule, mockID).Error; err != nil {
		if wantsJSON(c) {
			c.JSON(http.StatusNotFound, gin.H{"error": "mock rule not found"})
			return
		}
		renderMockInputErrorInMocksPage(c, "Mock rule not found")
		return
	}

	input, err := parseMockRuleInput(c)
	if err != nil {
		renderMockInputErrorInMocksPage(c, err.Error())
		return
	}
	if err := validateScopeAssignment(input.Scope, input.EndpointID); err != nil {
		renderMockInputErrorInMocksPage(c, err.Error())
		return
	}

	updateRuleFromInput(&rule, input)
	if err := storage.DB.Save(&rule).Error; err != nil {
		renderMockInputErrorInMocksPage(c, "failed to update mock rule")
		return
	}

	endpoint := findEndpointByPointer(rule.EndpointID)
	broadcastMockChange("updated", endpoint, rule)
	if wantsJSON(c) {
		c.JSON(http.StatusOK, gin.H{"status": "updated", "id": rule.ID})
		return
	}
	c.Redirect(http.StatusSeeOther, "/mocks?success="+url.QueryEscape("Rule updated successfully"))
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
	broadcastMockChange("deleted", &endpoint, rule)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func DeleteManagedMockRule(c *gin.Context) {
	mockID := strings.TrimSpace(c.Param("mockId"))

	var rule models.MockRule
	if err := storage.DB.First(&rule, mockID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mock rule not found"})
		return
	}
	if err := storage.DB.Delete(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete mock rule"})
		return
	}
	endpoint := findEndpointByPointer(rule.EndpointID)
	broadcastMockChange("deleted", endpoint, rule)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func ToggleMockRule(c *gin.Context) {
	endpoint, ok := getEndpointByIDParam(c)
	if !ok {
		return
	}
	mockID := strings.TrimSpace(c.Param("mockId"))

	result := storage.DB.Model(&models.MockRule{}).
		Where("id = ? AND endpoint_id = ?", mockID, endpoint.ID).
		UpdateColumn("is_active", gorm.Expr("NOT is_active"))
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to toggle mock rule"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "mock rule not found"})
		return
	}

	var rule models.MockRule
	if err := storage.DB.Where("id = ? AND endpoint_id = ?", mockID, endpoint.ID).First(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to toggle mock rule"})
		return
	}

	broadcastMockChange("toggled", &endpoint, rule)
	if wantsJSON(c) {
		c.JSON(http.StatusOK, gin.H{"status": "updated", "id": rule.ID, "is_active": rule.IsActive})
		return
	}
	c.Redirect(http.StatusSeeOther, "/endpoints")
}

func ToggleManagedMockRule(c *gin.Context) {
	mockID := strings.TrimSpace(c.Param("mockId"))

	result := storage.DB.Model(&models.MockRule{}).
		Where("id = ?", mockID).
		UpdateColumn("is_active", gorm.Expr("NOT is_active"))
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to toggle mock rule"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "mock rule not found"})
		return
	}

	var rule models.MockRule
	if err := storage.DB.First(&rule, mockID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to toggle mock rule"})
		return
	}

	endpoint := findEndpointByPointer(rule.EndpointID)
	broadcastMockChange("toggled", endpoint, rule)
	if wantsJSON(c) {
		c.JSON(http.StatusOK, gin.H{"status": "updated", "id": rule.ID, "is_active": rule.IsActive})
		return
	}
	c.Redirect(http.StatusSeeOther, "/mocks")
}

func parseMockRuleInput(c *gin.Context) (mockRuleInput, error) {
	in := mockRuleInput{
		Scope:           normalizeMockScope(c.PostForm("scope")),
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

	excludedEndpointIDs, err := parseManagedExcludedEndpointIDs(c)
	if err != nil {
		return in, err
	}
	if len(excludedEndpointIDs) > 0 {
		raw, marshalErr := json.Marshal(excludedEndpointIDs)
		if marshalErr != nil {
			return in, &parseError{msg: "excluded_endpoint_ids could not be processed"}
		}
		in.ExcludedEndpointIDs = string(raw)
	}

	if endpointRaw := strings.TrimSpace(c.PostForm("endpoint_id")); endpointRaw != "" {
		endpointVal, err := strconv.ParseUint(endpointRaw, 10, 64)
		if err != nil {
			return in, &parseError{msg: "endpoint_id must be a valid ID"}
		}
		endpointID := uint(endpointVal)
		in.EndpointID = &endpointID
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
	if err := validateExcludedEndpoints(in.Scope, in.ExcludedEndpointIDs); err != nil {
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

func parseManagedEndpointIDs(c *gin.Context) ([]uint, error) {
	values := c.PostFormArray("endpoint_ids")
	if len(values) == 0 {
		values = c.PostFormArray("endpoint_ids[]")
	}
	seen := map[uint]struct{}{}
	ids := make([]uint, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		parsed, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil {
			return nil, &parseError{msg: "endpoint_ids contains an invalid ID"}
		}
		id := uint(parsed)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func parseManagedExcludedEndpointIDs(c *gin.Context) ([]uint, error) {
	values := c.PostFormArray("excluded_endpoint_ids")
	if len(values) == 0 {
		values = c.PostFormArray("excluded_endpoint_ids[]")
	}
	seen := map[uint]struct{}{}
	ids := make([]uint, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		parsed, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil {
			return nil, &parseError{msg: "excluded_endpoint_ids contains an invalid ID"}
		}
		id := uint(parsed)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func validateExcludedEndpoints(scope string, raw string) error {
	trimmed := strings.TrimSpace(raw)
	if normalizeMockScope(scope) != models.MockScopeGlobal {
		if trimmed != "" && trimmed != "[]" {
			return &parseError{msg: "excluded_endpoint_ids solo aplica para scope=global"}
		}
		return nil
	}
	if trimmed == "" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(trimmed), &ids); err != nil {
		return &parseError{msg: "excluded_endpoint_ids must be a JSON array of IDs"}
	}
	if len(ids) == 0 {
		return nil
	}

	var count int64
	if err := storage.DB.Model(&models.Endpoint{}).Where("id IN ?", ids).Count(&count).Error; err != nil {
		return &parseError{msg: "excluded_endpoint_ids validation failed"}
	}
	if count != int64(len(ids)) {
		return &parseError{msg: "excluded_endpoint_ids contains endpoints that do not exist"}
	}
	return nil
}
func validateScopeAssignment(scope string, endpointID *uint) error {
	norm := normalizeMockScope(scope)
	if norm == models.MockScopeGlobal {
		return nil
	}
	if endpointID == nil || *endpointID == 0 {
		return &parseError{msg: "endpoint_id is required when scope=endpoint"}
	}
	var endpoint models.Endpoint
	if err := storage.DB.First(&endpoint, *endpointID).Error; err != nil {
		return &parseError{msg: "endpoint_id does not exist"}
	}
	return nil
}

func normalizeMockScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case models.MockScopeGlobal:
		return models.MockScopeGlobal
	default:
		return models.MockScopeEndpoint
	}
}

func buildRuleFromInput(in mockRuleInput) models.MockRule {
	return models.MockRule{
		Scope:               normalizeMockScope(in.Scope),
		EndpointID:          endpointPointerForScope(in.Scope, in.EndpointID),
		ExcludedEndpointIDs: excludedEndpointIDsForScope(in.Scope, in.ExcludedEndpointIDs),
		Name:                in.Name,
		Priority:            in.Priority,
		IsActive:            in.IsActive,
		Method:              in.Method,
		PathMode:            in.PathMode,
		PathValue:           in.PathValue,
		QueryMode:           in.QueryMode,
		QueryJSON:           in.QueryJSON,
		HeadersMode:         in.HeadersMode,
		HeadersJSON:         in.HeadersJSON,
		BodyMode:            in.BodyMode,
		BodyPattern:         in.BodyPattern,
		ResponseStatus:      in.ResponseStatus,
		ResponseHeaders:     in.ResponseHeaders,
		ResponseBody:        in.ResponseBody,
		DelayMs:             in.DelayMs,
	}
}

func updateRuleFromInput(rule *models.MockRule, in mockRuleInput) {
	rule.Scope = normalizeMockScope(in.Scope)
	rule.EndpointID = endpointPointerForScope(in.Scope, in.EndpointID)
	rule.ExcludedEndpointIDs = excludedEndpointIDsForScope(in.Scope, in.ExcludedEndpointIDs)
	rule.Name = in.Name
	rule.Priority = in.Priority
	rule.IsActive = in.IsActive
	rule.Method = in.Method
	rule.PathMode = in.PathMode
	rule.PathValue = in.PathValue
	rule.QueryMode = in.QueryMode
	rule.QueryJSON = in.QueryJSON
	rule.HeadersMode = in.HeadersMode
	rule.HeadersJSON = in.HeadersJSON
	rule.BodyMode = in.BodyMode
	rule.BodyPattern = in.BodyPattern
	rule.ResponseStatus = in.ResponseStatus
	rule.ResponseHeaders = in.ResponseHeaders
	rule.ResponseBody = in.ResponseBody
	rule.DelayMs = in.DelayMs
}

func endpointPointerForScope(scope string, endpointID *uint) *uint {
	if normalizeMockScope(scope) == models.MockScopeGlobal {
		return nil
	}
	return endpointID
}

func excludedEndpointIDsForScope(scope string, raw string) string {
	if normalizeMockScope(scope) != models.MockScopeGlobal {
		return ""
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	return trimmed
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

func findEndpointByPointer(endpointID *uint) *models.Endpoint {
	if endpointID == nil {
		return nil
	}
	var endpoint models.Endpoint
	if err := storage.DB.First(&endpoint, *endpointID).Error; err != nil {
		return nil
	}
	return &endpoint
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

func renderMockInputErrorInMocksPage(c *gin.Context, msg string) {
	if wantsJSON(c) {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	c.Redirect(http.StatusSeeOther, "/mocks?error="+url.QueryEscape(msg))
}

func broadcastMockChange(action string, endpoint *models.Endpoint, rule models.MockRule) {
	var endpointID interface{}
	var slug interface{}
	if endpoint != nil {
		endpointID = endpoint.ID
		slug = endpoint.Slug
	}

	broadcaster.DefaultHub.Broadcast(broadcaster.Event{
		Type: "mock_changed",
		Data: map[string]interface{}{
			"action":      action,
			"id":          rule.ID,
			"scope":       normalizeMockScope(rule.Scope),
			"endpoint_id": endpointID,
			"slug":        slug,
			"is_active":   rule.IsActive,
		},
	})
}

func wantsJSON(c *gin.Context) bool {
	return strings.Contains(strings.ToLower(c.GetHeader("Accept")), "application/json") || strings.Contains(strings.ToLower(c.ContentType()), "application/json")
}
