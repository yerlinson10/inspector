package models

import "time"

type RequestLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	EndpointID   uint      `gorm:"index" json:"endpoint_id"`
	EndpointSlug string    `gorm:"index" json:"endpoint_slug"`
	Type         string    `gorm:"index;not null" json:"type"` // http, webhook, websocket
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	Headers      string    `gorm:"type:text" json:"headers"`
	QueryParams  string    `gorm:"type:text" json:"query_params"`
	Body         string    `gorm:"type:text" json:"body"`
	RemoteAddr   string    `json:"remote_addr"`
	SizeBytes    int64     `json:"size_bytes"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}
