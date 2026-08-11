package tenant

import (
	dataaccessv1 "weave/core/gen/core/data_access/v1"
)

type Server struct {
	dataaccessv1.UnimplementedTenantServiceServer
}

func NewServer() *Server {
	return &Server{}
}
