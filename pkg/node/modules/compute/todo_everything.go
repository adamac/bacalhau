package compute

import (
	"context"
	"net/url"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"

	"github.com/bacalhau-project/bacalhau/pkg/bidstrategy"
	"github.com/bacalhau-project/bacalhau/pkg/bidstrategy/resource"
	"github.com/bacalhau-project/bacalhau/pkg/bidstrategy/semantic"
	"github.com/bacalhau-project/bacalhau/pkg/compute"
	"github.com/bacalhau-project/bacalhau/pkg/compute/capacity"
	"github.com/bacalhau-project/bacalhau/pkg/compute/capacity/disk"
	"github.com/bacalhau-project/bacalhau/pkg/compute/logstream"
	"github.com/bacalhau-project/bacalhau/pkg/compute/sensors"
	"github.com/bacalhau-project/bacalhau/pkg/compute/store"
	"github.com/bacalhau-project/bacalhau/pkg/config/types"
	"github.com/bacalhau-project/bacalhau/pkg/executor"
	executor_util "github.com/bacalhau-project/bacalhau/pkg/executor/util"
	"github.com/bacalhau-project/bacalhau/pkg/model"
	"github.com/bacalhau-project/bacalhau/pkg/models"
	"github.com/bacalhau-project/bacalhau/pkg/publicapi"
	compute_endpoint "github.com/bacalhau-project/bacalhau/pkg/publicapi/endpoint/compute"
	"github.com/bacalhau-project/bacalhau/pkg/publisher"
	"github.com/bacalhau-project/bacalhau/pkg/repo"
	"github.com/bacalhau-project/bacalhau/pkg/storage"
	"github.com/bacalhau-project/bacalhau/pkg/transport/bprotocol"
)

func NewExecutionStore(lc fx.Lifecycle, r *repo.FsRepo, h host.Host) (store.ExecutionStore, error) {
	store, err := r.InitExecutionStore(context.TODO(), h.ID().String())
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return store.Close(ctx)
		},
	})

	return store, nil
}

func NewRunningCapacityTracker(cfg types.CapacityConfig) *capacity.LocalTracker {
	return capacity.NewLocalTracker(capacity.LocalTrackerParams{
		MaxCapacity: models.Resources(model.ParseResourceUsageConfig(cfg.TotalResourceLimits)),
	})
}

func NewEnqueuedCapacityTracker(cfg types.CapacityConfig) *capacity.LocalTracker {
	return capacity.NewLocalTracker(capacity.LocalTrackerParams{
		MaxCapacity: models.Resources(model.ParseResourceUsageConfig(cfg.QueueResourceLimits)),
	})
}

func NewComputeCallback(h host.Host) *bprotocol.CallbackProxy {
	return bprotocol.NewCallbackProxy(bprotocol.CallbackProxyParams{
		Host: h,
	})
}

func NewResultsPath(lc fx.Lifecycle) (*compute.ResultsPath, error) {
	path, err := compute.NewResultsPath()
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{OnStop: func(ctx context.Context) error {
		return path.Close()
	}})
	return path, nil
}

func NewBaseExecutor(
	h host.Host,
	callback *bprotocol.CallbackProxy,
	store store.ExecutionStore,
	storages storage.StorageProvider,
	executors executor.ExecutorProvider,
	publishers publisher.PublisherProvider,
	resultsPath *compute.ResultsPath,
) *compute.BaseExecutor {
	return compute.NewBaseExecutor(compute.BaseExecutorParams{
		ID:          h.ID().String(),
		Callback:    callback,
		Store:       store,
		Storages:    storages,
		Executors:   executors,
		Publishers:  publishers,
		ResultsPath: *resultsPath,
		// TODO this value is never set by anything in the NewComputeNode method, remove?
		FailureInjectionConfig: model.FailureInjectionComputeConfig{IsBadActor: false},
	})
}

