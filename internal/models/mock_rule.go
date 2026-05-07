package models

import "time"

const (
	MockScopeEndpoint = "endpoint"
	MockScopeGlobal   = "global"
)

type MockRule struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	EndpointID          *uint     `gorm:"index;index:idx_mock_rules_active_scope_endpoint,priority:3" json:"endpoint_id,omitempty"`
	Scope               string    `gorm:"size:16;index;index:idx_mock_rules_active_scope_endpoint,priority:2;not null;default:endpoint" json:"scope"`
	Name                string    `gorm:"not null" json:"name"`
	Priority            int       `gorm:"index;not null;default:100" json:"priority"`
	IsActive            bool      `gorm:"index;index:idx_mock_rules_active_scope_endpoint,priority:1;not null;default:true" json:"is_active"`
	Method              string    `gorm:"size:16" json:"method"`
	PathMode            string    `gorm:"size:16;not null;default:any" json:"path_mode"`
	PathValue           string    `gorm:"type:text" json:"path_value"`
	QueryMode           string    `gorm:"size:16;not null;default:any" json:"query_mode"`
	QueryJSON           string    `gorm:"type:text" json:"query_json"`
	HeadersMode         string    `gorm:"size:16;not null;default:any" json:"headers_mode"`
	HeadersJSON         string    `gorm:"type:text" json:"headers_json"`
	BodyMode            string    `gorm:"size:16;not null;default:any" json:"body_mode"`
	BodyPattern         string    `gorm:"type:text" json:"body_pattern"`
	ExcludedEndpointIDs string    `gorm:"type:text" json:"excluded_endpoint_ids"`
	ResponseStatus      int       `gorm:"not null;default:200" json:"response_status"`
	ResponseHeaders     string    `gorm:"type:text" json:"response_headers"`
	ResponseBody        string    `gorm:"type:text" json:"response_body"`
	DelayMs             int       `gorm:"not null;default:0" json:"delay_ms"`
	HitCount            int64     `gorm:"not null;default:0" json:"hit_count"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
