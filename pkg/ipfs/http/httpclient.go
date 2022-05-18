package ipfs_http

import (
	"context"
	"fmt"

	"github.com/filecoin-project/bacalhau/pkg/system"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
	ma "github.com/multiformats/go-multiaddr"
)

type IPFSHttpClient struct {
	ctx     context.Context
	Address string
	Api     *httpapi.HttpApi
}

func NewIPFSHttpClient(
	ctx context.Context,
	address string,
) (*IPFSHttpClient, error) {
	addr, err := ma.NewMultiaddr(address)
	if err != nil {
		return nil, err
	}
	api, err := httpapi.NewApi(addr)
	if err != nil {
		return nil, err
	}
	return &IPFSHttpClient{
		ctx:     ctx,
		Address: address,
		Api:     api,
	}, nil
}

func (ipfsHttp *IPFSHttpClient) GetLocalAddrs() ([]ma.Multiaddr, error) {
	return ipfsHttp.Api.Swarm().LocalAddrs(ipfsHttp.ctx)
}

func (ipfsHttp *IPFSHttpClient) GetPeers() ([]iface.ConnectionInfo, error) {
	return ipfsHttp.Api.Swarm().Peers(ipfsHttp.ctx)
}

func (ipfsHttp *IPFSHttpClient) GetLocalAddrStrings() ([]string, error) {
	addressStrings := []string{}
	addrs, err := ipfsHttp.GetLocalAddrs()
	if err != nil {
		return addressStrings, nil
	}
	for _, addr := range addrs {
		addressStrings = append(addressStrings, addr.String())
	}
	return addressStrings, nil
}

// the libp2p addresses we should connect to
func (ipfsHttp *IPFSHttpClient) GetSwarmAddresses() ([]string, error) {
	addressStrings := []string{}
	addresses, err := ipfsHttp.GetLocalAddrStrings()
	if err != nil {
		return addressStrings, nil
	}
	peerId, err := ipfsHttp.GetPeerId()
	if err != nil {
		return addressStrings, nil
	}
	for _, address := range addresses {
		addressStrings = append(addressStrings, fmt.Sprintf("%s/p2p/%s", address, peerId))
	}
	return addressStrings, nil
}

func (ipfsHttp *IPFSHttpClient) GetPeerId() (string, error) {
	key, err := ipfsHttp.Api.Key().Self(ipfsHttp.ctx)
	if err != nil {
		return "", err
	}
	return key.ID().String(), nil
}

// return the peer ids of peers that provide the given cid
func (ipfsHttp *IPFSHttpClient) GetCidProviders(cid string) ([]string, error) {
	peerChan, err := ipfsHttp.Api.Dht().FindProviders(ipfsHttp.ctx, path.New(cid))
	if err != nil {
		return []string{}, err
	}
	providers := []string{}
	for addressInfo := range peerChan {
		providers = append(providers, addressInfo.ID.String())
	}
	return providers, nil
}

func (ipfsHttp *IPFSHttpClient) HasCidLocally(cid string) (bool, error) {
	peerId, err := ipfsHttp.GetPeerId()
	if err != nil {
		return false, err
	}
	providers, err := ipfsHttp.GetCidProviders(cid)
	if err != nil {
		return false, err
	}
	return system.StringArrayContains(providers, peerId), nil
}
