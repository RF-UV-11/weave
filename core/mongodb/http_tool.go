package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	databasev1 "weave/core/gen/database/v1"
)

const managedConnectorName = "weave_managed"

// GetOrCreateManagedConnector returns the tenant's single auto-created
// connector that mcp-gateway serves — every HttpTool a tenant registers
// belongs to this one connector, the same way a real MCP server groups
// multiple tools under one endpoint. gatewayBaseURL + "/" + tenantID +
// "/mcp" is the address orchestrator's MCP client actually calls;
// mcp-gateway strips the "/{tenant_id}" prefix and forwards the rest
// ("/mcp") to a per-tenant MCP sub-app whose default mount path is
// "/mcp" (mcp-gateway/server.py + the mcp SDK's streamable_http_app
// default) — the trailing "/mcp" here has to match that or every
// request 404s before it ever reaches tool routing.
func GetOrCreateManagedConnector(ctx context.Context, tenantID, gatewayBaseURL string) (*databasev1.Connector, error) {
	existing, err := getConnectorByName(ctx, tenantID, managedConnectorName)
	if err == nil {
		return existing, nil
	}

	endpoint := gatewayBaseURL + "/" + tenantID + "/mcp"
	c := &databasev1.Connector{
		XId:       "conn_" + newULID(),
		TenantId:  tenantID,
		Name:      managedConnectorName,
		Transport: "weave_managed",
		Endpoint:  endpoint,
		Status:    "active",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if _, err := Db.Db.Collection(ColNames.Connectors).InsertOne(ctx, c); err != nil {
		// Another concurrent RegisterHttpTool call may have created it
		// first (unique (tenant_id, name) index) — that's fine, just use theirs.
		if existing, getErr := getConnectorByName(ctx, tenantID, managedConnectorName); getErr == nil {
			return existing, nil
		}
		return nil, err
	}
	return c, nil
}

func getConnectorByName(ctx context.Context, tenantID, name string) (*databasev1.Connector, error) {
	var c databasev1.Connector
	err := Db.Db.Collection(ColNames.Connectors).
		FindOne(ctx, bson.M{"tenant_id": tenantID, "name": name}).Decode(&c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func CreateHttpTool(ctx context.Context, tenantID, connectorID, name, description, httpEndpoint, httpMethod, paramsSchema, credentialRefID string) (*databasev1.HttpTool, error) {
	t := &databasev1.HttpTool{
		XId:             "htool_" + newULID(),
		TenantId:        tenantID,
		ConnectorId:     connectorID,
		Name:            name,
		Description:     description,
		HttpEndpoint:    httpEndpoint,
		HttpMethod:      httpMethod,
		ParamsSchema:    paramsSchema,
		CredentialRefId: credentialRefID,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	if _, err := Db.Db.Collection(ColNames.HttpTools).InsertOne(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// ListHttpTools is scoped to tenantID — same isolation rule as every
// other lookup in core (docs/architecture/SECURITY.md §2). mcp-gateway
// calls this (by tenant, parsed from the connector endpoint path it
// receives the request on) to answer tools/list and to resolve a
// tools/call by name.
func ListHttpTools(ctx context.Context, tenantID string) ([]*databasev1.HttpTool, error) {
	cur, err := Db.Db.Collection(ColNames.HttpTools).Find(ctx, bson.M{"tenant_id": tenantID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var tools []*databasev1.HttpTool
	for cur.Next(ctx) {
		var t databasev1.HttpTool
		if err := cur.Decode(&t); err != nil {
			return nil, err
		}
		tools = append(tools, &t)
	}
	return tools, cur.Err()
}

func GetHttpTool(ctx context.Context, tenantID, id string) (*databasev1.HttpTool, error) {
	var t databasev1.HttpTool
	err := Db.Db.Collection(ColNames.HttpTools).
		FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func DeleteHttpTool(ctx context.Context, tenantID, id string) error {
	_, err := Db.Db.Collection(ColNames.HttpTools).DeleteOne(ctx, bson.M{"_id": id, "tenant_id": tenantID})
	return err
}
