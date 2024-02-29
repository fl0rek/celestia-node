package p2p

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/boxo/bitswap/client"
	"github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/bitswap/server"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/go-datastore"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	hst "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/share/store"
	//"github.com/celestiaorg/celestia-node/share"
	//"github.com/celestiaorg/celestia-node/share/eds"
	//"github.com/celestiaorg/celestia-node/share/shwap"

	"encoding/json"
	//"encoding/hex"
	"github.com/libp2p/go-libp2p/core/peer"
	bsmsg "github.com/ipfs/boxo/bitswap/message"

	"github.com/ipfs/go-cid"
	//"os"
	//"github.com/celestiaorg/celestia-node/share/shwap/row"

	//"github.com/celestiaorg/celestia-node/share/eds/edstest"
	//"github.com/celestiaorg/celestia-node/share"
)

const (
	// default size of bloom filter in blockStore
	defaultBloomFilterSize = 512 << 10
	// default amount of hash functions defined for bloom filter
	defaultBloomFilterHashes = 7
	// default size of arc cache in blockStore
	defaultARCCacheSize = 64 << 10
)

func sum(c chan cid.Cid) {
	cid := <- c
	println(": ", cid.String())
}

/*
=======
func dataExchange(params bitSwapParams) exchange.Interface {
	prefix := protocol.ID(fmt.Sprintf("/celestia/%s", params.Net))
	println(">", prefix, "<")
	tr := &Tr { }

	row_data, _ := os.ReadFile("/row.data")
	row := &shwap.Row{}
	row.UnmarshalBinary(row_data)
	row.RowID.RowIndex = 0
	row.RowID.Height = 1
	row_block, _ := row.IPLDBlock()
	
	params.Bs.Put(params.Ctx, row_block)
	row_exists, _ := params.Bs.Has(params.Ctx, row_block.Cid())
	println("inserted", row_exists, row_block.Cid().String())


	sample_data, _ := os.ReadFile("/sample.data")
	sample := &shwap.Sample{}
	sample.UnmarshalBinary(sample_data)
	sample.SampleID.RowID.RowIndex = 1
	sample.SampleID.RowID.Height = 2
	sample.SampleID.ShareIndex = 3
	sample_block, _ := sample.IPLDBlock()
	
	params.Bs.Put(params.Ctx, sample_block)
	sample_exists, _ := params.Bs.Has(params.Ctx, sample_block.Cid())
	println("inserted", sample_exists, sample_block.Cid().String())


	nsdata_data, _ := os.ReadFile("/namespaced_data.data")
	nsdata := &shwap.Data{}
	nsdata.UnmarshalBinary(nsdata_data)
	nsdata.DataID.RowID.RowIndex = 5
	nsdata.DataID.RowID.Height = 6
	nsdata.DataID.DataNamespace = string(share.TxNamespace)
	nsdata_block, _ := nsdata.IPLDBlock()
	
	params.Bs.Put(params.Ctx, nsdata_block)
	nsdata_exists, _ := params.Bs.Has(params.Ctx, nsdata_block.Cid())
	println("inserted", nsdata_exists, nsdata_block.Cid().String())

	chain, _ := params.Bs.AllKeysChan(params.Ctx)
	println("id:", row.RowID.RowIndex, row.RowID.Height)
	println("len: ", len(chain))

	return bitswap.New(
		*/

// dataExchange provides a constructor for IPFS block's DataExchange over BitSwap.
func dataExchange(params bitSwapParams) exchange.SessionExchange {
	tr := &Tr { }

	prefix := protocolID(params.Net)
	net := network.NewFromIpfsHost(params.Host, &routinghelpers.Null{}, network.Prefix(prefix))
	srvr := server.New(
		params.Ctx,
		net,
		params.Bs,
		server.ProvideEnabled(false), // we don't provide blocks over DHT
		// NOTE: These below are required for our protocol to work reliably.
		// // See https://github.com/celestiaorg/celestia-node/issues/732
		server.SetSendDontHaves(false),
		server.WithTracer(tr),
	)

	clnt := client.New(
		params.Ctx,
		net,
		params.Bs,
		client.WithBlockReceivedNotifier(srvr),
		client.SetSimulateDontHavesOnTimeout(false),
		client.WithoutDuplicatedBlockStats(),
	)
	net.Start(srvr, clnt) // starting with hook does not work

	params.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) (err error) {
			err = errors.Join(err, clnt.Close())
			err = errors.Join(err, srvr.Close())
			net.Stop()
			return err
		},
	})

	return clnt
}

type Tr struct {
       //typp string;
}

func (t *Tr) MessageReceived(peerID peer.ID, msg bsmsg.BitSwapMessage) {
       println("Rcvd:", peerID.String())
       /*
       bytes, err := msg.ToProtoV1().Marshal()
       if err != nil {
               panic("xd")
       }
       */
       //println(hex.EncodeToString(bytes))
       s, _ := json.MarshalIndent(msg, "", "\t")
       println("data: ", s)
       //fmt.Printf("%+v\n", msg)

}
func (t *Tr) MessageSent(peerID peer.ID, msg bsmsg.BitSwapMessage) {
       println("Sent:", peerID.String())
       /*
       bytes, err := msg.ToProtoV1().Marshal()
       if err != nil {
               panic("xd")
       }
       */
       //println(hex.EncodeToString(bytes))
       fmt.Printf("%+v\n", msg)
}

func blockstoreFromDatastore(ctx context.Context, ds datastore.Batching) (blockstore.Blockstore, error) {
	return blockstore.CachedBlockstore(
		ctx,
		blockstore.NewBlockstore(ds),
		blockstore.CacheOpts{
			HasBloomFilterSize:   defaultBloomFilterSize,
			HasBloomFilterHashes: defaultBloomFilterHashes,
			HasTwoQueueCacheSize: defaultARCCacheSize,
		},
	)
}

func blockstoreFromEDSStore(ctx context.Context, s *store.Store, ds datastore.Batching) (blockstore.Blockstore, error) {
	return blockstore.CachedBlockstore(
		ctx,
		store.NewBlockstore(s, ds),
		blockstore.CacheOpts{
			HasTwoQueueCacheSize: defaultARCCacheSize,
		},
	)
}

type bitSwapParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Ctx       context.Context
	Net       Network
	Host      hst.Host
	Bs        blockstore.Blockstore
}

func protocolID(network Network) protocol.ID {
	return protocol.ID(fmt.Sprintf("/celestia/%s", network))
}
