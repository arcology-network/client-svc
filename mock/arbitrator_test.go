package mock

import (
	"math/big"
	"testing"

	"github.com/arcology-network/common-lib/types"
	ctypes "github.com/arcology-network/common-lib/types"
	"github.com/arcology-network/concurrentlib/clib"
	"github.com/arcology-network/mevm/geth/common"
	"github.com/arcology-network/mevm/geth/core"
	mevmTypes "github.com/arcology-network/mevm/geth/core/types"
	"github.com/arcology-network/mevm/geth/crypto"
	adaptor "github.com/arcology-network/vm-adaptor/evm"
)

const ArbitratorTest = "608060405234801561001057600080fd5b506000608190508073ffffffffffffffffffffffffffffffffffffffff1663f02e3aff6001600381111561004057fe5b6002600381111561004d57fe5b6040518363ffffffff167c010000000000000000000000000000000000000000000000000000000002815260040180806020018460030b60030b81526020018360030b60030b8152602001828103825260118152602001807f636f6e63757272656e74686173686d61700000000000000000000000000000008152506020019350505050600060405180830381600087803b1580156100eb57600080fd5b505af11580156100ff573d6000803e3d6000fd5b50505050506103e1806101136000396000f3fe60806040526004361061005c576000357c010000000000000000000000000000000000000000000000000000000090048063342d6b8b146100615780637a52d2ca146100bc578063f49b62c4146100e7578063fce554e614610122575b600080fd5b34801561006d57600080fd5b506100ba6004803603604081101561008457600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610187565b005b3480156100c857600080fd5b506100d1610286565b6040518082815260200191505060405180910390f35b3480156100f357600080fd5b506101206004803603602081101561010a57600080fd5b810190808035906020019092919050505061028f565b005b34801561012e57600080fd5b506101716004803603602081101561014557600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610299565b6040518082815260200191505060405180910390f35b6000608190508073ffffffffffffffffffffffffffffffffffffffff16634f7c4f4c84846040518363ffffffff167c010000000000000000000000000000000000000000000000000000000002815260040180806020018473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001838152602001828103825260118152602001807f636f6e63757272656e74686173686d61700000000000000000000000000000008152506020019350505050600060405180830381600087803b15801561026957600080fd5b505af115801561027d573d6000803e3d6000fd5b50505050505050565b60008054905090565b8060008190555050565b600080608190508073ffffffffffffffffffffffffffffffffffffffff1663c41eb85a846040518263ffffffff167c010000000000000000000000000000000000000000000000000000000002815260040180806020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001828103825260118152602001807f636f6e63757272656e74686173686d61700000000000000000000000000000008152506020019250505060206040518083038186803b15801561037257600080fd5b505afa158015610386573d6000803e3d6000fd5b505050506040513d602081101561039c57600080fd5b810190808051906020019092919050505091505091905056fea165627a7a72305820cb88f272478dd300e639171641f4c780a27cdcdab913bd9edf3aca6d59ff8c1d0029"

var user1 = common.BytesToAddress([]byte{1, 1, 1, 1})
var user2 = common.BytesToAddress([]byte{2, 2, 2, 2})

func TestArbitratorEthNoConflict(t *testing.T) {
	context := setup()
	eu := core.NewEU(0x0101, context.state, context.api, createTestConfigOnBlock(context.blockNo))
	ok, contract := context.deploy(eu, ArbitratorTest, user1, 0, common.BytesToHash([]byte{1}))
	if !ok {
		t.Error("Failed to deploy ArbitratorTest.")
		return
	}
	config := createTestConfigOnBlock(2)
	config.Coinbase = &coinbase
	data := crypto.Keccak256([]byte("ethStorageRead()"))[:4]
	m1 := mevmTypes.NewMessage(user1, contract, 1, new(big.Int).SetUint64(0), 1e8, new(big.Int).SetUint64(1), data, true)
	m2 := mevmTypes.NewMessage(user2, contract, 0, new(big.Int).SetUint64(0), 1e8, new(big.Int).SetUint64(1), data, true)
	result1, r1 := runSequentialTx(eu, context.api, common.BytesToHash([]byte{2}), &m1, config)
	result2, r2 := runSequentialTx(eu, context.api, common.BytesToHash([]byte{3}), &m2, config)
	if r1.Status != 1 || r2.Status != 1 {
		t.Error("Failed to call ethStorageRead().")
		return
	}
	flags := new(Arbitrator).DetectConflict([]*types.EuResult{result1, result2})
	if flags[0] || flags[1] {
		t.Error("Call ethStorageRead() makes unexpected confliction")
		return
	}
}

func TestArbitratorEthConflict(t *testing.T) {
	context := setup()
	eu := core.NewEU(0x0101, context.state, context.api, createTestConfigOnBlock(context.blockNo))
	ok, contract := context.deploy(eu, ArbitratorTest, user1, 0, common.BytesToHash([]byte{1}))
	if !ok {
		t.Error("Failed to deploy ArbitratorTest")
		return
	}

	data := crypto.Keccak256([]byte("ethStorageWrite(int256)"))[:4]
	m1 := mevmTypes.NewMessage(user1, contract, 1, new(big.Int).SetUint64(0), 1e8, new(big.Int).SetUint64(1), append(data, common.BytesToHash([]byte{1}).Bytes()...), true)
	m2 := mevmTypes.NewMessage(user2, contract, 0, new(big.Int).SetUint64(0), 1e8, new(big.Int).SetUint64(1), append(data, common.BytesToHash([]byte{2}).Bytes()...), true)
	config := createTestConfigOnBlock(2)
	config.Coinbase = &coinbase
	result1, r1 := runSequentialTx(eu, context.api, common.BytesToHash([]byte{2}), &m1, config)
	result2, r2 := runSequentialTx(eu, context.api, common.BytesToHash([]byte{3}), &m2, config)
	if r1.Status != 1 || r2.Status != 1 {
		t.Error("Failed to call ethStorageWrite().")
		return
	}
	flags := new(Arbitrator).DetectConflict([]*types.EuResult{result1, result2})
	if flags[0] || !flags[1] {
		t.Error("Failed to find out the confliction.")
		return
	}
}

