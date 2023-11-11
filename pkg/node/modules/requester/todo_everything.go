package requester

import (
	"context"
	"time"

	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"

	"github.com/bacalhau-project/bacalhau/pkg/compute"
	"github.com/bacalhau-project/bacalhau/pkg/config/types"
	"github.com/bacalhau-project/bacalhau/pkg/eventhandler"
	"github.com/bacalhau-project/bacalhau/pkg/jobstore"
	"github.com/bacalhau-project/bacalhau/pkg/lib/backoff"
	"github.com/bacalhau-project/bacalhau/pkg/model"
	"github.com/bacalhau-project/bacalhau/pkg/models"
	"github.com/bacalhau-project/bacalhau/pkg/orchestrator"
	"github.com/bacalhau-project/bacalhau/pkg/orchestrator/evaluation"
	"github.com/bacalhau-project/bacalhau/pkg/orchestrator/planner"
	"github.com/bacalhau-project/bacalhau/pkg/orchestrator/retry"
	"github.com/bacalhau-project/bacalhau/pkg/orchestrator/scheduler"
	"github.com/bacalhau-project/bacalhau/pkg/orchestrator/selection/discovery"
	"github.com/bacalhau-project/bacalhau/pkg/orchestrator/selection/ranking"
	"github.com/bacalhau-project/bacalhau/pkg/orchestrator/selection/selector"
	"github.com/bacalhau-project/bacalhau/pkg/orchestrator/transformer"
	"github.com/bacalhau-project/bacalhau/pkg/publicapi"
	orchestrator_endpoint "github.com/bacalhau-project/bacalhau/pkg/publicapi/endpoint/orchestrator"
	requester_endpoint "github.com/bacalhau-project/bacalhau/pkg/publicapi/endpoint/requester"
	pubsub2 "github.com/bacalhau-project/bacalhau/pkg/pubsub"
	"github.com/bacalhau-project/bacalhau/pkg/pubsub/libp2p"
	"github.com/bacalhau-project/bacalhau/pkg/repo"
	"github.com/bacalhau-project/bacalhau/pkg/requester"
	"github.com/bacalhau-project/bacalhau/pkg/requester/pubsub/jobinfo"
	"github.com/bacalhau-project/bacalhau/pkg/routing"
	s3helper "github.com/bacalhau-project/bacalhau/pkg/s3"
	"github.com/bacalhau-project/bacalhau/pkg/storage"
	"github.com/bacalhau-project/bacalhau/pkg/system"
	"github.com/bacalhau-project/bacalhau/pkg/transport/bprotocol"
)

type RequesterService struct {
	// Visible for testing
	Endpoint       requester.Endpoint
	JobStore       jobstore.Store
	NodeDiscoverer orchestrator.NodeDiscoverer
	ComputeProxy   *bprotocol.ComputeProxy
	LocalCallback  compute.Callback
}

func NewRequesterService(endpoint *requester.BaseEndpoint, discover orchestrator.NodeDiscoverer, store jobstore.Store, proxy *bprotocol.ComputeProxy) *RequesterService {
	return &RequesterService{
		Endpoint:       endpoint,
		LocalCallback:  endpoint,
		NodeDiscoverer: discover,
		JobStore:       store,
		ComputeProxy:   proxy,
	}
}

func NewComputeProxy(h host.Host) *bprotocol.ComputeProxy {
	return bprotocol.NewComputeProxy(bprotocol.ComputeProxyParams{
		Host:          h,
		LocalEndpoint: nil,
	})
}

func NewJobStore(lc fx.Lifecycle, r *repo.FsRepo, h host.Host) (jobstore.Store, error) {
	store, err := r.InitJobStore(context.TODO(), h.ID().String())
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{OnStop: func(ctx context.Context) error {
		return store.Close(ctx)
	}})
	return store, nil
}

func NewContextProvider(lc fx.Lifecycle, h host.Host) *eventhandler.TracerContextProvider {
	provider := eventhandler.NewTracerContextProvider(h.ID().String())
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return provider.Shutdown()
		},
	})
	return provider
}

func NewJobEventConsumer(traceProvider *eventhandler.TracerContextProvider) *eventhandler.ChainedJobEventHandler {
	return eventhandler.NewChainedJobEventHandler(traceProvider)
}

func NewEventEmitter(eventHandler *eventhandler.ChainedJobEventHandler) orchestrator.EventEmitter {
	return orchestrator.NewEventEmitter(orchestrator.EventEmitterParams{EventConsumer: eventHandler})
}

