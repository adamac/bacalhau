package common

import (
	"github.com/labstack/echo/v4"
	"github.com/libp2p/go-libp2p/core/host"

	"github.com/bacalhau-project/bacalhau/pkg/config/types"
	"github.com/bacalhau-project/bacalhau/pkg/publicapi"
	"github.com/bacalhau-project/bacalhau/pkg/publicapi/endpoint/agent"
	"github.com/bacalhau-project/bacalhau/pkg/publicapi/endpoint/shared"
	"github.com/bacalhau-project/bacalhau/pkg/routing"
)

func NewPublicAPIServer(cfg types.APIConfig, h host.Host, nodeInfoProvider *routing.NodeInfoProvider) (*publicapi.Server, error) {
	// TODO don't do this here.
	cfg.ServerConfig.SkippedTimeoutPaths = append(cfg.ServerConfig.SkippedTimeoutPaths, []string{
		"/api/v1/requester/websocket/events",
		"/api/v1/requester/logs",
	}...)
	apiServer, err := publicapi.NewAPIServer(publicapi.ServerParams{
		Router:             echo.New(),
		Address:            cfg.Host,
		Port:               uint16(cfg.Port),
		HostID:             h.ID().String(),
		AutoCertDomain:     cfg.TLS.AutoCert,
		AutoCertCache:      cfg.TLS.AutoCertCachePath,
		TLSCertificateFile: cfg.TLS.ServerCertificate,
		TLSKeyFile:         cfg.TLS.ServerKey,
		Config:             cfg.ServerConfig,
	})
	if err != nil {
		return nil, err
	}

	// TODO what is the point of this, they return something, we ignore it and don't use it? Yay side effects...
	shared.NewEndpoint(shared.EndpointParams{
		Router:           apiServer.Router,
		NodeID:           h.ID().String(),
		PeerStore:        h.Peerstore(),
		NodeInfoProvider: nodeInfoProvider,
	})

	agent.NewEndpoint(agent.EndpointParams{
		Router:           apiServer.Router,
		NodeInfoProvider: nodeInfoProvider,
	})

	return apiServer, nil
}
