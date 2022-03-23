package workers

import (
	"github.com/arcology-network/component-lib/actor"
	"github.com/arcology-network/component-lib/log"
	"go.uber.org/zap"
)

type Initializer struct {
	actor.WorkerThread
}

//return a Subscriber struct
func NewInitializer(concurrency int, groupid string) *Initializer {
	in := Initializer{}
	in.Set(concurrency, groupid)
	return &in
}

func (i *Initializer) OnStart() {
}

func (i *Initializer) OnMessageArrived(msgs []*actor.Message) error {
	i.AddLog(log.LogLevel_Info, "client-svc initialize ", zap.String("send command", actor.MsgStartSub))
	i.MsgBroker.Send(actor.MsgStartSub, "")
	return nil
}
