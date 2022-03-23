package workers

import (
	ethTypes "github.com/arcology-network/3rd-party/eth/types"
	pbc "github.com/arcology-network/client-svc/dappcontainer"
	"github.com/arcology-network/common-lib/common"
	"github.com/arcology-network/common-lib/types"
	"github.com/arcology-network/component-lib/actor"
	"github.com/arcology-network/component-lib/aggregator/aggregator"
	"github.com/arcology-network/component-lib/log"
	"go.uber.org/zap"
)

type AggreSelector struct {
	actor.WorkerThread
	aggregator   *aggregator.Aggregator
	savedMessage *actor.Message
}

//return a Subscriber struct
func NewAggreSelector(concurrency int, groupid string) *AggreSelector {
	agg := AggreSelector{}
	agg.Set(concurrency, groupid)
	agg.aggregator = aggregator.NewAggregator()
	return &agg
}

func (a *AggreSelector) OnStart() {

}

func (a *AggreSelector) OnMessageArrived(msgs []*actor.Message) error {
	for _, v := range msgs {
		switch v.Name {
		case actor.MsgBlockCompleted:
			remainingQuantity := a.aggregator.OnClearInfoReceived()
			a.AddLog(log.LogLevel_Info, " AggreSelector clear pool", zap.Int("remainingQuantity", remainingQuantity))
		case actor.MsgInclusive:
			a.savedMessage = v.CopyHeader()
			inclusive := msgs[0].Data.(*types.InclusiveList)
			inclusive.Mode = types.InclusiveMode_Results
			copyInclusive := inclusive.CopyListAddHeight(v.Height, v.Round)
			result, _ := a.aggregator.OnListReceived(copyInclusive)
			a.SendMsg(result)
		case actor.MsgReceipts:
			receipts := v.Data.(*[]*ethTypes.Receipt)
			for _, r := range *receipts {
				receipt := r
				newhash := common.ToNewHash(receipt.TxHash, v.Height, v.Round)
				result := a.aggregator.OnDataReceived(newhash, receipt)
				a.SendMsg(result)
			}

		}
	}
	return nil
}
func (a *AggreSelector) SendMsg(SelectedData *[]*interface{}) {
	if SelectedData != nil {
		receipts := make([]pbc.Receipt, len(*SelectedData))
		for ri, v := range *SelectedData {
			rcpt := (*v).(*ethTypes.Receipt)

			logs := make([]*pbc.Receipt_Log, len(rcpt.Logs))
			for li, t := range rcpt.Logs {
				topics := make([][]byte, len(t.Topics))
				for i, tt := range t.Topics {
					tthash := tt
					topics[i] = tthash.Bytes()
				}
				logs[li] = &pbc.Receipt_Log{
					Topics:  topics,
					Data:    t.Data,
					Address: t.Address.Bytes(),
				}
			}

			receipts[ri] = pbc.Receipt{
				Txhash:          rcpt.TxHash.Bytes(),
				ContractAddress: rcpt.ContractAddress.Bytes(),
				Status:          rcpt.Status,
				Logs:            logs,
			}

		}
		a.LatestMessage = a.savedMessage
		a.MsgBroker.LatestMessage = a.savedMessage
		a.AddLog(log.LogLevel_Info, "AggreSelector send selected receipts", zap.Int("nums", len(receipts)))
		a.MsgBroker.Send(actor.MsgSelectedReceipts, &receipts)
	}
}
