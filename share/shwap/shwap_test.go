package shwap

import (
	"context"
	"testing"
	"time"
	"encoding/hex"

	"github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/boxo/routing/offline"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	record "github.com/libp2p/go-libp2p-record"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"

	bsmsg "github.com/ipfs/boxo/bitswap/message"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestSampleRoundtripGetBlock tests full protocol round trip of:
// EDS -> Sample -> IPLDBlock -> BlockService -> Bitswap and in reverse.
func TestSampleRoundtripGetBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	square := edstest.RandEDS(t, 8)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	b := edsBlockstore(square)
	client := remoteClient(ctx, t, b)

	width := int(square.Width())
	for i := 0; i < width*width; i++ {
		smpl, err := NewSampleFromEDS(RowProofType, i, square, 1) // TODO: Col
		require.NoError(t, err)

		sampleVerifiers.Add(smpl.SampleID, func(sample Sample) error {
			return sample.Verify(root)
		})

		cid := smpl.Cid()
		t.Log("requesting ", cid)
		blkOut, err := client.GetBlock(ctx, cid)
		require.NoError(t, err)
		assert.EqualValues(t, cid, blkOut.Cid())

		smpl, err = SampleFromBlock(blkOut)
		assert.NoError(t, err)

		//err = smpl.Verify(root)
		//assert.NoError(t, err)
		break
	}
}

