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

func NewPublicAPIServer(cfg types.ServerAPIConfig, h host.Host, nodeInfoProvider *routing.NodeInfoProvider) (*publicapi.Server, error) {
	apiServer, err := publicapi.NewAPIServer(publicapi.ServerParams{
		Router: echo.New(),
		HostID: h.ID().String(),
		Config: cfg,
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