func RegisterEventEmitterHandlers(
	lc fx.Lifecycle,
	h host.Host,
	emitter *eventhandler.ChainedJobEventHandler,
	provider *eventhandler.TracerContextProvider,
	requesterAPI *requester_endpoint.Endpoint,
	publisher *jobinfo.Publisher,
) error {
	// Register event handlers
	eventTracer, err := eventhandler.NewTracer()
	if err != nil {
		return err
	}
	lifecycleEventHandler := system.NewJobLifecycleEventHandler(h.ID().String())

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// order of event handlers is important as triggering some handlers might depend on the state of others.
			emitter.AddHandlers(
				// add tracing metadata to the context about the read event
				eventhandler.JobEventHandlerFunc(lifecycleEventHandler.HandleConsumedJobEvent),
				// ends the span for the job if received a terminal event
				provider,
				// record the event in a log
				eventTracer,
				// dispatches events to listening websockets
				requesterAPI,
				// publish job events to the network
				publisher,
			)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return eventTracer.Shutdown()
		},
	})
	return nil
}

func NewJobInfoPubSub(lc fx.Lifecycle, h host.Host, pubsub *libp2p_pubsub.PubSub) (*libp2p.PubSub[jobinfo.Envelope], error) {
	// PubSub to publish job events to the network
	jobInfoPubSub, err := libp2p.NewPubSub[jobinfo.Envelope](libp2p.PubSubParams{
		Host:        h,
		TopicName:   "bacalhau-job-info",
		PubSub:      pubsub,
		IgnoreLocal: true,
	})
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// TODO what is the point of this if we noop?
			return jobInfoPubSub.Subscribe(context.TODO(), pubsub2.NewNoopSubscriber[jobinfo.Envelope]())
		},
		OnStop: func(ctx context.Context) error {
			return jobInfoPubSub.Close(ctx)
		},
	})
	return jobInfoPubSub, nil
}

func NewNodeDiscoveryChain(store routing.NodeInfoStore) orchestrator.NodeDiscoverer {
	nodeDiscoveryChain := discovery.NewChain(true)
	nodeDiscoveryChain.Add(
		discovery.NewStoreNodeDiscoverer(discovery.StoreNodeDiscovererParams{
			Store: store,
		}),
	)
	return nodeDiscoveryChain
}

func NewJobInfoPublisher(store jobstore.Store, pubsub *libp2p.PubSub[jobinfo.Envelope]) *jobinfo.Publisher {
	return jobinfo.NewPublisher(jobinfo.PublisherParams{
		JobStore: store,
		PubSub:   pubsub,
	})
}

func NewNodeSelector(cfg types.RequesterConfig, store jobstore.Store, nodeDiscover orchestrator.NodeDiscoverer) *selector.NodeSelector {
	nodeRankerChain := ranking.NewChain()
	nodeRankerChain.Add(
		// rankers that act as filters and give a -1 score to nodes that do not match the filter
		ranking.NewEnginesNodeRanker(),
		ranking.NewPublishersNodeRanker(),
		ranking.NewStoragesNodeRanker(),
		ranking.NewLabelsNodeRanker(),
		ranking.NewMaxUsageNodeRanker(),
		// TODO add this to the config
		ranking.NewMinVersionNodeRanker(ranking.MinVersionNodeRankerParams{MinVersion: models.BuildVersionInfo{
			Major: "1", Minor: "0", GitVersion: "v1.0.4"},
		}),
		ranking.NewPreviousExecutionsNodeRanker(ranking.PreviousExecutionsNodeRankerParams{JobStore: store}),
		// arbitrary rankers
		ranking.NewRandomNodeRanker(ranking.RandomNodeRankerParams{
			RandomnessRange: cfg.NodeRankRandomnessRange,
		}),
	)

	// node selector
	return selector.NewNodeSelector(selector.NodeSelectorParams{
		NodeDiscoverer: nodeDiscover,
		NodeRanker:     nodeRankerChain,
	})
}

func NewEvaluationBroker(lc fx.Lifecycle, cfg types.EvaluationBrokerConfig) (orchestrator.EvaluationBroker, error) {
	broker, err := evaluation.NewInMemoryBroker(evaluation.InMemoryBrokerParams{
		VisibilityTimeout:    time.Duration(cfg.EvalBrokerVisibilityTimeout),
		InitialRetryDelay:    time.Duration(cfg.EvalBrokerInitialRetryDelay),
		SubsequentRetryDelay: time.Duration(cfg.EvalBrokerSubsequentRetryDelay),
		MaxReceiveCount:      cfg.EvalBrokerMaxRetryCount,
	})
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// use lifecycle
			broker.SetEnabled(true)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			broker.SetEnabled(false)
			return nil
		},
	})
	return broker, nil
}