func TestArbitratorClibNoConflict(t *testing.T) {
	context := setup()
	eu := core.NewEU(0x0101, context.state, context.api, createTestConfigOnBlock(context.blockNo))
	ok, contract := context.deploy(eu, ArbitratorTest, user1, 0, common.BytesToHash([]byte{1}))
	if !ok {
		t.Error("Failed to deploy ArbitratorTest.")
		return
	}

	data := crypto.Keccak256([]byte("clibRead(address)"))[:4]
	m1 := mevmTypes.NewMessage(user1, contract, 1, new(big.Int).SetUint64(0), 1e8, new(big.Int).SetUint64(1), append(data, common.BytesToHash([]byte{1}).Bytes()...), true)
	m2 := mevmTypes.NewMessage(user2, contract, 0, new(big.Int).SetUint64(0), 1e8, new(big.Int).SetUint64(1), append(data, common.BytesToHash([]byte{1}).Bytes()...), true)
	config := createTestConfigOnBlock(2)
	config.Coinbase = &coinbase
	result1, r1 := runSequentialTx(eu, context.api, common.BytesToHash([]byte{2}), &m1, config)
	result2, r2 := runSequentialTx(eu, context.api, common.BytesToHash([]byte{3}), &m2, config)
	if r1.Status != 1 || r2.Status != 1 {
		t.Error("Failed to call clibRead().")
		return
	}
	flags := new(Arbitrator).DetectConflict([]*types.EuResult{result1, result2})
	if flags[0] || flags[1] {
		t.Error("Call clibRead() makes unexpected confliction")
		return
	}
}

func TestArbitratorClibConflict(t *testing.T) {
	context := setup()
	eu := core.NewEU(0x0101, context.state, context.api, createTestConfigOnBlock(context.blockNo))
	ok, contract := context.deploy(eu, ArbitratorTest, user1, 0, common.BytesToHash([]byte{1}))
	if !ok {
		t.Error("Failed to deploy ArbitratorTest.")
		return
	}

	data := crypto.Keccak256([]byte("clibWrite(address,uint256)"))[:4]
	m1 := mevmTypes.NewMessage(user1, contract, 1, new(big.Int).SetUint64(0), 1e8, new(big.Int).SetUint64(1), append(append(data, common.BytesToHash([]byte{1}).Bytes()...), common.BytesToHash([]byte{2}).Bytes()...), true)
	m2 := mevmTypes.NewMessage(user2, contract, 0, new(big.Int).SetUint64(0), 1e8, new(big.Int).SetUint64(1), append(append(data, common.BytesToHash([]byte{1}).Bytes()...), common.BytesToHash([]byte{3}).Bytes()...), true)
	config := createTestConfigOnBlock(2)
	config.Coinbase = &coinbase
	result1, r1 := runSequentialTx(eu, context.api, common.BytesToHash([]byte{2}), &m1, config)
	result2, r2 := runSequentialTx(eu, context.api, common.BytesToHash([]byte{3}), &m2, config)
	if r1.Status != 1 || r2.Status != 1 {
		t.Error("Failed to call clibWrite().")
		return
	}
	flags := new(Arbitrator).DetectConflict([]*ctypes.EuResult{result1, result2})
	if flags[0] || !flags[1] {
		t.Error("Failed to find out the confliction.")
		return
	}
}

type context struct {
	snapshot clib.Snapshot
	cache    *mockEthCache
	api      *adaptor.API
	state    core.StateDB
	blockNo  uint64
}

func setup() *context {
	snapshot := clib.NewSnapshot()
	ethCache := &mockEthCache{
		accounts: map[string]*mockEthAccount{
			string(coinbase.Bytes()): {
				balance: new(big.Int),
			},
			string(user1.Bytes()): {
				balance: new(big.Int).SetUint64(1000000000),
			},
			string(user2.Bytes()): {
				balance: new(big.Int).SetUint64(1000000000),
			},
		},
		codes:    make(map[string][]byte),
		storages: make(map[string]map[string]string),
	}
	kapi := adaptor.NewAPI(0x0101, snapshot)
	kapi.SetSnapShot(snapshot)
	state := core.NewStateDB(ethCache, ethCache, kapi)
	return &context{
		snapshot, ethCache, kapi, state, 10000000,
	}
}

func (c *context) deploy(eu *core.EU, code string, owner common.Address, nonce uint64, hash common.Hash) (bool, *common.Address) {
	deployMsg := mevmTypes.NewMessage(owner, nil, nonce, new(big.Int).SetUint64(0), 1e8, new(big.Int).SetUint64(1), common.Hex2Bytes(ArbitratorTest), true)
	config := createTestConfigOnBlock(2)
	config.Coinbase = &coinbase
	euResult, receipt := runSequentialTx(eu, c.api, hash, &deployMsg, config)
	if receipt.Status != 1 {
		return false, nil
	}

	c.cache.Commit(euResult.W)
	c.snapshot.Commit(euResult.W.ClibWrites)
	return true, &receipt.ContractAddress
}
