package chat

import (
	dataaccessv1 "weave/core/gen/core/data_access/v1"
)

type Server struct {
	dataaccessv1.UnimplementedChatServiceServer
}

func NewServer() *Server {
	return &Server{}
}