func NewPlanner(
	h host.Host,
	proxy *bprotocol.ComputeProxy,
	store jobstore.Store,
	emitter orchestrator.EventEmitter,
) *planner.ChainedPlanner {
	// planners that execute the proposed plan by the scheduler
	// order of the planners is important as they are executed in order
	planners := planner.NewChain(
		// planner that persist the desired state as defined by the scheduler
		planner.NewStateUpdater(store),

		// planner that forwards the desired state to the compute nodes,
		// and updates the observed state if the compute node accepts the desired state
		planner.NewComputeForwarder(planner.ComputeForwarderParams{
			ID:             h.ID().String(),
			ComputeService: proxy,
			JobStore:       store,
		}),

		// planner that publishes events on job completion or failure
		planner.NewEventEmitter(planner.EventEmitterParams{
			ID:           h.ID().String(),
			EventEmitter: emitter,
		}),

		// logs job completion or failure
		planner.NewLoggingPlanner(),
	)
	return planners
}

// TODO this should accept a config
func NewRetryStrategy() *retry.Chain {
	retryStrategyChain := retry.NewChain()
	retryStrategyChain.Add(
		retry.NewFixedStrategy(retry.FixedStrategyParams{ShouldRetry: true}),
	)
	return retryStrategyChain
}

func NewInMemorySchedulerProvider(
	store jobstore.Store,
	planners *planner.ChainedPlanner,
	selector *selector.NodeSelector,
	retryStrat *retry.Chain,
) orchestrator.SchedulerProvider {
	batchServiceJobScheduler := scheduler.NewBatchServiceJobScheduler(scheduler.BatchServiceJobSchedulerParams{
		JobStore:      store,
		Planner:       planners,
		NodeSelector:  selector,
		RetryStrategy: retryStrat,
	})
	schedulerProvider := orchestrator.NewMappedSchedulerProvider(map[string]orchestrator.Scheduler{
		models.JobTypeBatch:   batchServiceJobScheduler,
		models.JobTypeService: batchServiceJobScheduler,
		models.JobTypeOps: scheduler.NewOpsJobScheduler(scheduler.OpsJobSchedulerParams{
			JobStore:     store,
			Planner:      planners,
			NodeSelector: selector,
		}),
		models.JobTypeDaemon: scheduler.NewDaemonJobScheduler(scheduler.DaemonJobSchedulerParams{
			JobStore:     store,
			Planner:      planners,
			NodeSelector: selector,
		}),
	})
	return schedulerProvider
}

func DispatchOrchestratorWorkers(lc fx.Lifecycle, cfg types.WorkerConfig, provider orchestrator.SchedulerProvider, broker orchestrator.EvaluationBroker) {
	workers := make([]*orchestrator.Worker, 0, cfg.WorkerCount)
	for i := 1; i <= cfg.WorkerCount; i++ {
		log.Debug().Msgf("Starting worker %d", i)
		// worker config the polls from the broker
		worker := orchestrator.NewWorker(orchestrator.WorkerParams{
			SchedulerProvider:     provider,
			EvaluationBroker:      broker,
			DequeueTimeout:        time.Duration(cfg.WorkerEvalDequeueTimeout),
			DequeueFailureBackoff: backoff.NewExponential(time.Duration(cfg.WorkerEvalDequeueBaseBackoff), time.Duration(cfg.WorkerEvalDequeueMaxBackoff)),
		})
		workers = append(workers, worker)
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			for _, worker := range workers {
				worker.Start(ctx)
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			for _, worker := range workers {
				worker.Stop()
			}
			return nil
		},
	})
}

func NewBaseEndpoint(
	cfg types.JobDefaults,
	h host.Host,
	broker orchestrator.EvaluationBroker,
	emitter orchestrator.EventEmitter,
	proxy *bprotocol.ComputeProxy,
	store jobstore.Store,
	storage storage.StorageProvider,
) (*requester.BaseEndpoint, error) {
	publicKey := h.Peerstore().PubKey(h.ID())
	marshaledPublicKey, err := crypto.MarshalPublicKey(publicKey)
	if err != nil {
		return nil, err
	}

	endpoint := requester.NewBaseEndpoint(&requester.BaseEndpointParams{
		ID:                         h.ID().String(),
		PublicKey:                  marshaledPublicKey,
		EvaluationBroker:           broker,
		Store:                      store,
		EventEmitter:               emitter,
		ComputeEndpoint:            proxy,
		StorageProviders:           storage,
		DefaultJobExecutionTimeout: time.Duration(cfg.ExecutionTimeout),
	})
	// TODO is this really the right place?
	// weird to have a one off method to call things on the host, seems like it needs a refactor, put it as a helper or delete
	bprotocol.NewCallbackHandler(bprotocol.CallbackHandlerParams{
		Host:     h,
		Callback: endpoint,
	})
	return endpoint, nil
}

