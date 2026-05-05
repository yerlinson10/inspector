package models

import "time"

type SentRequest struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Type            string    `gorm:"index;not null" json:"type"` // http, webhook, websocket
	Method          string    `json:"method"`
	URL             string    `json:"url"`
	Headers         string    `gorm:"type:text" json:"headers"`
	Body            string    `gorm:"type:text" json:"body"`
	ResponseStatus  int       `json:"response_status"`
	ResponseHeaders string    `gorm:"type:text" json:"response_headers"`
	ResponseBody    string    `gorm:"type:text" json:"response_body"`
	DurationMs      int64     `json:"duration_ms"`
	Error           string    `gorm:"type:text" json:"error"`
	CreatedAt       time.Time `gorm:"index" json:"created_at"`
}
