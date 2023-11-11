package common

import (
	"go.uber.org/fx"

	"github.com/bacalhau-project/bacalhau/pkg/config/types"
	"github.com/bacalhau-project/bacalhau/pkg/model"
)

func ConfigFields(cfg types.NodeConfig) fx.Option {
	return fx.Options(
		fx.Provide(func() types.Libp2pConfig { return cfg.Libp2p }),
		fx.Provide(func() types.ComputeConfig { return cfg.Compute }),
		fx.Provide(func() types.RequesterConfig { return cfg.Requester }),
		fx.Provide(func() types.ExecutorPluginConfig { return cfg.Compute.Executors }),
		fx.Provide(func() types.StoragePluginConfig { return cfg.Compute.Storages }),
		fx.Provide(func() types.PublisherPluginConfig { return cfg.Compute.Publishers }),
		fx.Provide(func() types.ServerAPIConfig { return cfg.ServerAPI }),
		fx.Provide(func() types.JobDefaults { return cfg.Requester.JobDefaults }),
		fx.Provide(func() types.EvaluationBrokerConfig { return cfg.Requester.EvaluationBroker }),
		fx.Provide(func() types.LoggingConfig { return cfg.Compute.Logging }),
		fx.Provide(func() types.JobTimeoutConfig { return cfg.Compute.JobTimeouts }),
		fx.Provide(func() types.CapacityConfig { return cfg.Compute.Capacity }),
		fx.Provide(func() model.JobSelectionPolicy { return cfg.Compute.JobSelection }),
	)
}