// TODO why do we have two endpoints????????
func NewV2BaseEndpoint(
	cfg types.RequesterConfig,
	h host.Host,
	broker orchestrator.EvaluationBroker,
	emitter orchestrator.EventEmitter,
	proxy *bprotocol.ComputeProxy,
	store jobstore.Store,
) (*orchestrator.BaseEndpoint, error) {

	publicKey := h.Peerstore().PubKey(h.ID())
	marshaledPublicKey, err := crypto.MarshalPublicKey(publicKey)
	if err != nil {
		return nil, err
	}

	// result transformers that are applied to the result before it is returned to the user
	resultTransformers := transformer.ChainedTransformer[*models.SpecConfig]{}

	if !cfg.StorageProvider.S3.PreSignedURLDisabled {
		// S3 result signer
		s3Config, err := s3helper.DefaultAWSConfig()
		if err != nil {
			return nil, err
		}
		resultSigner := s3helper.NewResultSigner(s3helper.ResultSignerParams{
			ClientProvider: s3helper.NewClientProvider(s3helper.ClientProviderParams{
				AWSConfig: s3Config,
			}),
			Expiration: time.Duration(cfg.StorageProvider.S3.PreSignedURLExpiration),
		})
		resultTransformers = append(resultTransformers, resultSigner)
	}

	return orchestrator.NewBaseEndpoint(&orchestrator.BaseEndpointParams{
		ID:               h.ID().String(),
		EvaluationBroker: broker,
		Store:            store,
		EventEmitter:     emitter,
		ComputeProxy:     proxy,
		JobTransformer: transformer.ChainedTransformer[*models.Job]{
			transformer.JobFn(transformer.IDGenerator),
			transformer.NameOptional(),
			// TODO fix these duplicate types
			transformer.DefaultsApplier(transformer.JobDefaults{ExecutionTimeout: time.Duration(cfg.JobDefaults.ExecutionTimeout)}),
			transformer.RequesterInfo(h.ID().String(), marshaledPublicKey),
		},
		ResultTransformer: resultTransformers,
	}), nil

}

func DispatchHouseKeeping(lc fx.Lifecycle, cfg types.RequesterConfig, endpoint *requester.BaseEndpoint, store jobstore.Store, h host.Host) {
	housekeeping := requester.NewHousekeeping(requester.HousekeepingParams{
		Endpoint: endpoint,
		JobStore: store,
		NodeID:   h.ID().String(),
		Interval: time.Duration(cfg.HousekeepingBackgroundTaskInterval),
	})

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			housekeeping.Start(ctx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			housekeeping.Stop()
			return nil
		},
	})
}

func NewRequesterAPIServer(
	server *publicapi.Server,
	endpoint *requester.BaseEndpoint,
	store jobstore.Store,
	discover orchestrator.NodeDiscoverer,
) *requester_endpoint.Endpoint {
	// register debug info providers for the /debug endpoint
	debugInfoProviders := []model.DebugInfoProvider{
		discovery.NewDebugInfoProvider(discover),
	}

	// register requester public http apis
	requesterAPIServer := requester_endpoint.NewEndpoint(requester_endpoint.EndpointParams{
		Router:    server.Router,
		Requester: endpoint,
		JobStore:  store,
		// TODO this is taking the same thing in twice, move the Debug stuff to internal
		DebugInfoProviders: debugInfoProviders,
		NodeDiscoverer:     discover,
	})

	return requesterAPIServer
}

func RegisterOrchestratorEndpoint(server *publicapi.Server, endpoint *orchestrator.BaseEndpoint, jobStore jobstore.Store, nodeStore routing.NodeInfoStore) {
	orchestrator_endpoint.NewEndpoint(orchestrator_endpoint.EndpointParams{
		Router:       server.Router,
		Orchestrator: endpoint,
		JobStore:     jobStore,
		NodeStore:    nodeStore,
	})
}

func Service() fx.Option {
	return fx.Module("requester",
		fx.Provide(NewComputeProxy),
		fx.Provide(NewJobStore),
		fx.Provide(NewContextProvider),
		fx.Provide(NewJobEventConsumer),
		fx.Provide(NewEventEmitter),
		fx.Provide(NewJobInfoPubSub),
		fx.Provide(NewNodeDiscoveryChain),
		fx.Provide(NewJobInfoPublisher),
		fx.Provide(NewNodeSelector),
		fx.Provide(NewEvaluationBroker),
		fx.Provide(NewPlanner),
		fx.Provide(NewRetryStrategy),
		fx.Provide(NewInMemorySchedulerProvider),
		fx.Provide(NewBaseEndpoint),
		fx.Provide(NewV2BaseEndpoint),
		fx.Provide(NewRequesterAPIServer),
		fx.Provide(NewRequesterService),

		fx.Invoke(RegisterEventEmitterHandlers),
		fx.Invoke(DispatchHouseKeeping),
		fx.Invoke(RegisterOrchestratorEndpoint),
		fx.Invoke(DispatchHouseKeeping),
		fx.Invoke(DispatchOrchestratorWorkers),
	)
}