type executorBufferParams struct {
	fx.In

	Cfg            types.JobTimeoutConfig
	H              host.Host
	Base           *compute.BaseExecutor
	Callback       *bprotocol.CallbackProxy
	RunningTracker *capacity.LocalTracker `name:"running"`
	EnqueueTracker *capacity.LocalTracker `name:"enqueue"`
}

func NewExecutorBuffer(
	params executorBufferParams,
) *compute.ExecutorBuffer {
	return compute.NewExecutorBuffer(compute.ExecutorBufferParams{
		ID:                         params.H.ID().String(),
		DelegateExecutor:           params.Base,
		Callback:                   params.Callback,
		RunningCapacityTracker:     params.RunningTracker,
		EnqueuedCapacityTracker:    params.EnqueueTracker,
		DefaultJobExecutionTimeout: time.Duration(params.Cfg.DefaultJobExecutionTimeout),
	})
}

func NewRunningExecutionsInfoProvider(runner *compute.ExecutorBuffer) *sensors.RunningExecutionsInfoProvider {
	return sensors.NewRunningExecutionsInfoProvider(sensors.RunningExecutionsInfoProviderParams{
		Name:          "ActiveJobs",
		BackendBuffer: runner,
	})
}

func InvokeLoggingSensor(lc fx.Lifecycle, cfg types.LoggingConfig, provider *sensors.RunningExecutionsInfoProvider) {
	// bail if there isn't an interval
	if !(cfg.LogRunningExecutionsInterval > 0) {
		return
	}
	logSensor := sensors.NewLoggingSensor(sensors.LoggingSensorParams{
		InfoProvider: provider,
		Interval:     time.Duration(cfg.LogRunningExecutionsInterval),
	})

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// TODO do we need a stop method or is this context canceled when the app closes?
			logSensor.Start(ctx)
			return nil
		},
	})
}

func NewCapacityCalculator(cfg types.CapacityConfig, storages storage.StorageProvider) *capacity.ChainedUsageCalculator {
	return capacity.NewChainedUsageCalculator(capacity.ChainedUsageCalculatorParams{
		Calculators: []capacity.UsageCalculator{
			capacity.NewDefaultsUsageCalculator(capacity.DefaultsUsageCalculatorParams{
				Defaults: models.Resources(model.ParseResourceUsageConfig(cfg.DefaultJobResourceLimits)),
			}),
			disk.NewDiskUsageCalculator(disk.DiskUsageCalculatorParams{
				Storages: storages,
			}),
		},
	})
}

// TODO this will break tests because we are ignoring the ComputeConfig BidSemanticStrategy
// TODO these should all be provided on their own and then warpped up into a chained strat?
func NewSemanticBidStrategy(
	timeoutCfg types.JobTimeoutConfig,
	policyCfg model.JobSelectionPolicy,
	storages storage.StorageProvider,
	publishers publisher.PublisherProvider,
	executors executor.ExecutorProvider,
) bidstrategy.SemanticBidStrategy {
	return semantic.NewChainedSemanticBidStrategy(
		executor_util.NewExecutorSpecificBidStrategy(executors),
		semantic.NewNetworkingStrategy(policyCfg.AcceptNetworkedJobs),
		semantic.NewExternalCommandStrategy(semantic.ExternalCommandStrategyParams{
			Command: policyCfg.ProbeExec,
		}),
		semantic.NewExternalHTTPStrategy(semantic.ExternalHTTPStrategyParams{
			URL: policyCfg.ProbeHTTP,
		}),
		semantic.NewStatelessJobStrategy(semantic.StatelessJobStrategyParams{
			RejectStatelessJobs: policyCfg.RejectStatelessJobs,
		}),
		semantic.NewInputLocalityStrategy(semantic.InputLocalityStrategyParams{
			Locality: semantic.JobSelectionDataLocality(policyCfg.Locality),
			Storages: storages,
		}),
		semantic.NewProviderInstalledStrategy(
			publishers,
			func(j *models.Job) string { return j.Task().Publisher.Type },
		),
		semantic.NewStorageInstalledBidStrategy(storages),
		semantic.NewTimeoutStrategy(semantic.TimeoutStrategyParams{
			MaxJobExecutionTimeout:                time.Duration(timeoutCfg.MaxJobExecutionTimeout),
			MinJobExecutionTimeout:                time.Duration(timeoutCfg.MinJobExecutionTimeout),
			JobExecutionTimeoutClientIDBypassList: timeoutCfg.JobExecutionTimeoutClientIDBypassList,
		}),
		// TODO XXX: don't hardcode networkSize, calculate this dynamically from
		//  libp2p instead somehow. https://github.com/bacalhau-project/bacalhau/issues/512
		semantic.NewDistanceDelayStrategy(semantic.DistanceDelayStrategyParams{
			NetworkSize: 1,
		}),
	)
}

