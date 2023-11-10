package common

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"

	"github.com/bacalhau-project/bacalhau/pkg/config"
	"github.com/bacalhau-project/bacalhau/pkg/config/types"
	bac_libp2p "github.com/bacalhau-project/bacalhau/pkg/libp2p"
	"github.com/bacalhau-project/bacalhau/pkg/libp2p/rcmgr"
)

func Libp2pHost(lc fx.Lifecycle, cfg types.Libp2pConfig) (host.Host, error) {
	privKey, err := config.GetLibp2pPrivKey()
	if err != nil {
		return nil, err
	}
	libp2pHost, err := bac_libp2p.NewHost(cfg.SwarmPort, privKey, rcmgr.DefaultResourceManager)
	if err != nil {
		return nil, fmt.Errorf("error creating libp2p host: %w", err)
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return libp2pHost.Close()
		},
	})
	return libp2pHost, nil
}
