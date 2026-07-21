package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"servicesphere/backend-services/configs"
)

// Queries aggregates every collection's interface (mongodb/<collection>.go).
// Add one embed per new collection — this is what lets a handler in
// rpc_services/ call mongodb.Db.<Method> without knowing the concrete type.
type Queries interface {
	Ticket
}

// DbType is the concrete Queries implementation. Each field is a handle to
// one MongoDB collection; the generated proto message for that collection
// (protos/database/v1/<entity>.proto, marked is_collection: true) is stored
// directly as the document — see getDbConnClient's UseJSONStructTags, which
// tells the driver to BSON-encode using the json struct tags protoc-gen-go
// already emits, so no separate DB-only struct is needed.
type DbType struct {
	MongoConn *mongo.Client
	DbClient  *mongo.Database

	TicketCollection *mongo.Collection
}

var Db Queries

func getDbConnClient(ctx context.Context) *mongo.Client {
	bsonOpts := &options.BSONOptions{
		UseJSONStructTags: true,
		NilSliceAsEmpty:   true,
		NilMapAsEmpty:     true,
	}
	client, err := mongo.Connect(
		options.Client().
			ApplyURI(configs.Vars.MongoConnUri).
			SetBSONOptions(bsonOpts),
	)
	if err != nil {
		panic(err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		panic(err)
	}
	return client
}

// InitDatabase connects to MongoDB, wires every collection handle, and
// assigns the package-level Db. Call once at startup before serving RPCs.
func InitDatabase(ctx context.Context) {
	mongoConn := getDbConnClient(ctx)
	dbClient := mongoConn.Database(configs.Vars.MongoDbName)

	db := &DbType{
		MongoConn:        mongoConn,
		DbClient:         dbClient,
		TicketCollection: dbClient.Collection(ColNames.Ticket),
	}
	Db = db

	if err := ensureIndexes(ctx, db); err != nil {
		panic(err)
	}
}
