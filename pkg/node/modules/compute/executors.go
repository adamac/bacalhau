package compute

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/bacalhau-project/bacalhau/pkg/config/types"
	"github.com/bacalhau-project/bacalhau/pkg/executor"
	"github.com/bacalhau-project/bacalhau/pkg/executor/docker"
	"github.com/bacalhau-project/bacalhau/pkg/executor/util"
	"github.com/bacalhau-project/bacalhau/pkg/executor/wasm"
	"github.com/bacalhau-project/bacalhau/pkg/lib/provider"
	"github.com/bacalhau-project/bacalhau/pkg/models"
)

func ExecutorPluginProvider(lc fx.Lifecycle, cfg types.ExecutorPluginConfig) (executor.ExecutorProvider, error) {
	pe := util.NewPluginExecutorManager()
	for _, c := range cfg.Plugins {
		if err := pe.RegisterPlugin(c); err != nil {
			return nil, err
		}
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return pe.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return pe.Stop(ctx)
		},
	})
	return pe, nil
}

func ExecutorBuiltinProvider(lc fx.Lifecycle, cfg types.ExecutorPluginConfig) (executor.ExecutorProvider, error) {
	provided := make(map[string]executor.Executor)
	for _, e := range cfg.Plugins {
		switch e.Name {
		case models.EngineDocker:
			dockerExecutor, err := docker.NewExecutor(context.TODO(), "dockerID-TODO")
			if err != nil {
				return nil, err
			}
			provided[e.Name] = dockerExecutor
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					return dockerExecutor.Shutdown(ctx)
				},
			})
		case models.EngineWasm:
			wasmExecutor, err := wasm.NewExecutor()
			if err != nil {
				return nil, err
			}
			provided[e.Name] = wasmExecutor
		default:
			return nil, fmt.Errorf("ExecutorProvider %s unsupported: %s", e)
		}

	}
	return provider.NewMappedProvider(provided), nil
}
