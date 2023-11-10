package compute

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/fx"

	"github.com/bacalhau-project/bacalhau/pkg/config"
	"github.com/bacalhau-project/bacalhau/pkg/config/types"
	"github.com/bacalhau-project/bacalhau/pkg/ipfs"
	"github.com/bacalhau-project/bacalhau/pkg/lib/provider"
	"github.com/bacalhau-project/bacalhau/pkg/models"
	"github.com/bacalhau-project/bacalhau/pkg/publisher"
	ipfs_publisher "github.com/bacalhau-project/bacalhau/pkg/publisher/ipfs"
	noop_publisher "github.com/bacalhau-project/bacalhau/pkg/publisher/noop"
	"github.com/bacalhau-project/bacalhau/pkg/publisher/s3"
	"github.com/bacalhau-project/bacalhau/pkg/publisher/tracing"
	s3helper "github.com/bacalhau-project/bacalhau/pkg/s3"
)

func PublisherBuiltinProvider(lc fx.Lifecycle, cfg types.PublisherPluginConfig, ipfsClient ipfs.Client) (publisher.PublisherProvider, error) {
	provided := make(map[string]publisher.Publisher)
	for _, p := range cfg.Plugins {
		switch p.Name {
		case models.PublisherNoop:
			provided[p.Name] = tracing.Wrap(noop_publisher.NewNoopPublisher())
		case models.PublisherIPFS:
			ipfsPublisher, err := ipfs_publisher.NewIPFSPublisher(ipfsClient)
			if err != nil {
				return nil, err
			}
			provided[p.Name] = tracing.Wrap(ipfsPublisher)
		case models.PublisherS3:
			// TODO this needs to be configured better than hard codeing it.
			dir, err := os.MkdirTemp(config.GetStoragePath(), "bacalhau-s3-publisher")
			if err != nil {
				return nil, err
			}
			s3Publish, err := s3Publisher(dir)
			if err != nil {
				return nil, err
			}
			provided[p.Name] = tracing.Wrap(s3Publish)
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					if err := os.RemoveAll(dir); err != nil {
						return fmt.Errorf("unable to clean up S3 publisher directory: %w", err)
					}
					return nil
				},
			})
		default:
			return nil, fmt.Errorf("PublisherProvider %s unsupported", p.Name)
		}
	}

	return provider.NewMappedProvider(provided), nil
}

func s3Publisher(dir string) (*s3.Publisher, error) {
	cfg, err := s3helper.DefaultAWSConfig()
	if err != nil {
		return nil, err
	}
	clientProvider := s3helper.NewClientProvider(s3helper.ClientProviderParams{
		AWSConfig: cfg,
	})
	return s3.NewPublisher(s3.PublisherParams{
		LocalDir:       dir,
		ClientProvider: clientProvider,
	}), nil
}
