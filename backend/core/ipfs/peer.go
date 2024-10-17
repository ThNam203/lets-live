package ipfs

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	chunker "github.com/ipfs/boxo/chunker"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/ipld/unixfs/importer/balanced"
	"github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	ufsio "github.com/ipfs/boxo/ipld/unixfs/io"

	"github.com/ipfs/boxo/provider"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multihash"
)

var (
	defaultReprovideInterval = 12 * time.Hour
)

type Peer struct {
	ctx context.Context

	host  host.Host
	dht   routing.Routing
	store datastore.Batching

	ipld.DAGService // become a DAG service
	exch            exchange.Interface
	bstore          blockstore.Blockstore
	bserv           blockservice.BlockService
	reprovider      provider.System
}

func (p *Peer) GetHost() host.Host {
	return p.host
}

func NewIPFSNode(
	ctx context.Context,
	datastore datastore.Batching,
	host host.Host,
	dht routing.Routing,
) (*Peer, error) {
	p := &Peer{
		ctx:   ctx,
		host:  host,
		dht:   dht,
		store: datastore,
	}

	// get the default blockstore implementation
	p.bstore = blockstore.NewBlockstore(p.store)

	err := p.setupBlockService()
	if err != nil {
		return nil, err
	}
	err = p.setupDAGService()
	if err != nil {
		p.bserv.Close()
		return nil, err
	}
	err = p.setupReprovider()
	if err != nil {
		p.bserv.Close()
		return nil, err
	}

	go p.onClose()

	return p, nil
}

func (p *Peer) setupBlockService() error {
	bswapnet := network.NewFromIpfsHost(p.host, p.dht)
	bswap := bitswap.New(p.ctx, bswapnet, p.bstore)
	p.bserv = blockservice.New(p.bstore, bswap)
	p.exch = bswap
	return nil
}

func (p *Peer) setupDAGService() error {
	p.DAGService = merkledag.NewDAGService(p.bserv)
	return nil
}

// no need reprovide fucntionality
func (p *Peer) setupReprovider() error {
	p.reprovider = provider.NewNoopProvider()
	return nil
}

func (p *Peer) onClose() {
	<-p.ctx.Done()
	p.reprovider.Close()
	p.bserv.Close()
}

func (p *Peer) Session(ctx context.Context) ipld.NodeGetter {
	ng := merkledag.NewSession(ctx, p.DAGService)
	if ng == p.DAGService {
		log.Println("DAGService does not support sessions")
	}
	return ng
}

func (p *Peer) AddFile(ctx context.Context, r io.Reader) (ipld.Node, error) {
	prefix, _ := merkledag.PrefixForCidVersion(0)

	hashFunCode, _ := multihash.Names["sha2-256"]
	prefix.MhType = hashFunCode
	prefix.MhLength = -1

	dbp := helpers.DagBuilderParams{
		Dagserv:    p,
		RawLeaves:  false,
		Maxlinks:   helpers.DefaultLinksPerBlock,
		NoCopy:     false,
		CidBuilder: &prefix,
	}

	chnk, err := chunker.FromString(r, "default")
	if err != nil {
		return nil, err
	}
	dbh, err := dbp.New(chnk)
	if err != nil {
		return nil, err
	}

	var n ipld.Node
	n, err = balanced.Layout(dbh)
	return n, err
}

// GetFile returns a reader to a file as identified by its root CID. The file
// must have been added as a UnixFS DAG (default for IPFS).
func (p *Peer) GetFile(ctx context.Context, c cid.Cid) (ufsio.ReadSeekCloser, error) {
	n, err := p.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return ufsio.NewDagReader(ctx, n, p)
}

// BlockStore offers access to the blockstore underlying the Peer's DAGService.
func (p *Peer) BlockStore() blockstore.Blockstore {
	return p.bstore
}

// HasBlock returns whether a given block is available locally. It is
// a shorthand for .Blockstore().Has().
func (p *Peer) HasBlock(ctx context.Context, c cid.Cid) (bool, error) {
	return p.BlockStore().Has(ctx, c)
}

// Exchange returns the underlying exchange implementation.
func (p *Peer) Exchange() exchange.Interface {
	return p.exch
}

// BlockService returns the underlying blockservice implementation.
func (p *Peer) BlockService() blockservice.BlockService {
	return p.bserv
}
