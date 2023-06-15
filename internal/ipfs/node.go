package ipfs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	icore "github.com/ipfs/boxo/coreiface"
	icorepath "github.com/ipfs/boxo/coreiface/path"
	"github.com/ipfs/boxo/files"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/plugin/loader" // This package is needed so that all the preloaded plugins are loaded automatically
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Node -
type Node struct {
	api   icore.CoreAPI
	node  *core.IpfsNode
	limit int64
}

// NewNode -
func NewNode(ctx context.Context, dir string, limit int64, blacklist []string, providers []Provider) (*Node, error) {
	api, node, err := spawn(ctx, dir, blacklist, providers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to spawn node")
	}
	return &Node{api, node, limit}, nil
}

// Start -
func (n *Node) Start(ctx context.Context, bootstrap ...string) error {
	log.Info().Msg("going to connect to bootstrap nodes...")

	bootstrapNodes := []string{
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		"/ip4/104.131.131.82/udp/4001/quic/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		"/dns4/node1.preload.ipfs.io/tcp/443/wss/ipfs/Qmbut9Ywz9YEDrz8ySBSgWyJk41Uvm2QJPhwDJzJyGFsD6",
		"/dns4/node0.preload.ipfs.io/tcp/443/wss/ipfs/QmZMxNdpMkewiVZLMRxaNxUeZpDUb34pWjZ1kZvsd16Zic",
		"/dns4/production-ipfs-peer.pinata.cloud/tcp/3000/ws/p2p/Qma8ddFEQWEU8ijWvdxXm3nxU7oHsRtCykAaVz8WUYhiKn",
	}

	if len(bootstrap) > 0 {
		bootstrapNodes = append(bootstrapNodes, bootstrap...)
	}

	if err := connectToPeers(ctx, n.api, bootstrapNodes); err != nil {
		return errors.Wrap(err, "failed connect to peers")
	}

	return nil
}

// Close -
func (n *Node) Close() error {
	return n.node.Close()
}

// Get -
func (n *Node) Get(ctx context.Context, cid string) (Data, error) {
	cidObj := icorepath.New(cid)
	if err := cidObj.IsValid(); err != nil {
		return Data{}, errors.Wrapf(ErrInvalidCID, cid)
	}

	start := time.Now()
	rootNode, err := n.api.Unixfs().Get(ctx, cidObj)
	if err != nil {
		return Data{}, errors.Wrapf(err, "could not get file with CID: %s", cid)
	}
	defer rootNode.Close()
	responseTime := time.Since(start).Milliseconds()

	file := files.ToFile(rootNode)
	if file == nil {
		return Data{}, errors.Errorf("could not get file with CID: %s", cid)
	}

	data, err := io.ReadAll(io.LimitReader(file, n.limit))
	if err != nil {
		return Data{}, err
	}

	return Data{
		Raw:          data,
		Node:         "ipfs-metadata-node",
		ResponseTime: responseTime,
	}, nil
}

var loadPluginsOnce sync.Once

func spawn(ctx context.Context, dir string, blacklist []string, providers []Provider) (icore.CoreAPI, *core.IpfsNode, error) {
	var onceErr error
	loadPluginsOnce.Do(func() {
		onceErr = setupPlugins("")
	})
	if onceErr != nil {
		return nil, nil, onceErr
	}

	repoPath, err := createRepository(dir, blacklist, providers)
	if err != nil {
		return nil, nil, err
	}

	r, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, nil, err
	}

	node, err := core.NewNode(ctx, &core.BuildCfg{
		Online: true,
		// Routing: libp2p.DHTOption,
		Repo: r,
	})
	if err != nil {
		return nil, nil, err
	}

	api, err := coreapi.NewCoreAPI(node)
	return api, node, err
}

func connectToPeers(ctx context.Context, ipfs icore.CoreAPI, peers []string) error {
	peerInfos := make(map[peer.ID]*peer.AddrInfo)
	for _, addrStr := range peers {
		addr, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			return err
		}
		pii, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return err
		}
		pi, ok := peerInfos[pii.ID]
		if !ok {
			pi = &peer.AddrInfo{ID: pii.ID}
			peerInfos[pi.ID] = pi
		}
		pi.Addrs = append(pi.Addrs, pii.Addrs...)
	}

	var wg sync.WaitGroup
	wg.Add(len(peerInfos))
	for _, peerInfo := range peerInfos {
		go func(peerInfo *peer.AddrInfo) {
			defer wg.Done()
			log.Info().Str("peer", peerInfo.ID.String()).Msg("connecting...")

			connectCtx, cancel := context.WithTimeout(ctx, time.Second*30)
			defer cancel()
			err := ipfs.Swarm().Connect(connectCtx, *peerInfo)
			if err != nil {
				log.Warn().
					Str("peer", peerInfo.ID.String()).
					Msgf("failed to connect: %s", err)
				return
			}
		}(peerInfo)
	}
	wg.Wait()

	connected, err := ipfs.Swarm().Peers(ctx)
	if err != nil {
		log.Warn().Msg("can't get perrs")
		return nil
	}
	for i := range connected {
		log.Info().Str("peer_id", connected[i].ID().String()).Str("address", connected[i].Address().String()).Msg("connected to peer")
	}
	return nil
}

func createRepository(dir string, blacklist []string, providers []Provider) (string, error) {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				return "", fmt.Errorf("failed to get dir: %s", err)
			}
		} else {
			return "", err
		}
	}

	// Create a config with default options and a 2048 bit key
	cfg, err := config.Init(io.Discard, 2048)
	if err != nil {
		return "", err
	}

	cfg.Swarm.DisableBandwidthMetrics = true
	cfg.Swarm.Transports.Network.Relay = config.False
	cfg.Swarm.Transports.Network.QUIC = config.False
	cfg.Swarm.AddrFilters = blacklist

	peers := make([]peer.AddrInfo, 0)
	for i := range providers {
		id, err := peer.Decode(providers[i].ID)
		if err != nil {
			log.Err(err).Str("peer", providers[i].ID).Msg("invalid identity")
			continue
		}
		peers = append(peers, peer.AddrInfo{
			ID: id,
			Addrs: []ma.Multiaddr{
				ma.StringCast(providers[i].Address),
			},
		})
	}
	cfg.Peering = config.Peering{
		Peers: peers,
	}

	// Create the repo with the config
	if err = fsrepo.Init(dir, cfg); err != nil {
		return "", errors.Wrap(err, "failed to init node")
	}

	return dir, nil
}

func setupPlugins(externalPluginsPath string) error {
	// Load any external plugins if available on externalPluginsPath
	plugins, err := loader.NewPluginLoader(filepath.Join(externalPluginsPath, "plugins"))
	if err != nil {
		return fmt.Errorf("error loading plugins: %s", err)
	}

	// Load preloaded and external plugins
	if err := plugins.Initialize(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	if err := plugins.Inject(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	return nil
}
