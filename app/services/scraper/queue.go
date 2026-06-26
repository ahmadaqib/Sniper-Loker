package scraper

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type QueueTask struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	Keyword     string        `bson:"keyword"`
	Location    string        `bson:"location"`
	Status      string        `bson:"status"`
	Attempts    int           `bson:"attempts"`
	LastError   string        `bson:"last_error,omitempty"`
	AvailableAt time.Time     `bson:"available_at"`
	CreatedAt   time.Time     `bson:"created_at"`
	UpdatedAt   time.Time     `bson:"updated_at"`
}

type Queue interface {
	Enqueue(ctx context.Context, query SearchQuery) (bson.ObjectID, error)
	Next(ctx context.Context) (*QueueTask, error)
	Ack(ctx context.Context, id bson.ObjectID) error
	Fail(ctx context.Context, id bson.ObjectID, err error, retryAt time.Time) error
}

type MongoQueue struct {
	collection *mongo.Collection
	now        func() time.Time
}

func NewMongoQueue(database *mongo.Database) *MongoQueue {
	return &MongoQueue{collection: database.Collection("scrape_queue"), now: time.Now}
}

func (q *MongoQueue) EnsureIndexes(ctx context.Context) error {
	_, err := q.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "available_at", Value: 1}}, Options: options.Index().SetName("idx_queue_ready")},
		{Keys: bson.D{{Key: "created_at", Value: 1}}, Options: options.Index().SetName("idx_queue_created_at")},
	})
	return err
}

func (q *MongoQueue) Enqueue(ctx context.Context, query SearchQuery) (bson.ObjectID, error) {
	now := q.now()
	task := QueueTask{
		ID:          bson.NewObjectID(),
		Keyword:     query.Keyword,
		Location:    query.Location,
		Status:      "pending",
		AvailableAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err := q.collection.InsertOne(ctx, task)
	return task.ID, err
}

func (q *MongoQueue) Next(ctx context.Context) (*QueueTask, error) {
	now := q.now()
	filter := bson.M{"status": "pending", "available_at": bson.M{"$lte": now}}
	update := bson.M{"$set": bson.M{"status": "running", "updated_at": now}, "$inc": bson.M{"attempts": 1}}
	opts := options.FindOneAndUpdate().SetSort(bson.D{{Key: "created_at", Value: 1}}).SetReturnDocument(options.After)

	var task QueueTask
	err := q.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&task)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (q *MongoQueue) Ack(ctx context.Context, id bson.ObjectID) error {
	_, err := q.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"status": "done", "updated_at": q.now()}})
	return err
}

func (q *MongoQueue) Fail(ctx context.Context, id bson.ObjectID, taskErr error, retryAt time.Time) error {
	message := ""
	if taskErr != nil {
		message = taskErr.Error()
	}
	_, err := q.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"status":       "pending",
			"last_error":   message,
			"available_at": retryAt,
			"updated_at":   q.now(),
		},
	})
	return err
}
