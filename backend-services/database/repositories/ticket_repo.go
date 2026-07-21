package repositories

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// TicketDoc is the MongoDB document shape for the "tickets" collection.
// This is the only place ticket data is shaped for storage — callers outside
// backend-services never see this type, only the generated proto Ticket message.
type TicketDoc struct {
	ID          string    `bson:"_id"`
	TenantID    string    `bson:"tenant_id"`
	CustomerID  string    `bson:"customer_id"`
	Subject     string    `bson:"subject"`
	Description string    `bson:"description"`
	Priority    string    `bson:"priority"`
	Status      string    `bson:"status"`
	SLADueAt    time.Time `bson:"sla_due_at"`
	CreatedAt   time.Time `bson:"created_at"`
	// IdempotencyKey dedupes retried CreateTicket calls per tenant.
	IdempotencyKey string `bson:"idempotency_key,omitempty"`
}

var ErrNotFound = errors.New("ticket not found")

type TicketRepository struct {
	collection *mongo.Collection
}

func NewTicketRepository(db *mongo.Database) *TicketRepository {
	return &TicketRepository{collection: db.Collection("tickets")}
}

// Create inserts a ticket, or returns the existing one if idempotencyKey was
// already used for this tenant (dedupe on retried billable/create calls).
func (r *TicketRepository) Create(ctx context.Context, doc TicketDoc) (TicketDoc, error) {
	if doc.IdempotencyKey != "" {
		existing, err := r.findByIdempotencyKey(ctx, doc.TenantID, doc.IdempotencyKey)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return TicketDoc{}, err
		}
	}
	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		return TicketDoc{}, err
	}
	return doc, nil
}

func (r *TicketRepository) findByIdempotencyKey(ctx context.Context, tenantID, key string) (TicketDoc, error) {
	var doc TicketDoc
	err := r.collection.FindOne(ctx, bson.M{"tenant_id": tenantID, "idempotency_key": key}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return TicketDoc{}, ErrNotFound
	}
	return doc, err
}

func (r *TicketRepository) Get(ctx context.Context, tenantID, id string) (TicketDoc, error) {
	var doc TicketDoc
	err := r.collection.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return TicketDoc{}, ErrNotFound
	}
	return doc, err
}

// List returns tickets for a tenant (optionally filtered by customer), newest first,
// plus the id to resume from for the next page (empty when this is the last page).
func (r *TicketRepository) List(ctx context.Context, tenantID, customerID string, pageSize int32, pageToken string) ([]TicketDoc, string, error) {
	filter := bson.M{"tenant_id": tenantID}
	if customerID != "" {
		filter["customer_id"] = customerID
	}
	if pageToken != "" {
		filter["_id"] = bson.M{"$lt": pageToken}
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	findOpts := options.Find().SetLimit(int64(pageSize)).SetSort(bson.D{{Key: "_id", Value: -1}})
	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, "", err
	}
	defer cursor.Close(ctx)

	var docs []TicketDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, "", err
	}

	nextPageToken := ""
	if int32(len(docs)) == pageSize {
		nextPageToken = docs[len(docs)-1].ID
	}
	return docs, nextPageToken, nil
}
