package models

import "time"

type MockRule struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	EndpointID      uint      `gorm:"index;not null" json:"endpoint_id"`
	Name            string    `gorm:"not null" json:"name"`
	Priority        int       `gorm:"index;not null;default:100" json:"priority"`
	IsActive        bool      `gorm:"index;not null;default:true" json:"is_active"`
	Method          string    `gorm:"size:16" json:"method"`
	PathMode        string    `gorm:"size:16;not null;default:any" json:"path_mode"`
	PathValue       string    `gorm:"type:text" json:"path_value"`
	QueryMode       string    `gorm:"size:16;not null;default:any" json:"query_mode"`
	QueryJSON       string    `gorm:"type:text" json:"query_json"`
	HeadersMode     string    `gorm:"size:16;not null;default:any" json:"headers_mode"`
	HeadersJSON     string    `gorm:"type:text" json:"headers_json"`
	BodyMode        string    `gorm:"size:16;not null;default:any" json:"body_mode"`
	BodyPattern     string    `gorm:"type:text" json:"body_pattern"`
	ResponseStatus  int       `gorm:"not null;default:200" json:"response_status"`
	ResponseHeaders string    `gorm:"type:text" json:"response_headers"`
	ResponseBody    string    `gorm:"type:text" json:"response_body"`
	DelayMs         int       `gorm:"not null;default:0" json:"delay_ms"`
	HitCount        int64     `gorm:"not null;default:0" json:"hit_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
