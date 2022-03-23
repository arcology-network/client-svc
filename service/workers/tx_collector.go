package workers

import (
	"time"

	"github.com/arcology-network/component-lib/actor"
)

type TxCollector struct {
	actor.WorkerThread
	waitSeconds int
	maxSize     int

	txChan   chan []byte
	exitChan chan bool
}

//return a Subscriber struct
func NewTxCollector(concurrency int, groupid string, maxSize, waitSeconds int) *TxCollector {
	collector := TxCollector{}
	collector.Set(concurrency, groupid)
	collector.waitSeconds = waitSeconds
	collector.maxSize = maxSize

	return &collector
}

func (tc *TxCollector) OnStart() {
	ticker := time.NewTicker(time.Duration(tc.waitSeconds) * time.Second)
	tc.txChan = make(chan []byte, 50)
	tc.exitChan = make(chan bool, 0)
	go func() {
		txs := make([][]byte, 0, tc.maxSize)
		for {
			select {
			case tx := <-tc.txChan:
				txs = append(txs, tx)
				if len(txs) >= tc.maxSize {
					tc.MsgBroker.Send(actor.MsgTxLocals, txs)
					txs = make([][]byte, 0, tc.maxSize)
				}
			case <-ticker.C:
				if len(txs) > 0 {
					tc.MsgBroker.Send(actor.MsgTxLocals, txs)
					txs = make([][]byte, 0, tc.maxSize)
				}
			case <-tc.exitChan:
				ticker.Stop()
				break
			}
		}
	}()

}

func (tc *TxCollector) OnMessageArrived(msgs []*actor.Message) error {
	for _, v := range msgs {
		switch v.Name {
		case actor.MsgTxLocal:
			data := v.Data.([]byte)
			tc.txChan <- data
		}
	}
	return nil
}
