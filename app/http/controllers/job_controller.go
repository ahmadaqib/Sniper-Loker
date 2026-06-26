package controllers

import (
	"context"
	"net/http"
	"time"

	"Goravel-learn/app/repositories"
	"Goravel-learn/app/services"

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
	repo, err := services.NewDefaultJobRepository(ctx)
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, goravelhttp.Json{"error": err.Error()})
	}

	since, _ := time.Parse(time.RFC3339, ctx.Request().Query("since"))
	jobs, err := repo.ListJobs(ctx, repositories.JobFilter{
		Keyword:  ctx.Request().Query("keyword"),
		Location: ctx.Request().Query("location"),
		Since:    since,
		Limit:    int64(ctx.Request().QueryInt("limit", 50)),
	})
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, goravelhttp.Json{"error": err.Error()})
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
