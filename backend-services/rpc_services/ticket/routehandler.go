// RPC methods delegate to the mongodb layer; no business logic here (see
// backend-services/CLAUDE.md) — validation and defaulting live in mongodb/ticket.go.
package ticket

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	dataaccessv1 "servicesphere/backend-services/gen/backend_services/data_access/v1"
	databasev1 "servicesphere/backend-services/gen/database/v1"
	"servicesphere/backend-services/mongodb"
)

func (s *Server) CreateTicket(ctx context.Context, req *connect.Request[dataaccessv1.CreateTicketRequest]) (*connect.Response[dataaccessv1.CreateTicketResponse], error) {
	msg := req.Msg
	if msg.TenantId == "" || msg.Subject == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("tenant_id and subject are required"))
	}

	t, err := mongodb.Db.CreateTicket(ctx, &databasev1.Ticket{
		TenantId:       msg.TenantId,
		CustomerId:     msg.CustomerId,
		Subject:        msg.Subject,
		Description:    msg.Description,
		Priority:       msg.Priority,
		IdempotencyKey: msg.IdempotencyKey,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&dataaccessv1.CreateTicketResponse{Ticket: t}), nil
}

func (s *Server) GetTicket(ctx context.Context, req *connect.Request[dataaccessv1.GetTicketRequest]) (*connect.Response[dataaccessv1.GetTicketResponse], error) {
	msg := req.Msg
	t, err := mongodb.Db.GetTicket(ctx, msg.TenantId, msg.Id)
	if errors.Is(err, mongodb.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("ticket not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&dataaccessv1.GetTicketResponse{Ticket: t}), nil
}

func (s *Server) ListTickets(ctx context.Context, req *connect.Request[dataaccessv1.ListTicketsRequest]) (*connect.Response[dataaccessv1.ListTicketsResponse], error) {
	msg := req.Msg
	var pageSize int32
	var pageToken string
	if msg.Page != nil {
		pageSize = msg.Page.PageSize
		pageToken = msg.Page.PageToken
	}

	tickets, nextToken, err := mongodb.Db.ListTickets(ctx, msg.TenantId, msg.CustomerId, pageSize, pageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&dataaccessv1.ListTicketsResponse{
		Tickets: tickets,
		Page:    &databasev1.PageResponse{NextPageToken: nextToken},
	}), nil
}
