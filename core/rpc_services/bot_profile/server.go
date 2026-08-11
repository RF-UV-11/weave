package botprofile

import (
	dataaccessv1 "weave/core/gen/core/data_access/v1"
)

type Server struct {
	dataaccessv1.UnimplementedBotProfileServiceServer
}

func NewServer() *Server {
	return &Server{}
}
