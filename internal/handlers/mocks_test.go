package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"inspector/internal/models"
	"inspector/internal/storage"

	"github.com/gin-gonic/gin"
)

func setupMockRuleTestDB(t *testing.T) {
	t.Helper()
	if err := storage.Init(":memory:"); err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
}

func TestMockRuleCRUDHandlers_JSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupMockRuleTestDB(t)

	endpoint := models.Endpoint{Name: "Demo", Slug: "demo", ResponseStatus: 200, ResponseBody: `{"ok":true}`}
	if err := storage.DB.Create(&endpoint).Error; err != nil {
		t.Fatalf("failed to seed endpoint: %v", err)
	}

	r := gin.New()
	r.POST("/endpoints/:id/mocks", CreateMockRule)
	r.POST("/endpoints/:id/mocks/:mockId/toggle", ToggleMockRule)
	r.DELETE("/endpoints/:id/mocks/:mockId", DeleteMockRule)

	form := url.Values{}
	form.Set("name", "Rule A")
	form.Set("priority", "5")
	form.Set("method", "POST")
	form.Set("path_mode", "exact")
	form.Set("path_value", "/in/demo")
	form.Set("response_status", "201")
	form.Set("response_headers", `{"Content-Type":"application/json"}`)
	form.Set("response_body", `{"mocked":true}`)
	form.Set("is_active", "true")

	req := httptest.NewRequest(http.MethodPost, "/endpoints/"+strconv.FormatUint(uint64(endpoint.ID), 10)+"/mocks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d", w.Code)
	}

	var created models.MockRule
	if err := storage.DB.Where("endpoint_id = ?", endpoint.ID).First(&created).Error; err != nil {
		t.Fatalf("failed to load created mock rule: %v", err)
	}

	toggleReq := httptest.NewRequest(http.MethodPost, "/endpoints/"+strconv.FormatUint(uint64(endpoint.ID), 10)+"/mocks/"+strconv.FormatUint(uint64(created.ID), 10)+"/toggle", nil)
	toggleReq.Header.Set("Accept", "application/json")
	toggleW := httptest.NewRecorder()
	r.ServeHTTP(toggleW, toggleReq)
	if toggleW.Code != http.StatusOK {
		t.Fatalf("expected toggle 200, got %d", toggleW.Code)
	}

	var toggled models.MockRule
	if err := storage.DB.First(&toggled, created.ID).Error; err != nil {
		t.Fatalf("failed to reload toggled rule: %v", err)
	}
	if toggled.IsActive {
		t.Fatalf("expected toggled rule to be inactive")
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/endpoints/"+strconv.FormatUint(uint64(endpoint.ID), 10)+"/mocks/"+strconv.FormatUint(uint64(created.ID), 10), nil)
	delW := httptest.NewRecorder()
	r.ServeHTTP(delW, delReq)
	if delW.Code != http.StatusOK {
		t.Fatalf("expected delete 200, got %d", delW.Code)
	}

	var count int64
	storage.DB.Model(&models.MockRule{}).Where("id = ?", created.ID).Count(&count)
	if count != 0 {
		t.Fatalf("expected deleted rule count 0, got %d", count)
	}
}

func TestCreateMockRule_InvalidRegex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupMockRuleTestDB(t)

	endpoint := models.Endpoint{Name: "Demo", Slug: "demo-2", ResponseStatus: 200, ResponseBody: `{"ok":true}`}
	if err := storage.DB.Create(&endpoint).Error; err != nil {
		t.Fatalf("failed to seed endpoint: %v", err)
	}

	r := gin.New()
	r.POST("/endpoints/:id/mocks", CreateMockRule)

	form := url.Values{}
	form.Set("name", "Broken Rule")
	form.Set("path_mode", "regex")
	form.Set("path_value", "[invalid")
	form.Set("response_status", "200")
	form.Set("response_headers", `{"Content-Type":"application/json"}`)
	form.Set("response_body", `{"mocked":true}`)

	req := httptest.NewRequest(http.MethodPost, "/endpoints/"+strconv.FormatUint(uint64(endpoint.ID), 10)+"/mocks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid input 400, got %d", w.Code)
	}
}
