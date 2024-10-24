package ipfs

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sen1or/lets-live/core/logger"
	"sen1or/lets-live/core/storage"

	"sync"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/config"
	kuboCore "github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/core/corehttp"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/plugin/loader"
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// TODO: add way to check if hls directory exists

// KuboStorage use the kubo (an IPFS implementation) as a library
// to create our own ipfs node
// - It works outside the box and fully-fledged
// - Only a single node doing the upload to IPFS network
type KuboStorage struct {
	ipfsApi iface.CoreAPI
	node    *kuboCore.IpfsNode
	ctx     context.Context
	gateway string

	hlsDirectory     string
	hlsDirectoryHash string
}

func NewKuboStorage(hlsDirectory string, gateway string) storage.Storage {
	ctx := context.Background()

	ipfsStorage := &KuboStorage{
		ctx:          ctx,
		hlsDirectory: hlsDirectory,
		gateway:      gateway,
	}

	ipfsStorage.setup()

	return ipfsStorage
}

func (s *KuboStorage) setup() {
	ipfsApi, node, err := s.spawnEphemeral()

	if err != nil {
		log.Panic(err)
	}

	s.ipfsApi = *ipfsApi
	s.node = node

	hlsDirectoryHashString, err := s.AddDirectory(s.hlsDirectory)
	s.hlsDirectoryHash = hlsDirectoryHashString

	logger.Infof("added hls directory to ipfs with hash %s", hlsDirectoryHashString)
	if err != nil {
		logger.Infof("failed to add hls directory: %s", err)
	}

	// TODO: connect to other peers and go online instead of local gateway
	go s.goOnlineIPFSNode()
}

// create and return the directory hash string
func (s *KuboStorage) AddDirectory(directoryPath string) (string, error) {
	directoryNode, err := getUnixfsNode(directoryPath)
	defer (func() {
		if directoryNode != nil {
			directoryNode.Close()
		}
	})()

	if err != nil {
		return "", fmt.Errorf("failed to create directory: %s", err)
	}

	directoryHash, err := s.ipfsApi.Unixfs().Add(s.ctx, directoryNode)
	if err != nil {
		return "", fmt.Errorf("failed to add directory: %s", err)
	}

	return directoryHash.String(), nil
}

func (s *KuboStorage) AddFile(filePath string) (string, error) {
	file, err := getUnixfsNode(filePath)
	defer file.Close()

	if err != nil {
		return "", fmt.Errorf("failed to get file: %s", err)
	}

	opts := []options.UnixfsAddOption{
		options.Unixfs.Pin(false),
	}

	p, err := s.ipfsApi.Unixfs().Add(s.ctx, file, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to add file into ipfs: %s", err)
	}

	return s.gateway + p.String(), err
}

// TODO: Should add this fucntion to the Storage interface... or not
func (s *KuboStorage) AddFileIntoHLSDirectory(filePath string) (string, error) {
	file, err := getUnixfsNode(filePath)
	defer file.Close()

	if err != nil {
		return "", fmt.Errorf("failed to get file: %s", err)
	}

	opts := []options.UnixfsAddOption{
		options.Unixfs.Pin(false),
	}

	p, err := s.ipfsApi.Unixfs().Add(s.ctx, file, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to add file into ipfs: %s", err)
	}

	finalHash, err := s.addHashedFileToDirectory(p, s.hlsDirectoryHash, filepath.Base(filePath))
	logger.Infof("segment saved with cid %s and final hash %s", p.RootCid().String(), finalHash)
	return s.gateway + finalHash, err
}

// Add the hashed file into "hls" directory hash which is already get added into IPFS storage
func (s *KuboStorage) addHashedFileToDirectory(fileHash path.Path, directoryToAddTo string, filename string) (string, error) {
	directoryPath, err := path.NewPath(directoryToAddTo)
	if err != nil {
		return "", err
	}

	newDirectoryHash, err := s.ipfsApi.Object().AddLink(s.ctx, directoryPath, filename, fileHash)
	if err != nil {
		return "", err
	}

	return filepath.Join(newDirectoryHash.String(), filename), nil
}

func createIPFSNode(ctx context.Context, repoPath string) (*iface.CoreAPI, *kuboCore.IpfsNode, error) {
	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, nil, err
	}

	repo.SetConfigKey("Addresses.Gateway", "/ip4/0.0.0.0/tcp/8080")

	nodeOptions := &kuboCore.BuildCfg{
		Online:  true,
		Routing: libp2p.DHTOption,
		Repo:    repo,
	}

	node, err := kuboCore.NewNode(ctx, nodeOptions)
	// node.IsDaemon = true

	if err != nil {
		return nil, nil, err
	}

	coreApi, err := coreapi.NewCoreAPI(node)
	return &coreApi, node, nil
}

