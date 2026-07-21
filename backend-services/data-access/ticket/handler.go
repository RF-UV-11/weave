// Package ticket is the Connect/gRPC handler for the ticket data-access domain.
// It is the only code in the repo, outside database/repositories, allowed to
// know that tickets live in MongoDB — everything else calls this over gRPC.
package ticket

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	ulid "github.com/oklog/ulid/v2"

	dataaccessv1 "servicesphere/backend-services/gen/backend_services/data_access/v1"
	"servicesphere/backend-services/gen/backend_services/data_access/v1/dataaccessv1connect"
	databasev1 "servicesphere/backend-services/gen/database/v1"
	"servicesphere/backend-services/database/repositories"
)

var slaByPriority = map[databasev1.TicketPriority]time.Duration{
	databasev1.TicketPriority_TICKET_PRIORITY_URGENT: 4 * time.Hour,
	databasev1.TicketPriority_TICKET_PRIORITY_HIGH:   24 * time.Hour,
	databasev1.TicketPriority_TICKET_PRIORITY_MEDIUM: 3 * 24 * time.Hour,
	databasev1.TicketPriority_TICKET_PRIORITY_LOW:    7 * 24 * time.Hour,
}

type Handler struct {
	dataaccessv1connect.UnimplementedTicketServiceHandler
	repo *repositories.TicketRepository
}

func NewHandler(repo *repositories.TicketRepository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) CreateTicket(ctx context.Context, req *connect.Request[dataaccessv1.CreateTicketRequest]) (*connect.Response[dataaccessv1.CreateTicketResponse], error) {
	msg := req.Msg
	if msg.TenantId == "" || msg.Subject == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("tenant_id and subject are required"))
	}

	priority := msg.Priority
	if priority == databasev1.TicketPriority_TICKET_PRIORITY_UNSPECIFIED {
		priority = databasev1.TicketPriority_TICKET_PRIORITY_MEDIUM
	}
	now := time.Now().UTC()

	doc := repositories.TicketDoc{
		ID:             "tkt_" + ulid.Make().String(),
		TenantID:       msg.TenantId,
		CustomerID:     msg.CustomerId,
		Subject:        msg.Subject,
		Description:    msg.Description,
		Priority:       priority.String(),
		Status:         databasev1.TicketStatus_TICKET_STATUS_OPEN.String(),
		SLADueAt:       now.Add(slaByPriority[priority]),
		CreatedAt:      now,
		IdempotencyKey: msg.IdempotencyKey,
	}

	saved, err := h.repo.Create(ctx, doc)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&dataaccessv1.CreateTicketResponse{Ticket: toProto(saved)}), nil
}

func (h *Handler) GetTicket(ctx context.Context, req *connect.Request[dataaccessv1.GetTicketRequest]) (*connect.Response[dataaccessv1.GetTicketResponse], error) {
	msg := req.Msg
	doc, err := h.repo.Get(ctx, msg.TenantId, msg.Id)
	if errors.Is(err, repositories.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("ticket not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&dataaccessv1.GetTicketResponse{Ticket: toProto(doc)}), nil
}

func (h *Handler) ListTickets(ctx context.Context, req *connect.Request[dataaccessv1.ListTicketsRequest]) (*connect.Response[dataaccessv1.ListTicketsResponse], error) {
	msg := req.Msg
	var pageSize int32
	var pageToken string
	if msg.Page != nil {
		pageSize = msg.Page.PageSize
		pageToken = msg.Page.PageToken
	}

	docs, nextToken, err := h.repo.List(ctx, msg.TenantId, msg.CustomerId, pageSize, pageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	tickets := make([]*databasev1.Ticket, len(docs))
	for i, d := range docs {
		tickets[i] = toProto(d)
	}
	return connect.NewResponse(&dataaccessv1.ListTicketsResponse{
		Tickets: tickets,
		Page:    &databasev1.PageResponse{NextPageToken: nextToken},
	}), nil
}

func toProto(d repositories.TicketDoc) *databasev1.Ticket {
	return &databasev1.Ticket{
		Id:          d.ID,
		TenantId:    d.TenantID,
		CustomerId:  d.CustomerID,
		Subject:     d.Subject,
		Description: d.Description,
		Priority:    databasev1.TicketPriority(databasev1.TicketPriority_value[d.Priority]),
		Status:      databasev1.TicketStatus(databasev1.TicketStatus_value[d.Status]),
		SlaDueAt:    d.SLADueAt.Format(time.RFC3339),
		CreatedAt:   d.CreatedAt.Format(time.RFC3339),
	}
}
