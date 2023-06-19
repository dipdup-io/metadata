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
	"github.com/ipfs/go-cid"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/plugin/loader" // This package is needed so that all the preloaded plugins are loaded automatically
	"github.com/ipfs/kubo/repo/fsrepo"
	p2p "github.com/libp2p/go-libp2p"
	kadDHT "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Node -
type Node struct {
	api   icore.CoreAPI
	node  *core.IpfsNode
	dht   *kadDHT.IpfsDHT
	limit int64
	wg    *sync.WaitGroup
}

// NewNode -
func NewNode(ctx context.Context, dir string, limit int64, blacklist []string, providers []Provider) (*Node, error) {
	api, node, err := spawn(ctx, dir, blacklist, providers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to spawn node")
	}
	host, err := p2p.New()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create p2p host")
	}
	dht, err := kadDHT.New(ctx, host, kadDHT.Mode(kadDHT.ModeClient), kadDHT.BootstrapPeers(kadDHT.GetDefaultBootstrapPeerAddrInfos()...))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create dht client")
	}
	return &Node{api, node, dht, limit, new(sync.WaitGroup)}, nil
}

// Start -
func (n *Node) Start(ctx context.Context, bootstrap ...string) error {
	log.Info().Msg("going to connect to bootstrap nodes...")

	if err := n.dht.Bootstrap(ctx); err != nil {
		return errors.Wrap(err, "dht client connection to bootstrap")
	}

	bootstrapNodes := []string{
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		"/ip4/104.131.131.82/udp/4001/quic/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		"/ip4/104.248.44.204/tcp/4001/p2p/QmWaik1eJcGHq1ybTWe7sezRfqKNcDRNkeBaLnGwQJz1Cj",
		"/ip4/167.71.55.120/tcp/4001/p2p/QmNfpLrQQZr5Ns9FAJKpyzgnDL2GgC6xBug1yUZozKFgu4",
		"/ip4/147.75.33.191/tcp/4001/p2p/12D3KooWPySxxWQjBgX9Jp6uAHQfVmdq8HG1gVvS1fRawHNSrmqW",
		"/ip4/147.75.80.9/tcp/4001/p2p/12D3KooWQYBPcvxFnnWzPGEx6JuBnrbF1FZq4jTahczuG2teEk1m",
		"/ip4/147.75.80.143/tcp/4001/p2p/12D3KooWEzCun34s9qpYEnKkG6epx2Ts9oVGRGnzCvM2s2edioLA",
		"/ip4/147.75.84.119/tcp/4001/p2p/12D3KooWQE3CWA3MJ1YhrYNP8EE3JErGbrCtpKRkFrWgi45nYAMn",
		"/ip4/147.75.84.173/tcp/4001/p2p/12D3KooWSafoW6yrSL7waghFAaiCqGy5mdjpQx4jn4CRNqbG7eqG",
		"/ip4/136.144.57.15/tcp/4001/p2p/12D3KooWJEfH2MB4RsUoaJPogDPRWbFTi8iehsxsqrQpiJwFNDrP",
		"/ip4/147.75.63.131/tcp/4001/p2p/12D3KooWHpE5KiQTkqbn8KbU88ZxwJxYJFaqP4mp9Z9bhNPhym9V",
		"/ip4/147.75.62.95/tcp/4001/p2p/12D3KooWBHvsSSKHeragACma3HUodK5FcPUpXccLu2vHooNsDf9k",
		"/ip4/147.75.50.77/tcp/4001/p2p/12D3KooWMaTJKNwQJyP1fw3ftGb5uqqM2U24Kam8aWqMRXzWHNiF",
		"/ip4/147.75.50.141/tcp/4001/p2p/12D3KooWNCmYvqPbeXmNC4rnTr7hbuVtJKDNpL1vvNz6mq9Sr2Xf",
		"/ip4/147.28.147.193/tcp/4001/p2p/12D3KooWDRak1XzURGh9MvGR4EWaP9kcbmdoagAcGMcNxBXXLzTF",
		"/ip4/139.178.69.93/tcp/4001/p2p/12D3KooWRi18oHN1j8McxS9RMnuibcTwxu6VCTYHyLNH2R14qhTy",
		"/ip4/139.178.91.227/tcp/4001/p2p/12D3KooWKhPb9tSnCqBswVfC5EPE7iSTXhbF4Ywwz2MKg5UCagbr",
		"/ip4/139.178.91.231/tcp/4001/p2p/12D3KooWAdxvJCV5KXZ6zveTJmnYGrSzAKuLUKZYkZssLk7UKv4i",
		"/ip4/147.75.49.91/tcp/4001/p2p/12D3KooWRgXWwnZQJgdW1GHW7hJ5UvZ8MLp7HBCSWS596PypAs8M",
		"/ip4/139.178.88.145/tcp/4001/p2p/12D3KooWPbxiW4wFYHs7MwCQNqK9YVedH7QYZXJKMFVduhwR1Lcs",
		"/ip4/145.40.90.155/tcp/4001/p2p/12D3KooWSH5uLrYe7XSFpmnQj1NCsoiGeKSRCV7T5xijpX2Po2aT",
		"/dns4/node1.preload.ipfs.io/tcp/443/wss/ipfs/Qmbut9Ywz9YEDrz8ySBSgWyJk41Uvm2QJPhwDJzJyGFsD6",
		"/dns4/node0.preload.ipfs.io/tcp/443/wss/ipfs/QmZMxNdpMkewiVZLMRxaNxUeZpDUb34pWjZ1kZvsd16Zic",
		"/dns4/production-ipfs-peer.pinata.cloud/tcp/3000/ws/p2p/Qma8ddFEQWEU8ijWvdxXm3nxU7oHsRtCykAaVz8WUYhiKn",
		"/dnsaddr/elastic.dag.house/tcp/443/wss/p2p/bafzbeibhqavlasjc7dvbiopygwncnrtvjd2xmryk5laib7zyjor6kf3avm",
		"/ip4/141.94.193.54/tcp/4001/p2p/12D3KooWAJJJwXsB5b68cbq69KpXiKqQAgTKssg76heHkg6mo2qB",
		"/ip4/18.232.70.69/tcp/4001/p2p/12D3KooWAYgR87jsuQMUno9MXQHGv3A7GGf4wLPVQhSG7jPNtejk",
		"/ip4/94.130.71.31/tcp/4001/p2p/12D3KooWJFBXTQaRhU3JJo5JiS4rHusKB7KuWmz1nfRjrkViaaMQ",
		"/ip4/148.113.152.143/tcp/4001/p2p/12D3KooWBYznKkpGnjj2JSdbbV4nSgomi4eDSDyhfbg7962HQDD1",
		"/ip4/75.101.168.30/tcp/4001/p2p/QmZNWqSqBRVkKkCF7UUVpwmKaNoV3yCgAcySiu33EMp1SZ",
		"/ip4/54.163.158.127/tcp/4001/p2p/QmU5SHDWb97oSnxNzWwCdoo5mbHcBhnhhbZvJraFWPsTsd",
		"/ip4/44.212.36.159/tcp/4001/p2p/Qme7qwEin3prdL4usjnHca7Wk2iCHd6VkTqXb337tYW3b9",
		"/ip4/54.162.86.85/tcp/4001/p2p/QmbTcHDGsFY7C8LJFJvxVDitjj9vjnv2SCtMPpCCPGUjGa",
		"/ip4/54.209.21.103/tcp/4001/p2p/QmTfhyfLC59LNFBKgdvoMkA9R6VgAte1h2gCs8XiFzDfFS",
		"/ip4/54.144.87.112/tcp/4001/p2p/12D3KooWNTYSAYUWwc7QwPkAajW9UtLeatSMArRfLRJfAHwsoewH",
		"/dns4/ipfs-swarm.fxhash2.xyz/tcp/4001/p2p/12D3KooWBpazXqzm5UnDtpTFbTkUJfXRHCCydnuFp2uq6vdzKVnF",
		"/ip4/54.152.12.107/tcp/4001/p2p/12D3KooWRiTA6r3uvnB87ntQ5eRZwTeVFGBjPUBuAHhdwTMW8Hss",
		"/ip4/3.86.210.183/tcp/4001/p2p/12D3KooWGrmsCy5FSXTovD235fAyp25SpPuzqtQvjqNDDZj4rAmb",
		"/ip4/54.89.217.140/tcp/4001/p2p/12D3KooWConXG4mVuWtxxubA3wfmUWdCwkL54nZ7SnUgbEDnHZkX",
		"/ip4/54.87.223.182/tcp/4001/p2p/12D3KooWHRtiyBEGddhe7wvNz5q8A6gyDReqpzN5aiyT2gktKqXd",
		"/ip4/128.199.70.49/tcp/4001/p2p/12D3KooWQySWhisgDXXJEJTHZiewHasbsmfAMYbERdtnAS39397v",
		"/ip4/35.171.4.239/tcp/4001/p2p/12D3KooWHJCJJrAjSnJB9Mx9JWMeBAjgdSXrV7FwkCZk61if2bR3",
		"/ip4/45.32.130.169/tcp/4001/p2p/12D3KooWA1x69gRbUDaJqpZvQARRCR6H848ZydM6BBnszrTQV4w1",
		"/ip4/54.147.190.40/tcp/4001/p2p/12D3KooWHR1v13MD6ybgj5T3Ds56MB8LGcaXRH8W9cNLJP19AnRy",
		"/ip4/54.80.114.62/tcp/4001/p2p/12D3KooWSMc3sjPAAxdNXPg5nUa9M76WK2Vp3uf9FhfARpnmKjEH",
		"/ip4/54.174.102.221/tcp/4001/p2p/12D3KooWQo32RF8QSanP2LUnPnuKshqZdCFuUtypexzpAiUCK3js",
		"/ip4/54.172.254.208/tcp/4001/p2p/12D3KooWMsupg6xmmfmRht93nmLyRizrECj4gNh4FdUvpxE5eqaW",
	}

	if len(bootstrap) > 0 {
		bootstrapNodes = append(bootstrapNodes, bootstrap...)
	}

	if err := connectToPeers(ctx, n.api, bootstrapNodes); err != nil {
		return errors.Wrap(err, "failed connect to peers")
	}

	n.wg.Add(1)
	go n.print(ctx)

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

// FindPeersForContent -
func (n *Node) FindPeersForContent(ctx context.Context, cidString string) error {
	c, err := cid.Decode(cidString)
	if err != nil {
		return errors.Wrapf(err, "cid decoding: %s", cidString)
	}
	providers, err := n.dht.FindProviders(ctx, c)
	if err != nil {
		return errors.Wrapf(err, "finding peers for cid: %s", cidString)
	}
	if len(providers) == 0 {
		return nil
	}

	peers, err := n.api.Swarm().Peers(ctx)
	if err != nil {
		return errors.Wrap(err, "receiving current peers")
	}

	for i := range providers {
		var connected bool
		for j := range peers {
			if peers[j].ID().String() == providers[i].ID.String() {
				connected = true
				break
			}
		}
		if connected {
			continue
		}

		connectCtx, cancel := context.WithTimeout(ctx, time.Second*15)
		defer cancel()

		if err := n.api.Swarm().Connect(connectCtx, providers[i]); err != nil {
			l := log.Warn().
				Str("peer", providers[i].ID.String())
			if len(providers[i].Addrs) > 0 {
				l = l.Str("address", providers[i].Addrs[0].String())
			}
			l.Msgf("failed to connect: %s", err)
		} else {
			log.Info().Str("peer", providers[i].ID.String()).Msg("connected")
		}
	}
	return nil
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
		Online:  true,
		Routing: libp2p.DHTClientOption,
		Repo:    r,
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

			if err := ipfs.Swarm().Connect(connectCtx, *peerInfo); err != nil {
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
	// cfg.Swarm.Transports.Network.QUIC = config.False
	cfg.Swarm.AddrFilters = blacklist
	cfg.Swarm.ConnMgr.HighWater = config.NewOptionalInteger(10000)
	cfg.Swarm.ConnMgr.LowWater = config.NewOptionalInteger(450)

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

func (n *Node) print(ctx context.Context) {
	defer n.wg.Done()

	ticker := time.NewTicker(time.Minute * 10)
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

			for i := range peers {
				log.Info().Str("peer_id", peers[i].ID().String()).Str("address", peers[i].Address().String()).Msg("connected to peer")
			}
		}
	}
}
