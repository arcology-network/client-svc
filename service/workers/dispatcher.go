package workers

import (
	pbc "github.com/arcology-network/client-svc/dappcontainer"
	"github.com/arcology-network/client-svc/service"
	"github.com/arcology-network/component-lib/actor"
)

type Dispatcher struct {
	actor.WorkerThread
	impl *service.ClientServiceImpl
}

//return a Subscriber struct
func NewDispatcher(concurrency int, groupid string, impl *service.ClientServiceImpl) *Dispatcher {
	d := Dispatcher{}
	d.Set(concurrency, groupid)
	d.impl = impl
	return &d
}

func (d *Dispatcher) OnStart() {

}

func (d *Dispatcher) OnMessageArrived(msgs []*actor.Message) error {
	var receipts *[]pbc.Receipt
	result := ""
	for _, v := range msgs {
		switch v.Name {
		case actor.MsgSelectedReceipts:
			receipts = v.Data.(*[]pbc.Receipt)
			isnil, err := d.IsNil(receipts, "receipts")
			if isnil {
				return err
			}
		case actor.MsgBlockCompleted:
			result = v.Data.(string)
		}
	}

	if result == actor.MsgBlockCompleted_Success && receipts != nil {
		// d.impl.OnNewReceiptsReceived(*receipts)
	}

	return nil
}
