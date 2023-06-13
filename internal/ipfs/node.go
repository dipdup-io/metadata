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
func NewNode(ctx context.Context, dir string, limit int64, blacklist []string) (*Node, error) {
	api, node, err := spawn(ctx, dir, blacklist)
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
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		"/ip4/104.131.131.82/udp/4001/quic/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		"/ip4/147.75.33.191/tcp/4001/p2p/12D3KooWPySxxWQjBgX9Jp6uAHQfVmdq8HG1gVvS1fRawHNSrmqW",
		"/ip4/147.75.80.9/tcp/4001/p2p/12D3KooWQYBPcvxFnnWzPGEx6JuBnrbF1FZq4jTahczuG2teEk1m",
		"/ip4/147.75.80.39/tcp/4001/p2p/12D3KooWDdzN3snjaMJEH9zuq3tjKUFpYHeSGNkiAreF6dQSbCiL",
		"/ip4/147.75.80.143/tcp/4001/p2p/12D3KooWEzCun34s9qpYEnKkG6epx2Ts9oVGRGnzCvM2s2edioLA",
		"/ip4/147.75.84.175/tcp/4001/p2p/12D3KooWDYVuVFGb9Yj6Gi9jWwSLqdnzZgqJg1a1scQMDc4R6RUJ",
		"/ip4/136.144.57.15/tcp/4001/p2p/12D3KooWJEfH2MB4RsUoaJPogDPRWbFTi8iehsxsqrQpiJwFNDrP",
		"/ip4/147.75.63.131/tcp/4001/p2p/12D3KooWHpE5KiQTkqbn8KbU88ZxwJxYJFaqP4mp9Z9bhNPhym9V",
		"/ip4/147.75.62.95/tcp/4001/p2p/12D3KooWBHvsSSKHeragACma3HUodK5FcPUpXccLu2vHooNsDf9k",
		"/ip4/147.75.50.141/tcp/4001/p2p/12D3KooWNCmYvqPbeXmNC4rnTr7hbuVtJKDNpL1vvNz6mq9Sr2Xf",
		"/ip4/147.28.147.193/tcp/4001/p2p/12D3KooWDRak1XzURGh9MvGR4EWaP9kcbmdoagAcGMcNxBXXLzTF",
		"/ip4/139.178.69.93/tcp/4001/p2p/12D3KooWRi18oHN1j8McxS9RMnuibcTwxu6VCTYHyLNH2R14qhTy",
		"/ip4/139.178.91.227/tcp/4001/p2p/12D3KooWKhPb9tSnCqBswVfC5EPE7iSTXhbF4Ywwz2MKg5UCagbr",
		"/ip4/139.178.91.231/tcp/4001/p2p/12D3KooWAdxvJCV5KXZ6zveTJmnYGrSzAKuLUKZYkZssLk7UKv4i",
		"/ip4/147.75.49.91/tcp/4001/p2p/12D3KooWRgXWwnZQJgdW1GHW7hJ5UvZ8MLp7HBCSWS596PypAs8M",
		"/ip4/139.178.88.145/tcp/4001/p2p/12D3KooWPbxiW4wFYHs7MwCQNqK9YVedH7QYZXJKMFVduhwR1Lcs",
		"/ip4/145.40.90.155/tcp/4001/p2p/12D3KooWSH5uLrYe7XSFpmnQj1NCsoiGeKSRCV7T5xijpX2Po2aT",
		"/ip4/147.75.85.47/tcp/4001/p2p/12D3KooWKd92H37a8gCDZPDAAGTYvEGAq7CNk1TcaCkcZedkTwFG",
		"/ip4/147.75.84.155/tcp/4001/p2p/12D3KooWJ59N9z5CyLTtcUTnuTKnRTEVxiztijiEAYbP16aZjQ3D",
		"/ip4/147.75.81.81/tcp/4001/p2p/12D3KooWLsSWaRsoCejZ6RMsGqdftpKbohczNqs3jvNfPgRwrMp2",
		"/ip4/147.75.101.41/tcp/4001/p2p/12D3KooWJc7GbwkjVg9voPNxdRnmEDS3i8NXNwRXD6kLattaMnE4",
		"/ip4/145.40.96.83/tcp/4001/p2p/12D3KooWCMMw5BKA5XHDJiuFitwparaYbMkidmxTCsJa8vXjt3yW",
		"/ip4/147.75.87.157/tcp/4001/p2p/12D3KooWDyGLvdtArZXZmf9JzPPCALXBHdUxzGYbMuHahWkUjFaf",
		"/ip4/147.75.85.127/tcp/4001/p2p/12D3KooWPzJxqGQWfaNqR9ft66e5c6NoBhDezXogLHeJQgD62Gvf",
		"/ip4/86.109.7.87/tcp/4001/p2p/12D3KooWFZmGztVoo2K1BcAoDEUmnp7zWFhaK5LcRHJ8R735T3eY",
		"/ip4/136.144.57.171/tcp/4001/p2p/12D3KooWKKcYZGRtQVdZVrTuARdJHLSBymB7dNN1R6PWwUT24qK4",
		"/ip4/147.28.147.173/tcp/4001/p2p/12D3KooWE8L7kAi4wTVcnSVgmHRxykpYX24Ck9toAifA9Dn2Q4Rw",
		"/ip4/145.40.67.73/tcp/4001/p2p/12D3KooWBszbJcQut3gW8CYPNgXsECiiRCMGm17xUb4Lr2iKQZEh",
		"/ip4/139.178.88.103/tcp/4001/p2p/12D3KooWQzwTxWF82GkjCCvU8RR55FjfTtoUTPYLJtJUPsHEN1VS",
		"/ip4/147.75.108.229/tcp/4001/p2p/12D3KooWHXKaRAKgQbPNqgpJwojmcHUajSFnQvHdKjPRbVHRhobC",
		"/ip4/139.178.68.73/tcp/4001/p2p/12D3KooWK2q1YYRBchmyAyyfLhKjvXMvYByt2zn6pbM3yA8Z2DJZ",
		"/ip4/147.75.108.191/tcp/4001/p2p/12D3KooWLSMVRxtFrRWofS6MjysgWnPh7iiFEGYeEAeBQceNrf4G",
		"/ip4/147.75.108.145/tcp/4001/p2p/12D3KooWQYb2nGCfqq4krBSZFRiFwjwZ8fjxsVpeMeGZoCJHR8Ch",
		"/ip4/139.178.94.209/tcp/4001/p2p/12D3KooWRZaQi1FWj7K1QBEMfzuvndS2gHPhT27yiwJHanEeuvBa",
		"/ip4/139.178.88.53/tcp/4001/p2p/12D3KooWBKx6Neuxph5yedV1F3YD6Cxd1eqGib6xUzT7BjdeaAao",
		"/ip4/104.248.44.204/tcp/4001/p2p/QmWaik1eJcGHq1ybTWe7sezRfqKNcDRNkeBaLnGwQJz1Cj",
		"/ip4/167.71.55.120/tcp/4001/p2p/QmNfpLrQQZr5Ns9FAJKpyzgnDL2GgC6xBug1yUZozKFgu4",
		"/ip4/64.225.105.42/tcp/4001/p2p/QmPo1ygpngghu5it8u4Mr3ym6SEU2Wp2wA66Z91Y1S1g29",
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
		return Data{}, errors.Wrapf(err, "invalid CID: %s", cid)
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

func spawn(ctx context.Context, dir string, blacklist []string) (icore.CoreAPI, *core.IpfsNode, error) {
	var onceErr error
	loadPluginsOnce.Do(func() {
		onceErr = setupPlugins("")
	})
	if onceErr != nil {
		return nil, nil, onceErr
	}

	repoPath, err := createRepository(dir, blacklist)
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

			connectCtx, cancel := context.WithTimeout(ctx, time.Second*20)
			defer cancel()
			err := ipfs.Swarm().Connect(connectCtx, *peerInfo)
			if err != nil {
				log.Warn().
					Str("peer", peerInfo.ID.String()).
					Msgf("failed to connect: %s", err)
				return
			}
			log.Info().
				Str("peer", peerInfo.ID.String()).
				Msg("connected")
		}(peerInfo)
	}
	wg.Wait()
	return nil
}

func createRepository(dir string, blacklist []string) (string, error) {
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
