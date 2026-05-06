package handlers

import (
	"context"
	"net/http"
	"time"

	"inspector/internal/storage"

	"github.com/gin-gonic/gin"
)

func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func Readyz(c *gin.Context) {
	if storage.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not-ready", "error": "database not initialized"})
		return
	}

	sqlDB, err := storage.DB.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not-ready", "error": "database unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not-ready", "error": "database ping failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
