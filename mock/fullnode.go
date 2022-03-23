package mock

import (
	"bufio"
	"math/big"
	"os"
	"strings"
	"time"

	pbs "github.com/arcology-network/client-svc/clientsvc"
	pbc "github.com/arcology-network/client-svc/dappcontainer"
	"github.com/arcology-network/common-lib/types"
	"github.com/arcology-network/concurrentlib/clib"
	"github.com/arcology-network/mevm/geth/common"
	"github.com/arcology-network/mevm/geth/core"
	mevmTypes "github.com/arcology-network/mevm/geth/core/types"
	"github.com/arcology-network/mevm/geth/rlp"
	adaptor "github.com/arcology-network/vm-adaptor/evm"
	log "github.com/sirupsen/logrus"
)

type FullNode struct {
	cache    *mockEthCache
	eu       *core.EU
	snapshot clib.Snapshot
	api      *adaptor.API
	state    core.StateDB
	blockNo  uint64
}

func NewFullNode() *FullNode {
	snapshot := clib.NewSnapshot()
	ethCache := &mockEthCache{
		accounts: map[string]*mockEthAccount{
			string(coinbase.Bytes()): {
				balance: new(big.Int),
			},
		},
		codes:    make(map[string][]byte),
		storages: make(map[string]map[string]string),
	}
	kapi := adaptor.NewAPI(0x0101, snapshot)
	kapi.SetSnapShot(snapshot)
	state := core.NewStateDB(ethCache, ethCache, kapi)
	// eu := core.NewEU(0x0101, state, kapi, createTestConfig())

	// Init accounts.
	file, err := os.Open("./addresses.txt")
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Failed to open addresses.txt")
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		log.WithFields(log.Fields{
			"line": line,
		}).Debug("File content")

		fields := strings.Split(line, ",")
		addrStr, balanceStr := fields[0], fields[1]
		balance, ok := new(big.Int).SetString(balanceStr, 10)
		if !ok {
			log.WithFields(log.Fields{
				"str": balanceStr,
			}).Warn("Failed to convert string to big.Int")
			continue
		}
		ethCache.accounts[string(common.HexToAddress(addrStr).Bytes())] = &mockEthAccount{
			balance: balance,
		}
	}
	if err := scanner.Err(); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Failed to read file")
	}

	return &FullNode{
		// eu:       eu,
		snapshot: snapshot,
		cache:    ethCache,
		api:      kapi,
		state:    state,
		blockNo:  10000000,
	}
}

func (node *FullNode) GetBlockNo() int {
	return int(node.blockNo)
}

func (node *FullNode) GetTimestamp() int {
	// 60 seconds per block.
	return int(node.blockNo) * 60
}

func (node *FullNode) Run(txs [][]byte) []*mevmTypes.Receipt {
	config := createTestConfigOnBlock(node.blockNo)
	node.eu = core.NewEU(0x0101, node.state, node.api, config)
	node.blockNo++

	receipts := make([]*mevmTypes.Receipt, 0, len(txs))
	for _, tx := range txs {
		var transaction mevmTypes.Transaction
		err := rlp.DecodeBytes(tx, &transaction)
		if err != nil {
			panic(err)
		}

		msg, err := transaction.AsMessage(mevmTypes.NewEIP155Signer(new(big.Int).SetUint64(1)))
		if err != nil {
			panic(err)
		}

		euResult, receipt := runSequentialTx(node.eu, node.api, transaction.Hash(), &msg, config)
		receipts = append(receipts, receipt)

		node.cache.Commit(euResult.W)
		node.snapshot.Commit(euResult.W.ClibWrites)
	}
	return receipts
}

