package ipfs

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sen1or/lets-live/internal"
	"sync"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
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
type IPFSStorage struct {
	ipfsApi iface.CoreAPI
	node    *core.IpfsNode
	ctx     context.Context
	gateway string

	hlsDirectory     string
	hlsDirectoryHash string
}

func NewIPFSStorage(hlsDirectory string, gateway string) internal.Storage {
	ctx := context.Background()

	ipfsStorage := &IPFSStorage{
		ctx:          ctx,
		hlsDirectory: hlsDirectory,
		gateway:      gateway,
	}

	ipfsStorage.setup()

	return ipfsStorage
}

func (s *IPFSStorage) setup() {
	ipfsApi, node, err := s.spawnEphemeral()
	if err != nil {
		log.Panic(err)
	}

	s.ipfsApi = *ipfsApi
	s.node = node

	hashResultChan := make(chan string, 1)
	go (func() {
		hlsDirectoryHashString, err := s.AddDirectory(s.hlsDirectory)
		hashResultChan <- hlsDirectoryHashString

		if err != nil {
			log.Panicf("failed to add hls directory: %s", err)
		}
	})()

	s.hlsDirectoryHash = <-hashResultChan
	go s.goOnlineIPFSNode()
}

func (s *IPFSStorage) GenerateRemotePlaylist(playlistPath string, variant internal.HLSVariant) (string, error) {
	file, err := os.Open(playlistPath)
	if err != nil {
		return "", fmt.Errorf("can't open playlist %s: %s", playlistPath, err)
	}
	defer file.Close()

	var newPlaylist string = ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line[0] != '#' {
			segment := variant.GetSegmentByFilename(line)
			if segment == nil || segment.FullLocalPath == "" {
				line = ""
			} else {
				line = segment.IPFSRemoteId
			}
		}

		newPlaylist = newPlaylist + line + "\n"
	}

	return newPlaylist, nil
}

// create and return the directory hash string
func (s *IPFSStorage) AddDirectory(directoryPath string) (string, error) {
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

func (s *IPFSStorage) Save(filePath string) (path.Path, error) {
	file, err := getUnixfsNode(filePath)
	defer file.Close()

	if err != nil {
		return nil, fmt.Errorf("failed to get file: %s", err)
	}

	opts := []options.UnixfsAddOption{
		options.Unixfs.Pin(false),
	}

	fileCid, err := s.ipfsApi.Unixfs().Add(s.ctx, file, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to add file into ipfs: %s", err)
	}

	return fileCid, err
}

func (s *IPFSStorage) SaveIntoHLSDirectory(filePath string) (string, error) {
	fileCid, err := s.Save(filePath)
	if err != nil {
		return "", err
	}

	finalHash, err := s.addHashedFileToDirectory(fileCid, s.hlsDirectoryHash, filepath.Base(filePath))
	// TODO: use gateway instead
	return "http://localhost:5002" + finalHash, err
}

// Add the hashed file into "hls" directory hash which is already get added into IPFS storage
func (s *IPFSStorage) addHashedFileToDirectory(fileHash path.Path, directoryToAddTo string, filename string) (string, error) {
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

func getUnixfsNode(path string) (files.Node, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	f, err := files.NewSerialFile(path, false, st)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func createIPFSNode(ctx context.Context, repoPath string) (*iface.CoreAPI, *core.IpfsNode, error) {
	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, nil, err
	}

	nodeOptions := &core.BuildCfg{
		Online:  true,
		Routing: libp2p.DHTOption,
		Repo:    repo,
	}

	node, err := core.NewNode(ctx, nodeOptions)
	node.IsDaemon = true

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

	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to create ephemeral node: %s", err)
	}
	return repoPath, nil
}

var loadPluginsOnce sync.Once

// Function "spawnEphemeral" Create a temporary just for one run
func (s *IPFSStorage) spawnEphemeral() (*iface.CoreAPI, *core.IpfsNode, error) {
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

func (s *IPFSStorage) connectToPeers(peers []string) error {
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
			}
		}(peerInfo)
	}
	wg.Wait()
	return nil
}

func (s *IPFSStorage) goOnlineIPFSNode() {
	defer log.Println("IPFS node exited")
	log.Println("IPFS node is running")

	bootstrapNodes := []string{
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
	}

	go s.connectToPeers(bootstrapNodes)

	addr := "/ip4/127.0.0.1/tcp/5002"
	var opts = []corehttp.ServeOption{
		corehttp.GatewayOption("/ipfs", "/ipns"),
	}

	if err := corehttp.ListenAndServe(s.node, addr, opts...); err != nil {
		return
	}
}
