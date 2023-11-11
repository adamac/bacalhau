package node

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"go.uber.org/fx"

	"github.com/bacalhau-project/bacalhau/pkg/config"
	"github.com/bacalhau-project/bacalhau/pkg/config/types"
	"github.com/bacalhau-project/bacalhau/pkg/ipfs"
	"github.com/bacalhau-project/bacalhau/pkg/node/modules/common"
	"github.com/bacalhau-project/bacalhau/pkg/node/modules/compute"
	"github.com/bacalhau-project/bacalhau/pkg/node/modules/requester"
	"github.com/bacalhau-project/bacalhau/pkg/repo"
	"github.com/bacalhau-project/bacalhau/pkg/routing"
	"github.com/bacalhau-project/bacalhau/pkg/routing/inmemory"
	"github.com/bacalhau-project/bacalhau/pkg/version"
)

func NewFXNode(ctx context.Context, cfg types.NodeConfig, ipfsClient ipfs.Client, r *repo.FsRepo) (*Node, error) {

	// idk what this is for but we do it.
	identify.ActivationThresh = 2

	node := new(Node)
	app := fx.New(
		common.ConfigFields(cfg),
		fx.Provide(func() *repo.FsRepo { return r }),
		fx.Provide(func() ipfs.Client { return ipfsClient }),
		fx.Provide(func() types.NodeConfig { return cfg }),
		fx.Provide(common.Libp2pHost),
		fx.Provide(func() routing.NodeInfoStore {
			// node info store that is used for both discovering compute nodes, as to find addresses of other nodes for routing requests.
			// TODO find a homme
			return inmemory.NewNodeInfoStore(inmemory.NodeInfoStoreParams{
				TTL: 10 * time.Minute,
			})
		}),
		common.NewPubSubService(common.PubSubConfig{
			Gossipsub: common.GossipSubConfig{
				TracerPath:   config.GetLibp2pTracerPath(),
				Threshold:    0.33,
				GlobalDecay:  2 * time.Minute,
				SourceDecay:  10 * time.Minute,
				PeerExchange: true,
			},
			NodeInfoPubSub: common.NodeInfoPubSubConfig{
				Topic:       NodeInfoTopic,
				IgnoreLocal: false,
			},
			NodeInfoSubscriber: common.NodeInfoSubscriberConfig{
				IgnoreErrors: true,
			},
			NodeInfoProvider: common.NodeInfoProviderConfig{
				Labels:  cfg.Labels,
				Version: *version.Get(),
			},
			NodeInfoPublisher: common.NodeInfoPublisherConfig{
				Interval: GetNodeInfoPublishConfig(),
			},
		}),
		fx.Provide(common.NewPublicAPIServer),
		// required for requester and computer
		fx.Provide(common.StorageBuiltinProvider),
		requester.Service(),

		// required for compute node.
		fx.Provide(compute.ExecutorBuiltinProvider),
		fx.Provide(compute.PublisherBuiltinProvider),
		compute.Service(),
	)

	if err := app.Start(ctx); err != nil {
		return nil, err
	}

	return node, nil
}