type resourceBidStrategyParams struct {
	fx.In

	Cfg            types.CapacityConfig
	Executors      executor.ExecutorProvider
	RunningTracker *capacity.LocalTracker `name:"running"`
	EnqueueTracker *capacity.LocalTracker `name:"enqueue"`
}

func NewResourceBidStrategy(params resourceBidStrategyParams) bidstrategy.ResourceBidStrategy {
	return resource.NewChainedResourceBidStrategy(
		executor_util.NewExecutorSpecificBidStrategy(params.Executors),
		resource.NewMaxCapacityStrategy(resource.MaxCapacityStrategyParams{
			MaxJobRequirements: models.Resources(model.ParseResourceUsageConfig(params.Cfg.JobResourceLimits)),
		}),
		resource.NewAvailableCapacityStrategy(resource.AvailableCapacityStrategyParams{
			RunningCapacityTracker:  params.RunningTracker,
			EnqueuedCapacityTracker: params.EnqueueTracker,
		}),
	)
}

func NewLogStreamServer(h host.Host, store store.ExecutionStore, executors executor.ExecutorProvider) *logstream.LogStreamServer {
	return logstream.NewLogStreamServer(logstream.LogStreamServerOptions{
		// TODO this context is only used for logging, canceling it is a noop
		Ctx:            context.TODO(),
		Host:           h,
		ExecutionStore: store,
		//
		Executors: executors,
	})
}

func NewNodeInfoProvider(
	cfg types.CapacityConfig,
	executors executor.ExecutorProvider,
	storages storage.StorageProvider,
	publishers publisher.PublisherProvider,
	runningTracker *capacity.LocalTracker,
	bufferRunner *compute.ExecutorBuffer,
) *compute.NodeInfoProvider {
	return compute.NewNodeInfoProvider(compute.NodeInfoProviderParams{
		Executors:          executors,
		Publisher:          publishers,
		Storages:           storages,
		CapacityTracker:    runningTracker,
		ExecutorBuffer:     bufferRunner,
		MaxJobRequirements: models.Resources(model.ParseResourceUsageConfig(cfg.JobResourceLimits)),
	})
}

func NewBidder(
	h host.Host,
	semanticStrategy bidstrategy.SemanticBidStrategy,
	resourceStrategy bidstrategy.ResourceBidStrategy,
	store store.ExecutionStore,
	callback *bprotocol.CallbackProxy,
	bufferRunner *compute.ExecutorBuffer,
	server *publicapi.Server,
) compute.Bidder {
	return compute.NewBidder(compute.BidderParams{
		NodeID:           h.ID().String(),
		SemanticStrategy: semanticStrategy,
		ResourceStrategy: resourceStrategy,
		Store:            store,
		Callback:         callback,
		Executor:         bufferRunner,
		GetApproveURL: func() *url.URL {
			return server.GetURI().JoinPath("/api/v1/compute/approve")
		},
	})

}

func NewBaseEndpoint(
	h host.Host,
	store store.ExecutionStore,
	capacityCalculator *capacity.ChainedUsageCalculator,
	bidder compute.Bidder,
	bufferRunner *compute.ExecutorBuffer,
	logserver *logstream.LogStreamServer,
) compute.BaseEndpoint {
	return compute.NewBaseEndpoint(compute.BaseEndpointParams{
		ID:              h.ID().String(),
		ExecutionStore:  store,
		UsageCalculator: capacityCalculator,
		Bidder:          bidder,
		Executor:        bufferRunner,
		LogServer:       *logserver,
	})
}

