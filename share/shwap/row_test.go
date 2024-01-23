package shwap

import (
	"testing"
	"os"
	"encoding/hex"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestRow(t *testing.T) {
	square := edstest.RandEDS(t, 8)
	root, err := share.NewRoot(square)
	require.NoError(t, err)

	data, _ := os.ReadFile("/tmp/row.data")
	row := &Row{}
	row.UnmarshalBinary(data)

	row, err := NewRowFromEDS(1, 2, square)
	require.NoError(t, err)

	data, err := row.MarshalBinary()
	require.NoError(t, err)

	os.WriteFile("row.data", data, 0644)
	t.Log(hex.EncodeToString(data))

	blk, err := row.IPLDBlock()
	require.NoError(t, err)
	assert.EqualValues(t, blk.Cid(), row.Cid())

	rowOut := &Row{}
	err = rowOut.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.EqualValues(t, row, rowOut)

	err = rowOut.Verify(root)
	require.NoError(t, err)
}
