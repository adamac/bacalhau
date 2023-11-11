package common

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"

	"github.com/bacalhau-project/bacalhau/pkg/config/types"
	"github.com/bacalhau-project/bacalhau/pkg/publicapi"
	"github.com/bacalhau-project/bacalhau/pkg/publicapi/endpoint/agent"
	"github.com/bacalhau-project/bacalhau/pkg/publicapi/endpoint/shared"
	"github.com/bacalhau-project/bacalhau/pkg/routing"
)

func NewPublicAPIServer(lc fx.Lifecycle, cfg types.ServerAPIConfig, h host.Host, nodeInfoProvider *routing.NodeInfoProvider) (*publicapi.Server, error) {
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

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return apiServer.ListenAndServe(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return apiServer.Shutdown(ctx)
		},
	})

	return apiServer, nil
}