func RegisterComputeHandler(h host.Host, baseEndpoint compute.BaseEndpoint) {
	bprotocol.NewComputeHandler(bprotocol.ComputeHandlerParams{
		Host:            h,
		ComputeEndpoint: baseEndpoint,
	})
}

func NewStartup(lc fx.Lifecycle, store store.ExecutionStore, bufferRunner *compute.ExecutorBuffer) *compute.Startup {
	startup := compute.NewStartup(store, bufferRunner)

	lc.Append(fx.Hook{OnStart: func(ctx context.Context) error {
		return startup.Execute(ctx)
	}})

	return startup
}

func RegisterComputeEndpoint(
	server *publicapi.Server,
	bidder compute.Bidder,
	store store.ExecutionStore,
	provider *sensors.RunningExecutionsInfoProvider,
) {
	// register debug info providers for the /debug endpoint
	debugInfoProviders := []model.DebugInfoProvider{
		provider,
		sensors.NewCompletedJobs(store),
	}

	// register compute public http apis
	compute_endpoint.NewEndpoint(compute_endpoint.EndpointParams{
		Router:             server.Router,
		Bidder:             bidder,
		Store:              store,
		DebugInfoProviders: debugInfoProviders,
	})
}

type ComputeService struct {
	// Visible for testing
	ID                  string
	LocalEndpoint       compute.Endpoint
	Capacity            capacity.Tracker `name:"running"`
	ExecutionStore      store.ExecutionStore
	Executors           executor.ExecutorProvider
	Storages            storage.StorageProvider
	LogServer           *logstream.LogStreamServer
	Bidder              compute.Bidder
	computeCallback     *bprotocol.CallbackProxy
	computeInfoProvider *compute.NodeInfoProvider
}

func NewComputeService(
	h host.Host,
	localEndpoint compute.Endpoint,
	capacity capacity.Tracker,
	executionStore store.ExecutionStore,
	executors executor.ExecutorProvider,
	storages storage.StorageProvider,
	logServer *logstream.LogStreamServer,
	bidder compute.Bidder,
	computeCallback *bprotocol.CallbackProxy,
	computeInfoProvider *compute.NodeInfoProvider,
) *ComputeService {
	return &ComputeService{
		ID:                  h.ID().String(),
		LocalEndpoint:       localEndpoint,
		Capacity:            capacity,
		ExecutionStore:      executionStore,
		Executors:           executors,
		Storages:            storages,
		LogServer:           logServer,
		Bidder:              bidder,
		computeCallback:     computeCallback,
		computeInfoProvider: computeInfoProvider,
	}
}

func Service() fx.Option {
	return fx.Module("compute",
		fx.Provide(NewExecutionStore),
		fx.Provide(fx.Annotated{
			Name:   "running",
			Target: NewRunningCapacityTracker,
		}),
		fx.Provide(fx.Annotated{
			Name:   "enqueue",
			Target: NewEnqueuedCapacityTracker,
		}),
		fx.Provide(NewComputeCallback),
		fx.Provide(NewResultsPath),
		fx.Provide(NewBaseExecutor),
		fx.Provide(NewExecutorBuffer),
		fx.Provide(NewRunningExecutionsInfoProvider),
		fx.Provide(NewCapacityCalculator),
		fx.Provide(NewSemanticBidStrategy),
		fx.Provide(NewResourceBidStrategy),
		fx.Provide(NewLogStreamServer),
		fx.Provide(NewNodeInfoProvider),
		fx.Provide(NewBidder),
		fx.Provide(NewBaseEndpoint),
		fx.Provide(NewStartup),
		fx.Provide(NewComputeService),

		fx.Invoke(InvokeLoggingSensor),
		fx.Invoke(RegisterComputeHandler),
		fx.Invoke(RegisterComputeEndpoint),
	)
}