// TODO: Debug why is it flaky
func AAATestSampleRoundtripGetBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	square := edstest.RandEDS(t, 8)
	root, err := share.NewRoot(square)
	require.NoError(t, err)
	b := edsBlockstore(square)
	client := remoteClient(ctx, t, b)

	set := cid.NewSet()
	width := int(square.Width())
	for i := 0; i < width*width; i++ {
		smpl, err := NewSampleFromEDS(RowProofType, i, square, 1) // TODO: Col
		require.NoError(t, err)
		set.Add(smpl.Cid())

		sampleVerifiers.Add(smpl.SampleID, func(sample Sample) error {
			return sample.Verify(root)
		})
	}

	blks, err := client.GetBlocks(ctx, set.Keys())
	require.NoError(t, err)

	err = set.ForEach(func(c cid.Cid) error {
		select {
		case blk := <-blks:
			assert.True(t, set.Has(blk.Cid()))

			smpl, err := SampleFromBlock(blk)
			assert.NoError(t, err)

			err = smpl.Verify(root) // bitswap already performed validation and this is only for testing
			assert.NoError(t, err)
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	assert.NoError(t, err)
}

func TestRowRoundtripGetBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	square := edstest.RandEDS(t, 16)
	root, err := share.NewRoot(square)
	require.NoError(t, err)
	b := edsBlockstore(square)
	client := remoteClient(ctx, t, b)

	width := int(square.Width())
	for i := 0; i < width; i++ {
		row, err := NewRowFromEDS(1, i, square)
		require.NoError(t, err)

		rowVerifiers.Add(row.RowID, func(row Row) error {
			return row.Verify(root)
		})

		cid := row.Cid()
		blkOut, err := client.GetBlock(ctx, cid)
		require.NoError(t, err)
		assert.EqualValues(t, cid, blkOut.Cid())

		row, err = RowFromBlock(blkOut)
		assert.NoError(t, err)

		err = row.Verify(root)
		assert.NoError(t, err)
	}
}

func TestRowRoundtripGetBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	square := edstest.RandEDS(t, 16)
	root, err := share.NewRoot(square)
	require.NoError(t, err)
	b := edsBlockstore(square)
	client := remoteClient(ctx, t, b)

	set := cid.NewSet()
	width := int(square.Width())
	for i := 0; i < width; i++ {
		row, err := NewRowFromEDS(1, i, square)
		require.NoError(t, err)
		set.Add(row.Cid())

		rowVerifiers.Add(row.RowID, func(row Row) error {
			return row.Verify(root)
		})
	}

	blks, err := client.GetBlocks(ctx, set.Keys())
	require.NoError(t, err)

	err = set.ForEach(func(c cid.Cid) error {
		select {
		case blk := <-blks:
			assert.True(t, set.Has(blk.Cid()))

			row, err := RowFromBlock(blk)
			assert.NoError(t, err)

			err = row.Verify(root)
			assert.NoError(t, err)
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	assert.NoError(t, err)
}

func TestDataRoundtripGetBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	namespace := sharetest.RandV0Namespace()
	sqr, root := edstest.RandEDSWithNamespace(t, namespace, 16)
	b := edsBlockstore(sqr)
	client := remoteClient(ctx, t, b)

	nds, err := NewDataFromEDS(sqr, 1, namespace)
	require.NoError(t, err)

	for _, nd := range nds {
		dataVerifiers.Add(nd.DataID, func(data Data) error {
			return data.Verify(root)
		})

		cid := nd.Cid()
		blkOut, err := client.GetBlock(ctx, cid)
		require.NoError(t, err)
		assert.EqualValues(t, cid, blkOut.Cid())

		ndOut, err := DataFromBlock(blkOut)
		assert.NoError(t, err)

		err = ndOut.Verify(root)
		assert.NoError(t, err)
	}
}

func TestDataRoundtripGetBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	namespace := sharetest.RandV0Namespace()
	sqr, root := edstest.RandEDSWithNamespace(t, namespace, 16)
	b := edsBlockstore(sqr)
	client := remoteClient(ctx, t, b)

	nds, err := NewDataFromEDS(sqr, 1, namespace)
	require.NoError(t, err)

	set := cid.NewSet()
	for _, nd := range nds {
		set.Add(nd.Cid())

		dataVerifiers.Add(nd.DataID, func(data Data) error {
			return data.Verify(root)
		})
	}

	blks, err := client.GetBlocks(ctx, set.Keys())
	require.NoError(t, err)

	err = set.ForEach(func(c cid.Cid) error {
		select {
		case blk := <-blks:
			assert.True(t, set.Has(blk.Cid()))

			smpl, err := DataFromBlock(blk)
			assert.NoError(t, err)

			err = smpl.Verify(root)
			assert.NoError(t, err)
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	assert.NoError(t, err)
}

func remoteClient(ctx context.Context, t *testing.T, bstore blockstore.Blockstore) exchange.Fetcher {
	net, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)

	//net.Connect(ctx, "/ip4/172.19.0.3/tcp/2121")
	//t.Log(hex.EncodeToString(data))
	t.Log(net.Hosts())

	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	routing := offline.NewOfflineRouter(dstore, record.NamespacedValidator{})

	trs := &Tr {
		t: t,
		typp: "server",
	}

	_ = bitswap.New(
		ctx,
		network.NewFromIpfsHost(net.Hosts()[0], routing),
		//network.NewFromIpfsHost("/ip4/172.19.0.3/tcp/2121", routing),
		bstore,
		bitswap.WithTracer(trs),
	)

	dstoreClient := dssync.MutexWrap(ds.NewMapDatastore())
	bstoreClient := blockstore.NewBlockstore(dstoreClient)
	routingClient := offline.NewOfflineRouter(dstoreClient, record.NamespacedValidator{})

	/*
	trc := &Tr {
		t: t,
		typp: "client",
	}
	*/

	bitswapClient := bitswap.New(
		ctx,
		network.NewFromIpfsHost(net.Hosts()[1], routingClient),
		bstoreClient,
		//bitswap.WithTracer(trc),
	)


	err = net.ConnectAllButSelf()
	require.NoError(t, err)

	return bitswapClient
}

type Tr struct {
	t *testing.T;
	typp string;
}

func (t *Tr) MessageReceived(peerID peer.ID, msg bsmsg.BitSwapMessage) {
	t.t.Log(t.typp, "Rcvd: ", peerID)
	t.t.Log(msg)
	bytes, err := msg.ToProtoV1().Marshal()
	if err != nil {
		panic("xd")
	}
	t.t.Log(hex.EncodeToString(bytes))
	
}
func (t *Tr) MessageSent(peerID peer.ID, msg bsmsg.BitSwapMessage) {
	t.t.Log(t.typp, "Send: ", peerID)
	bytes, err := msg.ToProtoV1().Marshal()
	if err != nil {
		panic("xd")
	}
	t.t.Log(hex.EncodeToString(bytes))
}