// Must load plugins before setting up everything
func setupPlugins(repoPath string) error {
	plugins, err := loader.NewPluginLoader(repoPath)
	if err != nil {
		return fmt.Errorf("error loading plugins: %s", err)
	}

	if err := plugins.Initialize(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	if err := plugins.Inject(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	return nil
}

func createTempRepo() (string, error) {
	repoPath, err := os.MkdirTemp("", "ipfs-shell")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir for ipfs: %s", err)
	}

	cfg, err := config.Init(log.Writer(), 2048)
	if err != nil {
		return "", fmt.Errorf("failed to init config file for repo: %s", err)
	}

	// https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#ipfs-filestore
	cfg.Experimental.FilestoreEnabled = true
	// https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#ipfs-urlstore
	cfg.Experimental.UrlstoreEnabled = true
	// https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#ipfs-p2p
	cfg.Experimental.Libp2pStreamMounting = true
	// https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#p2p-http-proxy
	cfg.Experimental.P2pHttpProxy = true
	// See also: https://github.com/ipfs/kubo/blob/master/docs/config.md
	// And: https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md

	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to create ephemeral node: %s", err)
	}
	return repoPath, nil
}

var loadPluginsOnce sync.Once

// Function "spawnEphemeral" Create a temporary just for one run
func (s *KuboStorage) spawnEphemeral() (*iface.CoreAPI, *kuboCore.IpfsNode, error) {
	log.Println("spawing ephemeral ipfs")
	var onceErr error
	loadPluginsOnce.Do(func() {
		onceErr = setupPlugins("")
	})

	if onceErr != nil {
		return nil, nil, onceErr
	}

	// Create a Temporary Repo
	repoPath, err := createTempRepo()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp repo: %s", err)
	}

	api, node, err := createIPFSNode(s.ctx, repoPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create ipfs node: %s", err)
	}

	return api, node, err
}

func (s *KuboStorage) connectToPeers(peers []string) error {
	var wg sync.WaitGroup
	peerInfos := make(map[peer.ID]*peer.AddrInfo, len(peers))
	for _, addrStr := range peers {
		addr, err := multiaddr.NewMultiaddr(addrStr)
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

	wg.Add(len(peerInfos))
	for _, peerInfo := range peerInfos {
		go func(peerInfo *peer.AddrInfo) {
			defer wg.Done()
			err := s.ipfsApi.Swarm().Connect(s.ctx, *peerInfo)
			if err != nil {
				log.Printf("failed to connect to %s: %s", peerInfo.ID, err)
			} else {
				log.Printf("ipfs connectted to %s", peerInfo.ID)
			}
		}(peerInfo)
	}
	wg.Wait()
	return nil
}

func (s *KuboStorage) goOnlineIPFSNode() {
	// bootstrapNodes := []string{
	// IPFS Bootstrapper nodes.
	// "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
	// "/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
	// "/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
	// "/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",

	// IPFS Cluster Pinning nodes
	// "/ip4/138.201.67.219/tcp/4001/p2p/QmUd6zHcbkbcs7SMxwLs48qZVX3vpcM8errYS7xEczwRMA",

	// "/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",      // mars.i.ipfs.io
	// "/ip4/104.131.131.82/udp/4001/quic/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ", // mars.i.ipfs.io

	// You can add more nodes here, for example, another IPFS node you might have running locally, mine was:
	// "/ip4/127.0.0.1/tcp/4010/p2p/QmZp2fhDLxjYue2RiUvLwT9MWdnbDxam32qYFnGmxZDh5L",
	// "/ip4/127.0.0.1/udp/4010/quic/p2p/QmZp2fhDLxjYue2RiUvLwT9MWdnbDxam32qYFnGmxZDh5L",
	// }

	// go s.connectToPeers(bootstrapNodes)

	addr := "/ip4/127.0.0.1/tcp/8080"
	var opts = []corehttp.ServeOption{
		corehttp.GatewayOption("/ipfs", "/ipns"),
	}

	if err := corehttp.ListenAndServe(s.node, addr, opts...); err != nil {
		log.Printf("ipfs bootstraping failed: %s\n", err)
		return
	}
}
