package models

import "time"

type Endpoint struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"not null" json:"name"`
	Slug            string    `gorm:"uniqueIndex;not null" json:"slug"`
	Description     string    `json:"description"`
	ResponseStatus  int       `gorm:"default:200" json:"response_status"`
	ResponseHeaders string    `gorm:"type:text" json:"response_headers"`
	ResponseBody    string    `gorm:"type:text" json:"response_body"`
	CreatedAt       time.Time `json:"created_at"`
}
