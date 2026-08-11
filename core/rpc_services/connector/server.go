package connector

import (
	dataaccessv1 "weave/core/gen/core/data_access/v1"
	"weave/core/vault"
)

type Server struct {
	dataaccessv1.UnimplementedConnectorServiceServer
	vault *vault.Vault
}

func NewServer(v *vault.Vault) *Server {
	return &Server{vault: v}
}
