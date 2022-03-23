package mock

import (
	"encoding/binary"

	"github.com/arcology-network/arbitrator-engine/go-wrapper"
	"github.com/arcology-network/common-lib/types"
	"github.com/arcology-network/mevm/geth/common"
	"github.com/arcology-network/mevm/geth/crypto"
)

type Arbitrator struct {
	group        uint32
	groupIDs     []uint32
	serialIDs    []uint32
	opIDs        []uint32
	scIDs        []uint64
	containerIDs []uint64
	varIDs       []uint64
}

func (arb *Arbitrator) DetectConflict(results []*types.EuResult) []bool {
	for _, result := range results {
		for addr := range result.R.BalanceReads {
			arb.addEthEntry(OP_READ, string(addr), ETH_RW_BALANCE)
		}
		for _, addr := range result.R.EthStorageReads {
			arb.addEthEntry(OP_READ, string(addr), ETH_RW_STORAGE)
		}
		for addr := range result.W.BalanceWrites {
			arb.addEthEntry(OP_ACC_WRITE, string(addr), ETH_RW_BALANCE)
		}
		// for addr := range result.W.NonceWrites {
		// 	arb.addEthEntry(OP_WRITE, string(addr), ETH_RW_NONCE)
		// }
		// for addr := range result.W.CodeWrites {
		// 	arb.addEthEntry(OP_WRITE, string(addr), ETH_RW_CODE)
		// }
		for addr := range result.W.EthStorageWrites {
			arb.addEthEntry(OP_WRITE, string(addr), ETH_RW_STORAGE)
		}

		for addr, rs := range result.R.ClibReads {
			for _, hmr := range rs.HashMapReads {
				arb.addClibEntry(OP_READ, string(addr)+hmr.ID+hmr.Key)
			}
		}
		for addr, ws := range result.W.ClibWrites {
			for _, hmw := range ws.HashMapWrites {
				arb.addClibEntry(OP_WRITE, string(addr)+hmw.ID+hmw.Key)
			}
		}
		arb.group++
	}

	arbitrator := wrapper.Start()
	wrapper.Insert(arbitrator, arb.groupIDs, arb.serialIDs, arb.scIDs, arb.containerIDs, arb.varIDs, arb.opIDs, uint32(len(arb.opIDs)))
	flags := wrapper.Detect(arbitrator, uint32(len(results)))
	wrapper.Clear(arbitrator)
	return flags
}

func (arb *Arbitrator) addEthEntry(opID uint32, addr string, rwType byte) {
	arb.groupIDs = append(arb.groupIDs, arb.group)
	arb.serialIDs = append(arb.serialIDs, 0)
	arb.opIDs = append(arb.opIDs, opID)
	scID, containerID, varID := addr2vid(addr, rwType)
	arb.scIDs = append(arb.scIDs, scID)
	arb.containerIDs = append(arb.containerIDs, containerID)
	arb.varIDs = append(arb.varIDs, varID)
}

func (arb *Arbitrator) addClibEntry(opID uint32, str string) {
	arb.groupIDs = append(arb.groupIDs, arb.group)
	arb.serialIDs = append(arb.serialIDs, 0)
	arb.opIDs = append(arb.opIDs, opID)
	hash := crypto.Keccak256Hash([]byte(str))
	scID, containerID, varID := hash2vid(hash)
	arb.scIDs = append(arb.scIDs, scID)
	arb.containerIDs = append(arb.containerIDs, containerID)
	arb.varIDs = append(arb.varIDs, varID)
}

const (
	ETH_RW_BALANCE = 1
	ETH_RW_NONCE   = 2
	ETH_RW_CODE    = 3
	ETH_RW_STORAGE = 4

	OP_READ      = 0
	OP_WRITE     = 1
	OP_ACC_WRITE = 2
)

func addr2vid(addr string, rwType byte) (uint64, uint64, uint64) {
	var vid [24]byte
	copy(vid[:20], []byte(addr))
	vid[23] = rwType

	scID := binary.LittleEndian.Uint64(vid[:8])
	containerID := binary.LittleEndian.Uint64(vid[8:16])
	varID := binary.LittleEndian.Uint64(vid[16:24])

	return scID, containerID, varID
}

func hash2vid(hash common.Hash) (uint64, uint64, uint64) {
	scID := binary.LittleEndian.Uint64(hash.Bytes()[:8])
	containerID := binary.LittleEndian.Uint64(hash.Bytes()[8:16])
	varID := binary.LittleEndian.Uint64(hash.Bytes()[16:24])

	return scID, containerID, varID
}
