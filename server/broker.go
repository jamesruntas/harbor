package main

import (
	"fmt"
	"net/http"
	"sync"
)

// Broker is a simple SSE pub/sub. The server publishes events by name;
// each connected frontend client receives them as SSE messages.
type Broker struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
}

func newBroker() *Broker {
	return &Broker{clients: make(map[chan string]struct{})}
}

func (b *Broker) subscribe() chan string {
	ch := make(chan string, 8)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broker) unsubscribe(ch chan string) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

// publish sends an event name to all connected clients.
// Slow clients are skipped (non-blocking send).
func (b *Broker) publish(event string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- event:
		default:
		}
	}
}

// ServeSSE handles a single SSE client connection.
// Blocks until the client disconnects.
func (b *Broker) ServeSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := b.subscribe()
	defer b.unsubscribe(ch)

	for {
		select {
		case event := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", event)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}
