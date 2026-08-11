package auth

import (
	dataaccessv1 "weave/core/gen/core/data_access/v1"
)

type Server struct {
	dataaccessv1.UnimplementedAuthServiceServer
	jwtSecret []byte
}

func NewServer(jwtSecret []byte) *Server {
	return &Server{jwtSecret: jwtSecret}
}
