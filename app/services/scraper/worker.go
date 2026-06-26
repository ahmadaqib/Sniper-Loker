package scraper

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

type TaskHandler interface {
	HandleScrapeTask(ctx context.Context, query SearchQuery) error
}

type Worker struct {
	queue      Queue
	handler    TaskHandler
	retryDelay time.Duration
}

func NewWorker(queue Queue, handler TaskHandler, retryDelay time.Duration) *Worker {
	if retryDelay <= 0 {
		retryDelay = time.Minute
	}
	return &Worker{queue: queue, handler: handler, retryDelay: retryDelay}
}

func (w *Worker) RunOnce(ctx context.Context) error {
	task, err := w.queue.Next(ctx)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil
	}
	if err != nil {
		return err
	}

	query := SearchQuery{Keyword: task.Keyword, Location: task.Location}
	if err := w.handler.HandleScrapeTask(ctx, query); err != nil {
		return w.queue.Fail(ctx, task.ID, err, time.Now().Add(w.retryDelay))
	}

	return w.queue.Ack(ctx, task.ID)
}
