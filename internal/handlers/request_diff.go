package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"inspector/internal/models"
	"inspector/internal/storage"

	"github.com/gin-gonic/gin"
)

type requestDiffSection struct {
	Name  string
	Left  string
	Right string
	Equal bool
}

func RequestDiff(c *gin.Context) {
	leftID, _ := strconv.ParseUint(strings.TrimSpace(c.Query("left")), 10, 64)
	rightID, _ := strconv.ParseUint(strings.TrimSpace(c.Query("right")), 10, 64)
	endpointSlug := strings.TrimSpace(c.Query("endpoint"))

	recentQuery := storage.DB.Model(&models.RequestLog{})
	if endpointSlug != "" {
		recentQuery = recentQuery.Where("endpoint_slug = ?", endpointSlug)
	}

	var recentRequests []models.RequestLog
	recentQuery.Order("created_at DESC").Limit(200).Find(&recentRequests)

	if leftID == 0 && len(recentRequests) > 0 {
		leftID = uint64(recentRequests[0].ID)
	}
	if rightID == 0 && len(recentRequests) > 1 {
		rightID = uint64(recentRequests[1].ID)
	}

	var leftRequest models.RequestLog
	hasLeft := leftID > 0 && storage.DB.First(&leftRequest, leftID).Error == nil

	if endpointSlug == "" && hasLeft {
		endpointSlug = leftRequest.EndpointSlug
	}

	var rightRequest models.RequestLog
	hasRight := rightID > 0 && storage.DB.First(&rightRequest, rightID).Error == nil

	var sections []requestDiffSection
	diffCount := 0

	if hasLeft && hasRight {
		sections = []requestDiffSection{
			compareRawSection("Method", strings.TrimSpace(leftRequest.Method), strings.TrimSpace(rightRequest.Method)),
			compareRawSection("Path", strings.TrimSpace(leftRequest.Path), strings.TrimSpace(rightRequest.Path)),
			compareRawSection("Remote IP", strings.TrimSpace(leftRequest.RemoteAddr), strings.TrimSpace(rightRequest.RemoteAddr)),
			compareRawSection("Headers", prettyMaybeJSON(leftRequest.Headers), prettyMaybeJSON(rightRequest.Headers)),
			compareRawSection("Query Params", prettyMaybeJSON(leftRequest.QueryParams), prettyMaybeJSON(rightRequest.QueryParams)),
			compareRawSection("Body", prettyMaybeJSON(leftRequest.Body), prettyMaybeJSON(rightRequest.Body)),
		}
		for _, section := range sections {
			if !section.Equal {
				diffCount++
			}
		}
	}

	var endpointOptions []string
	storage.DB.Model(&models.RequestLog{}).
		Distinct("endpoint_slug").
		Where("endpoint_slug <> ''").
		Order("endpoint_slug ASC").
		Pluck("endpoint_slug", &endpointOptions)

	sort.Strings(endpointOptions)

	c.HTML(http.StatusOK, "request_diff.html", withViewData(c, gin.H{
		"ContentTemplate": "request_diff_content",
		"title":           "Request Diff",
		"leftRequest":     leftRequest,
		"rightRequest":    rightRequest,
		"leftID":          uint(leftID),
		"rightID":         uint(rightID),
		"hasLeft":         hasLeft,
		"hasRight":        hasRight,
		"sections":        sections,
		"diffCount":       diffCount,
		"recentRequests":  recentRequests,
		"endpointOptions": endpointOptions,
		"currentEndpoint": endpointSlug,
		"now":             time.Now(),
	}))
}

func compareRawSection(name, left, right string) requestDiffSection {
	return requestDiffSection{
		Name:  name,
		Left:  left,
		Right: right,
		Equal: strings.TrimSpace(left) == strings.TrimSpace(right),
	}
}

func prettyMaybeJSON(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	var payload interface{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
		encoded, err := json.MarshalIndent(payload, "", "  ")
		if err == nil {
			return string(encoded)
		}
	}

	return raw
}
