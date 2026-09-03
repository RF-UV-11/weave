package memory

import (
	dataaccessv1 "weave/core/gen/core/data_access/v1"
	"weave/core/qdrant"
)

type Server struct {
	dataaccessv1.UnimplementedMemoryServiceServer
	qdrant *qdrant.Client
}

func NewServer(q *qdrant.Client) *Server {
	return &Server{qdrant: q}
}
