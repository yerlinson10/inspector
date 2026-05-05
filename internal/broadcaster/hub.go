package broadcaster

import (
	"encoding/json"
	"sync"
)

type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[chan string]bool
}

var DefaultHub = NewHub()

func NewHub() *Hub {
	return &Hub{
		clients: make(map[chan string]bool),
	}
}

func (h *Hub) Subscribe() chan string {
	ch := make(chan string, 64)
	h.mu.Lock()
	h.clients[ch] = true
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan string) {
	h.mu.Lock()
	delete(h.clients, ch)
	close(ch)
	h.mu.Unlock()
}

func (h *Hub) Broadcast(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	msg := string(data)
	h.mu.RLock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
			// skip slow clients
		}
	}
	h.mu.RUnlock()
}
