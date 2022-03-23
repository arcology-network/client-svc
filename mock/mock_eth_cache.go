package mock

import (
	"fmt"
	"math/big"

	"github.com/arcology-network/common-lib/types"
	"github.com/arcology-network/mevm/geth/core"
	"github.com/arcology-network/mevm/geth/crypto"
)

type mockEthAccount struct {
	balance  *big.Int
	nonce    uint64
	codeHash []byte
}

func (mock *mockEthAccount) GetBalance() *big.Int {
	return mock.balance
}

func (mock *mockEthAccount) GetNonce() uint64 {
	return mock.nonce
}

func (mock *mockEthAccount) GetCodeHash() []byte {
	return mock.codeHash
}

type mockEthCache struct {
	accounts map[string]*mockEthAccount
	codes    map[string][]byte
	storages map[string]map[string]string
}

func (mock *mockEthCache) GetAccount(addr string) (core.Account, error) {
	if acc, ok := mock.accounts[addr]; ok {
		return acc, nil
	}
	return nil, nil
}

func (mock *mockEthCache) GetCode(addr string) ([]byte, error) {
	if code, ok := mock.codes[addr]; ok {
		return code, nil
	}
	return nil, nil
}

func (mock *mockEthCache) GetState(addr string, key []byte) []byte {
	if s, ok := mock.storages[addr]; ok {
		if v, ok := s[string(key)]; ok {
			return []byte(v)
		}
	}
	return nil
}

func (mock *mockEthCache) Commit(writes *types.Writes) {
	for _, addr := range writes.NewAccounts {
		if _, ok := mock.accounts[string(addr)]; !ok {
			mock.accounts[string(addr)] = &mockEthAccount{
				balance:  new(big.Int),
				nonce:    0,
				codeHash: nil,
			}
		} else {
			panic(fmt.Sprintf("New account(%v) already exists", addr))
		}
	}

	for addr, amount := range writes.BalanceWrites {
		mock.accounts[string(addr)].balance = new(big.Int).Add(mock.accounts[string(addr)].balance, amount)
	}

	for addr, nonce := range writes.NonceWrites {
		mock.accounts[string(addr)].nonce = nonce
	}

	for addr, code := range writes.CodeWrites {
		mock.codes[string(addr)] = code
		mock.accounts[string(addr)].codeHash = crypto.Keccak256Hash(code).Bytes()
	}

	for addr, storage := range writes.EthStorageWrites {
		if _, ok := mock.storages[string(addr)]; !ok {
			mock.storages[string(addr)] = make(map[string]string)
		}
		for k, v := range storage {
			mock.storages[string(addr)][k] = v
		}
	}
}
