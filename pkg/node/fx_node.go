package node

import (
	"time"

	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"go.uber.org/fx"

	"github.com/bacalhau-project/bacalhau/pkg/config"
	"github.com/bacalhau-project/bacalhau/pkg/config/types"
	"github.com/bacalhau-project/bacalhau/pkg/ipfs"
	"github.com/bacalhau-project/bacalhau/pkg/node/modules/common"
	"github.com/bacalhau-project/bacalhau/pkg/repo"
	"github.com/bacalhau-project/bacalhau/pkg/version"
)

func NewFXNode(cfg types.NodeConfig, ipfsClient ipfs.Client, r *repo.FsRepo) (*Node, error) {

	// idk what this is for but we do it.
	identify.ActivationThresh = 2

	app := fx.New(
		fx.Provide(func() *repo.FsRepo { return r }),
		fx.Provide(func() ipfs.Client { return ipfsClient }),
		fx.Provide(func() types.NodeConfig { return cfg }),
		fx.Provide(common.ConfigFields),
		fx.Provide(common.Libp2pHost),
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

		// required for compute node.
		// fx.Provide(compute.ExecutorBuiltinProvider),
		// fx.Provide(compute.PublisherBuiltinProvider),
	)
}
