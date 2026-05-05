package handlers

import (
	"fmt"
	"net/http"
	"time"

	"inspector/internal/broadcaster"

	"github.com/gin-gonic/gin"
)

func SSEStream(c *gin.Context) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("X-Accel-Buffering", "no")
	c.Header("Access-Control-Allow-Origin", "*")

	// Flush headers and a small SSE comment immediately to reduce proxy/tunnel buffering.
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(c.Writer, ": connected\n\n")
	flusher.Flush()

	ch := broadcaster.DefaultHub.Subscribe()
	defer broadcaster.DefaultHub.Unsubscribe(ch)

	heartbeat := time.NewTicker(10 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
			flusher.Flush()
		case <-heartbeat.C:
			_, _ = fmt.Fprint(c.Writer, ": ping\n\n")
			flusher.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}
