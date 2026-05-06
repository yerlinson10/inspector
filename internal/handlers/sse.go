package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"inspector/internal/broadcaster"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var eventsWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const (
	eventsWSWriteWait  = 10 * time.Second
	eventsWSPongWait   = 60 * time.Second
	eventsWSPingPeriod = 25 * time.Second
)

// EventsWebSocket streams broadcaster events over a WebSocket. Recommended
// transport when the app is exposed via tunnels/proxies that buffer SSE
// (e.g. Cloudflare Tunnel).
func EventsWebSocket(c *gin.Context) {
	conn, err := eventsWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(eventsWSPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(eventsWSPongWait))
	})

	ch := broadcaster.DefaultHub.Subscribe()
	defer broadcaster.DefaultHub.Unsubscribe(ch)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ping := time.NewTicker(eventsWSPingPeriod)
	defer ping.Stop()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(eventsWSWriteWait))
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				return
			}
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(eventsWSWriteWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-done:
			return
		case <-c.Request.Context().Done():
			return
		}
	}
}

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
	c.Header("Content-Encoding", "identity")
	origin := c.GetHeader("Origin")
	if origin == "" {
		origin = c.Request.Header.Get("Referer")
		if origin != "" {
			if refURL, err := url.Parse(origin); err == nil && refURL.Scheme != "" && refURL.Host != "" {
				origin = refURL.Scheme + "://" + refURL.Host
			} else {
				origin = ""
			}
		}
	}
	if origin != "" {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
	}

	// Flush headers and a larger SSE prelude to reduce proxy/tunnel buffering.
	// Some tunnels don't forward tiny chunks quickly.
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(c.Writer, ":"+strings.Repeat(" ", 2048)+"\n")
	_, _ = fmt.Fprint(c.Writer, ": connected\n\n")
	flusher.Flush()

	ch := broadcaster.DefaultHub.Subscribe()
	defer broadcaster.DefaultHub.Unsubscribe(ch)

	heartbeat := time.NewTicker(5 * time.Second)
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
