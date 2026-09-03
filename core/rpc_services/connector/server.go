package connector

import (
	dataaccessv1 "weave/core/gen/core/data_access/v1"
	"weave/core/netguard"
	"weave/core/vault"
)

type Server struct {
	dataaccessv1.UnimplementedConnectorServiceServer
	vault        *vault.Vault
	resolver     netguard.Resolver
	allowPrivate bool
}

func NewServer(v *vault.Vault, resolver netguard.Resolver, allowPrivateEndpoints bool) *Server {
	return &Server{vault: v, resolver: resolver, allowPrivate: allowPrivateEndpoints}
}