func (node *FullNode) ExecuteTxs(request *pbs.NewTransactionsRequest) []pbc.Receipt {
	config := createTestConfigOnBlock(node.blockNo)
	node.eu = core.NewEU(0x0101, node.state, node.api, config)
	node.blockNo++

	var receipts []pbc.Receipt
	for _, txs := range request.Transactions {
		for _, t := range txs.List {
			// log.WithFields(log.Fields{
			// 	"raw": common.Bytes2Hex(t.RawTransaction),
			// }).Debug("Before decoding")
			var tx *mevmTypes.Transaction
			err := rlp.DecodeBytes(t.RawTransaction, &tx)
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Warn("Decode failed")
				continue
			}
			// log.WithFields(log.Fields{
			// 	"tx": tx,
			// }).Debug("Decode complete")

			msg, err := tx.AsMessage(mevmTypes.NewEIP155Signer(new(big.Int).SetUint64(1)))
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Warn("Validate signature failed")
				continue
			} else {
				// log.WithFields(log.Fields{
				// 	"msg": msg,
				// }).Debug("Convert tx to msg")
			}

			euResult, receipt := runSequentialTx(node.eu, node.api, common.BytesToHash(t.Hash), &msg, config)
			logs := make([]*pbc.Receipt_Log, 0, len(receipt.Logs))
			for _, log := range receipt.Logs {
				topics := make([][]byte, 0, len(log.Topics))
				for _, topic := range log.Topics {
					topics = append(topics, topic.Bytes())
				}
				logs = append(logs, &pbc.Receipt_Log{
					Topics:  topics,
					Data:    log.Data,
					Address: log.Address.Bytes(),
				})
			}
			receipts = append(receipts, pbc.Receipt{
				Txhash:          receipt.TxHash.Bytes(),
				ContractAddress: receipt.ContractAddress.Bytes(),
				Status:          receipt.Status,
				Logs:            logs,
			})

			node.cache.Commit(euResult.W)
			node.snapshot.Commit(euResult.W.ClibWrites)
		}
	}

	time.Sleep(time.Second * 3)
	return receipts
}

//coinbase common.Address
func runSequentialTx(eu *core.EU, api *adaptor.API, hash common.Hash, msg *mevmTypes.Message, config *core.Config) (*types.EuResult, *mevmTypes.Receipt) {
	euResult, receipt := eu.Run(hash, msg, config)
	rs, ws := api.Collect()
	var retEuResult *types.EuResult
	if euResult.Status == 1 { // success
		balanceReads := make(map[types.Address]*big.Int)
		for addr, amount := range euResult.R.BalanceReads {
			balanceReads[types.Address(addr.Bytes())] = amount
		}
		storageReads := make([]types.Address, 0, len(euResult.R.EthStorageReads))
		for _, addr := range euResult.R.EthStorageReads {
			storageReads = append(storageReads, types.Address(addr.Bytes()))
		}
		reads := &types.Reads{
			ClibReads:       rs,
			BalanceReads:    balanceReads,
			EthStorageReads: storageReads,
		}
		newAccounts := make([]types.Address, 0, len(euResult.W.NewAccounts))
		for _, addr := range euResult.W.NewAccounts {
			newAccounts = append(newAccounts, types.Address(addr.Bytes()))
		}
		balanceWrites := make(map[types.Address]*big.Int)
		for addr, amount := range euResult.W.BalanceWrites {
			balanceWrites[types.Address(addr.Bytes())] = amount
		}
		nonceWrites := make(map[types.Address]uint64)
		for addr, nonce := range euResult.W.NonceWrites {
			nonceWrites[types.Address(addr.Bytes())] = nonce
		}
		codeWrites := make(map[types.Address][]byte)
		for addr, code := range euResult.W.CodeWrites {
			codeWrites[types.Address(addr.Bytes())] = code
		}
		storageWrites := make(map[types.Address]map[string]string)
		for addr, ethStorageWrite := range euResult.W.EthStorageWrites {
			sw := make(map[string]string)
			for k, v := range ethStorageWrite {
				sw[string(k.Bytes())] = string(v.Bytes())
			}
			storageWrites[types.Address(addr.Bytes())] = sw
		}
		writes := &types.Writes{
			ClibWrites:       ws,
			NewAccounts:      newAccounts,
			BalanceWrites:    balanceWrites,
			NonceWrites:      nonceWrites,
			CodeWrites:       codeWrites,
			EthStorageWrites: storageWrites,
		}
		retEuResult = &types.EuResult{
			R: reads,
			W: writes,
		}
	} else { // fail
		balanceWrites := make(map[types.Address]*big.Int)
		for addr, amount := range euResult.W.BalanceWrites {
			balanceWrites[types.Address(addr.Bytes())] = amount
		}
		writes := &types.Writes{
			BalanceWrites: balanceWrites,
		}
		retEuResult = &types.EuResult{
			W: writes,
		}
		log.WithFields(log.Fields{
			"msg": msg,
		}).Warn("Failed to run tx")
	}
	retEuResult.H = string(euResult.H.Bytes())
	retEuResult.GasUsed = euResult.GasUsed
	retEuResult.Status = euResult.Status
	retEuResult.DC = api.GetDeferCall()
	return retEuResult, receipt
}
