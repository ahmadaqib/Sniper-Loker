package controllers

import (
	"context"
	"net/http"
	"time"

	"Goravel-learn/app/repositories"
	"Goravel-learn/app/services"
	"Goravel-learn/app/services/scraper"
	"Goravel-learn/app/facades"

	goravelhttp "github.com/goravel/framework/contracts/http"
	"github.com/gorilla/websocket"
)

type JobController struct {
	upgrader websocket.Upgrader
}

func NewJobController() *JobController {
	return &JobController{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
	}
}

func (c *JobController) Index(ctx goravelhttp.Context) goravelhttp.Response {
	requestCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()

	repo, err := services.NewDefaultJobRepository(requestCtx, false)
	if err != nil {
		return ctx.Response().Json(http.StatusServiceUnavailable, goravelhttp.Json{
			"error":  "database unavailable",
			"detail": err.Error(),
			"jobs":   []any{},
			"count":  0,
		})
	}

	since, _ := time.Parse(time.RFC3339, ctx.Request().Query("since"))
	jobs, err := repo.ListJobs(requestCtx, repositories.JobFilter{
		Keyword:  ctx.Request().Query("keyword"),
		Location: ctx.Request().Query("location"),
		Since:    since,
		Limit:    int64(ctx.Request().QueryInt("limit", 50)),
	})
	if err != nil {
		return ctx.Response().Json(http.StatusServiceUnavailable, goravelhttp.Json{
			"error":  "database unavailable",
			"detail": err.Error(),
			"jobs":   []any{},
			"count":  0,
		})
	}

	return ctx.Response().Success().Json(goravelhttp.Json{
		"jobs":      jobs,
		"count":     len(jobs),
		"timestamp": time.Now().UTC(),
	})
}

func (c *JobController) WebSocket(ctx goravelhttp.Context) goravelhttp.Response {
	keyword := ctx.Request().Query("keyword")
	location := ctx.Request().Query("location")
	channel := services.ChannelName(keyword, location)

	conn, err := c.upgrader.Upgrade(ctx.Response().Writer(), ctx.Request().Origin(), nil)
	if err != nil {
		return ctx.Response().Json(http.StatusBadRequest, goravelhttp.Json{"error": err.Error()})
	}

	unsubscribe := services.DefaultBroadcastService.Subscribe(channel, conn)
	go c.readUntilClosed(context.Background(), conn, unsubscribe)

	_ = conn.WriteJSON(map[string]any{
		"type":      "connected",
		"channel":   channel,
		"timestamp": time.Now().UTC(),
	})

	return nil
}

func (c *JobController) Search(ctx goravelhttp.Context) goravelhttp.Response {
	query := scraper.SearchQuery{
		Keyword:  firstNonEmpty(ctx.Request().Input("keyword"), ctx.Request().Query("keyword")),
		Location: firstNonEmpty(ctx.Request().Input("location"), ctx.Request().Query("location")),
	}

	initCtx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	service, err := services.NewDefaultScraperService(initCtx)
	cancel()
	if err != nil {
		return ctx.Response().Json(http.StatusServiceUnavailable, goravelhttp.Json{
			"error":  "scraper unavailable",
			"detail": err.Error(),
		})
	}

	go func() {
		runCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := service.Scrape(runCtx, query).Err(); err != nil {
							facades.Log().Errorf("Scraper error for %s %s: %v", query.Keyword, query.Location, err)
						}
	}()

	return ctx.Response().Json(http.StatusAccepted, goravelhttp.Json{
		"status":    "accepted",
		"keyword":   query.Keyword,
		"location":  query.Location,
		"channel":   services.ChannelName(query.Keyword, query.Location),
		"timestamp": time.Now().UTC(),
	})
}

func (c *JobController) readUntilClosed(ctx context.Context, conn *websocket.Conn, unsubscribe func()) {
	defer unsubscribe()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
