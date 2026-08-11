package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type DbType struct {
	Client *mongo.Client
	Db     *mongo.Database
}

var Db *DbType

var Healthy bool

func InitDatabase(uri, dbName string) error {
	opts := options.Client().
		ApplyURI(uri).
		SetBSONOptions(&options.BSONOptions{UseJSONStructTags: true})

	client, err := mongo.Connect(opts)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		return err
	}

	Db = &DbType{Client: client, Db: client.Database(dbName)}
	Healthy = true

	go pingLoop()

	return EnsureIndexes(context.Background())
}

func pingLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		Healthy = Db.Client.Ping(ctx, nil) == nil
		cancel()
	}
}
