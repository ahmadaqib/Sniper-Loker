package services

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"Goravel-learn/app/models"
	"Goravel-learn/app/services/scraper"

	"github.com/gorilla/websocket"
)

var DefaultBroadcastService = NewBroadcastService(2 * time.Second)

type BroadcastService struct {
	flushEvery time.Duration
	channels   sync.Map
}

type channelHub struct {
	connections sync.Map
	batch       chan any
}

func NewBroadcastService(flushEvery time.Duration) *BroadcastService {
	if flushEvery <= 0 {
		flushEvery = 2 * time.Second
	}
	return &BroadcastService{flushEvery: flushEvery}
}

func ChannelName(keyword, location string) string {
	key := slug(keyword)
	loc := slug(location)
	if key == "" {
		key = "all"
	}
	if loc == "" {
		loc = "all"
	}
	return key + "-" + loc
}

func (b *BroadcastService) Subscribe(channel string, conn *websocket.Conn) func() {
	hub := b.getHub(channel)
	hub.connections.Store(conn, true)
	return func() {
		hub.connections.Delete(conn)
		_ = conn.Close()
	}
}

func (b *BroadcastService) BroadcastJobs(keyword, location string, jobs []scraper.ScrapedJob) {
	if len(jobs) == 0 {
		return
	}
	channel := ChannelName(keyword, location)
	payload := map[string]any{
		"type":      "jobs",
		"channel":   channel,
		"jobs":      jobs,
		"count":     len(jobs),
		"timestamp": time.Now().UTC(),
	}
	b.getHub(channel).batch <- payload
}

func (b *BroadcastService) BroadcastStoredJobs(keyword, location string, jobs []models.Job) {
	if len(jobs) == 0 {
		return
	}
	channel := ChannelName(keyword, location)
	payload := map[string]any{
		"type":      "jobs",
		"channel":   channel,
		"jobs":      jobs,
		"count":     len(jobs),
		"timestamp": time.Now().UTC(),
	}
	b.getHub(channel).batch <- payload
}

func (b *BroadcastService) getHub(channel string) *channelHub {
	if value, ok := b.channels.Load(channel); ok {
		return value.(*channelHub)
	}

	hub := &channelHub{batch: make(chan any, 128)}
	actual, loaded := b.channels.LoadOrStore(channel, hub)
	if loaded {
		return actual.(*channelHub)
	}
	go b.flush(channel, hub)
	return hub
}

func (b *BroadcastService) flush(channel string, hub *channelHub) {
	ticker := time.NewTicker(b.flushEvery)
	defer ticker.Stop()

	var batch []any
	for {
		select {
		case payload := <-hub.batch:
			batch = append(batch, payload)
		case <-ticker.C:
			if len(batch) == 0 {
				continue
			}
			message := map[string]any{
				"type":      "batch",
				"channel":   channel,
				"items":     batch,
				"count":     len(batch),
				"timestamp": time.Now().UTC(),
			}
			hub.connections.Range(func(key, _ any) bool {
				conn := key.(*websocket.Conn)
				if err := conn.WriteJSON(message); err != nil {
					hub.connections.Delete(conn)
					_ = conn.Close()
				}
				return true
			})
			batch = nil
		}
	}
}

var slugInvalid = regexp.MustCompile(`[^a-z0-9]+`)

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = slugInvalid.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	return value
}
