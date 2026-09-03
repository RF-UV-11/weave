package httptool

import (
	dataaccessv1 "weave/core/gen/core/data_access/v1"
	"weave/core/netguard"
	"weave/core/vault"
)

type Server struct {
	dataaccessv1.UnimplementedHttpToolServiceServer
	vault          *vault.Vault
	gatewayBaseURL string
	resolver       netguard.Resolver
	allowPrivate   bool
}

func NewServer(v *vault.Vault, gatewayBaseURL string, resolver netguard.Resolver, allowPrivateEndpoints bool) *Server {
	return &Server{vault: v, gatewayBaseURL: gatewayBaseURL, resolver: resolver, allowPrivate: allowPrivateEndpoints}
}
