package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	ulid "github.com/oklog/ulid/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	databasev1 "servicesphere/backend-services/gen/database/v1"
)

var ErrNotFound = errors.New("not found")

var slaByPriority = map[databasev1.TicketPriority]time.Duration{
	databasev1.TicketPriority_TICKET_PRIORITY_URGENT: 4 * time.Hour,
	databasev1.TicketPriority_TICKET_PRIORITY_HIGH:   24 * time.Hour,
	databasev1.TicketPriority_TICKET_PRIORITY_MEDIUM: 3 * 24 * time.Hour,
	databasev1.TicketPriority_TICKET_PRIORITY_LOW:    7 * 24 * time.Hour,
}

// Ticket is the mongodb-layer interface for the "tickets" collection — the
// only place in the repo, besides initialize.go, that touches Mongo for it.
// rpc_services/ticket delegates to this and contains no query logic itself.
type Ticket interface {
	CreateTicket(ctx context.Context, t *databasev1.Ticket) (*databasev1.Ticket, error)
	GetTicket(ctx context.Context, tenantID, id string) (*databasev1.Ticket, error)
	ListTickets(ctx context.Context, tenantID, customerID string, pageSize int32, pageToken string) ([]*databasev1.Ticket, string, error)
}

func (db *DbType) CreateTicket(ctx context.Context, t *databasev1.Ticket) (*databasev1.Ticket, error) {
	if t.GetIdempotencyKey() != "" {
		existing, err := db.findTicketByIdempotencyKey(ctx, t.GetTenantId(), t.GetIdempotencyKey())
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	if t.Priority == databasev1.TicketPriority_TICKET_PRIORITY_UNSPECIFIED {
		t.Priority = databasev1.TicketPriority_TICKET_PRIORITY_MEDIUM
	}
	now := time.Now().UTC()
	t.XId = "tkt_" + ulid.Make().String()
	t.Status = databasev1.TicketStatus_TICKET_STATUS_OPEN
	t.SlaDueAt = now.Add(slaByPriority[t.Priority]).Format(time.RFC3339)
	t.CreatedAt = now.Format(time.RFC3339)

	if _, err := db.TicketCollection.InsertOne(ctx, t); err != nil {
		return nil, fmt.Errorf("insert ticket: %w", err)
	}
	return t, nil
}

func (db *DbType) findTicketByIdempotencyKey(ctx context.Context, tenantID, key string) (*databasev1.Ticket, error) {
	var t databasev1.Ticket
	err := db.TicketCollection.FindOne(ctx, bson.M{"tenant_id": tenantID, "idempotency_key": key}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (db *DbType) GetTicket(ctx context.Context, tenantID, id string) (*databasev1.Ticket, error) {
	var t databasev1.Ticket
	err := db.TicketCollection.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTickets returns tickets for a tenant (optionally filtered by customer),
// newest first, plus the id to resume from (empty when this is the last page).
func (db *DbType) ListTickets(ctx context.Context, tenantID, customerID string, pageSize int32, pageToken string) ([]*databasev1.Ticket, string, error) {
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
	cursor, err := db.TicketCollection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, "", err
	}
	defer cursor.Close(ctx)

	var tickets []*databasev1.Ticket
	if err := cursor.All(ctx, &tickets); err != nil {
		return nil, "", err
	}

	nextPageToken := ""
	if int32(len(tickets)) == pageSize {
		nextPageToken = tickets[len(tickets)-1].GetXId()
	}
	return tickets, nextPageToken, nil
}
