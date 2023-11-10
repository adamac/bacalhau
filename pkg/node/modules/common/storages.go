package common

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
	s3helper "github.com/bacalhau-project/bacalhau/pkg/s3"
	"github.com/bacalhau-project/bacalhau/pkg/storage"
	"github.com/bacalhau-project/bacalhau/pkg/storage/inline"
	ipfs_storage "github.com/bacalhau-project/bacalhau/pkg/storage/ipfs"
	localdirectory "github.com/bacalhau-project/bacalhau/pkg/storage/local_directory"
	"github.com/bacalhau-project/bacalhau/pkg/storage/s3"
	"github.com/bacalhau-project/bacalhau/pkg/storage/tracing"
	"github.com/bacalhau-project/bacalhau/pkg/storage/url/urldownload"
)

func StorageBuiltinProvider(lc fx.Lifecycle, cfg types.StoragePluginConfig, client ipfs.Client) (storage.StorageProvider, error) {
	provided := make(map[string]storage.Storage)
	for _, s := range cfg.Plugins {
		switch s.Name {
		case models.StorageSourceIPFS:
			dir, err := os.MkdirTemp(config.GetStoragePath(), "bacalhau-ipfs")
			if err != nil {
				return nil, err
			}
			ipfsStorage, err := ipfs_storage.NewStorageFx(dir, client)
			if err != nil {
				return nil, err
			}
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					if err := os.RemoveAll(dir); err != nil {
						return fmt.Errorf("unable to clean up IPFS storage directory: %w", err)
					}
					return nil
				},
			})
			provided[s.Name] = tracing.Wrap(ipfsStorage)
		case models.StorageSourceURL:
			dir, err := os.MkdirTemp(config.GetStoragePath(), "bacalhau-url")
			if err != nil {
				return nil, err
			}
			urlStorage, err := urldownload.NewStorageFx(dir)
			if err != nil {
				return nil, err
			}
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					if err := os.RemoveAll(dir); err != nil {
						return fmt.Errorf("unable to clean up url storage directory: %w", err)
					}
					return nil
				},
			})
			provided[s.Name] = tracing.Wrap(urlStorage)
		case models.StorageSourceInline:
			provided[s.Name] = inline.NewStorage()
		case models.StorageSourceS3:
			dir, err := os.MkdirTemp(config.GetStoragePath(), "bacalhau-s3-input")
			if err != nil {
				return nil, err
			}
			s3Storage, err := s3StorageProvider(dir)
			if err != nil {
				return nil, err
			}
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					if err := os.RemoveAll(dir); err != nil {
						return fmt.Errorf("unable to clean up s3 storage directory: %w", err)
					}
					return nil
				},
			})
			provided[s.Name] = s3Storage
		case models.StorageSourceLocalDirectory:
			// TODO making one of these should not take this much boiler plate, if there is 1 params pass it.
			// we don't need params for our params of params.
			localDirStorage, err := localdirectory.NewStorageProvider(
				localdirectory.StorageProviderParams{
					AllowedPaths: localdirectory.ParseAllowPaths([]string{}),
				},
			)
			if err != nil {
				return nil, err
			}
			provided[s.Name] = localDirStorage
		default:
			return nil, fmt.Errorf("StorageProvider %s unsupported", s.Name)
		}
	}
	return provider.NewMappedProvider(provided), nil
}

func s3StorageProvider(dir string) (*s3.StorageProvider, error) {
	cfg, err := s3helper.DefaultAWSConfig()
	if err != nil {
		return nil, err
	}
	clientProvider := s3helper.NewClientProvider(s3helper.ClientProviderParams{
		AWSConfig: cfg,
	})
	s3Storage := s3.NewStorage(s3.StorageProviderParams{
		LocalDir:       dir,
		ClientProvider: clientProvider,
	})
	return s3Storage, nil
}
