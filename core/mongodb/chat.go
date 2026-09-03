package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	databasev1 "weave/core/gen/database/v1"
)

const defaultSessionMessageLimit = 50

// CreateSession starts a new conversation (docs/architecture/
// ARCHITECTURE.md §5's short-term/session memory) — orchestrator calls
// this once per new conversation, then reuses the returned session_id
// on every subsequent turn instead of holding any state itself.
func CreateSession(ctx context.Context, tenantID, userID, botProfileID, channel string) (*databasev1.ChatSession, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	s := &databasev1.ChatSession{
		XId:          "ses_" + newULID(),
		TenantId:     tenantID,
		UserId:       userID,
		BotProfileId: botProfileID,
		Channel:      channel,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, err := Db.Db.Collection(ColNames.ChatSessions).InsertOne(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

// GetSession is scoped to tenantID — same isolation rule as every other
// lookup in core (docs/architecture/SECURITY.md §2). Used to reject a
// session_id that doesn't belong to the caller's tenant before appending
// to or reading it.
func GetSession(ctx context.Context, tenantID, id string) (*databasev1.ChatSession, error) {
	var s databasev1.ChatSession
	err := Db.Db.Collection(ColNames.ChatSessions).
		FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// AppendMessage persists one turn's message and bumps the parent
// session's updated_at. Does not verify the session belongs to tenantID
// itself — callers (core/rpc_services/chat) look the session up via
// GetSession first, which does enforce that.
func AppendMessage(ctx context.Context, tenantID, sessionID, role, content, toolUsed, connectorUsed string) (*databasev1.ChatMessage, error) {
	m := &databasev1.ChatMessage{
		XId:           "msg_" + newULID(),
		TenantId:      tenantID,
		SessionId:     sessionID,
		Role:          role,
		Content:       content,
		ToolUsed:      toolUsed,
		ConnectorUsed: connectorUsed,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if _, err := Db.Db.Collection(ColNames.ChatMessages).InsertOne(ctx, m); err != nil {
		return nil, err
	}
	_, err := Db.Db.Collection(ColNames.ChatSessions).UpdateOne(ctx,
		bson.M{"_id": sessionID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{"updated_at": m.CreatedAt}},
	)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// GetSessionMessages returns the most recent limit messages for a
// session, oldest-first (ready to hand straight to an LLM as prior
// turns) — scoped to tenantID, same isolation rule as everywhere else.
func GetSessionMessages(ctx context.Context, tenantID, sessionID string, limit int32) ([]*databasev1.ChatMessage, error) {
	if limit <= 0 {
		limit = defaultSessionMessageLimit
	}
	// Sorted by _id, not created_at: created_at is RFC3339-second
	// resolution, so messages appended within the same second (routine
	// under real load, not just in tests) would tie and sort
	// unpredictably. _id is a ULID — millisecond-resolution and
	// lexicographically sortable by creation order, so it's both the
	// unique key and a reliable ordering key.
	findOpts := options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}).SetLimit(int64(limit))
	cur, err := Db.Db.Collection(ColNames.ChatMessages).
		Find(ctx, bson.M{"tenant_id": tenantID, "session_id": sessionID}, findOpts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var messages []*databasev1.ChatMessage
	for cur.Next(ctx) {
		var m databasev1.ChatMessage
		if err := cur.Decode(&m); err != nil {
			return nil, err
		}
		messages = append(messages, &m)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}

	// Reverse: the query above sorts newest-first to make "most recent N"
	// a simple limit, but callers want oldest-first (chronological order,
	// same order the turns actually happened in).
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}
