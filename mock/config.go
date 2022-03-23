package mock

import (
	"math"
	"math/big"

	"github.com/arcology-network/mevm/geth/common"
	"github.com/arcology-network/mevm/geth/core"
	"github.com/arcology-network/mevm/geth/core/types"
	"github.com/arcology-network/mevm/geth/core/vm"
	"github.com/arcology-network/mevm/geth/params"
)

type fakeChain struct {
}

func (chain *fakeChain) GetHeader(common.Hash, uint64) *types.Header {
	return &types.Header{}
}

var coinbase = common.BytesToAddress([]byte{100, 100, 100})

func createTestConfig() *core.Config {
	vmConfig := vm.Config{}
	cfg := &core.Config{
		ChainConfig: params.MainnetChainConfig,
		VMConfig:    &vmConfig,
		BlockNumber: new(big.Int).SetUint64(10000000),
		ParentHash:  common.Hash{},
		Time:        new(big.Int).SetUint64(10000000),
		Coinbase:    &coinbase,
		GasLimit:    math.MaxUint64,
		Difficulty:  new(big.Int).SetUint64(10000000),
	}
	cfg.Chain = new(fakeChain)
	return cfg
}

func createTestConfigOnBlock(bn uint64) *core.Config {
	cfg := createTestConfig()
	cfg.BlockNumber = new(big.Int).SetUint64(bn)
	// 60 seconds per block.
	cfg.Time = new(big.Int).SetUint64(bn * 60)
	return cfg
}
