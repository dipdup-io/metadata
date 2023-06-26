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
	api       icore.CoreAPI
	node      *core.IpfsNode
	providers []Provider
	limit     int64
	wg        *sync.WaitGroup
}

// NewNode -
func NewNode(ctx context.Context, dir string, limit int64, blacklist []string, providers []Provider) (*Node, error) {
	api, node, err := spawn(ctx, dir, blacklist, providers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to spawn node")
	}
	return &Node{
		api:       api,
		node:      node,
		providers: providers,
		limit:     limit,
		wg:        new(sync.WaitGroup),
	}, nil
}

// Start -
func (n *Node) Start(ctx context.Context, bootstrap ...string) error {
	log.Info().Msg("going to connect to bootstrap nodes...")

	connected, err := n.api.Swarm().Peers(ctx)
	if err != nil {
		log.Warn().Msg("can't get perrs")
		return nil
	}
	for i := range connected {
		log.Info().
			Str("peer_id", connected[i].ID().String()).
			Str("address", connected[i].Address().String()).
			Msg("connected to peer")
	}

	n.wg.Add(1)
	go n.reconnect(ctx)

	return nil
}

// Close -
func (n *Node) Close() error {
	n.wg.Wait()
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
		Repo:   r,
		ExtraOpts: map[string]bool{
			"enable-gc": true,
		},
	})
	if err != nil {
		return nil, nil, err
	}

	api, err := coreapi.NewCoreAPI(node)
	return api, node, err
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
	cfg.Swarm.AddrFilters = blacklist
	cfg.Swarm.ConnMgr.HighWater = config.NewOptionalInteger(900)
	cfg.Swarm.ConnMgr.LowWater = config.NewOptionalInteger(600)
	cfg.Swarm.ConnMgr.GracePeriod = config.NewOptionalDuration(time.Minute * 5)
	cfg.Routing.AcceleratedDHTClient = true
	cfg.Routing.Type = config.NewOptionalString("auto")

	peers, err := providersToAddrInfo(providers)
	if err != nil {
		return "", errors.Wrap(err, "collecting providers info error")
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

func (n *Node) reconnect(ctx context.Context) {
	defer n.wg.Done()

	ticker := time.NewTicker(time.Minute * 3)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			peers, err := n.api.Swarm().Peers(ctx)
			if err != nil {
				log.Err(err).Msg("receiving peers")
				continue
			}

			for _, pi := range peers {
				log.Info().Str("peer_id", pi.ID().String()).Str("address", pi.Address().String()).Msg("connected to peer")
			}
		}
	}
}

func providersToAddrInfo(providers []Provider) ([]peer.AddrInfo, error) {
	peers := make([]peer.AddrInfo, 0)
	for i := range providers {
		id, err := peer.Decode(providers[i].ID)
		if err != nil {
			return nil, errors.Wrap(err, "providersToAddrInfo")
		}
		info := peer.AddrInfo{
			ID: id,
		}
		if providers[i].Address != "" {
			info.Addrs = []ma.Multiaddr{
				ma.StringCast(providers[i].Address),
			}
		}

		peers = append(peers, info)
	}
	return peers, nil
}
