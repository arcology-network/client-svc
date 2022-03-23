package server

import (
	"context"
	"encoding/gob"
	"fmt"
	"net/http"

	pb "github.com/arcology-network/client-svc/clientsvc"
	"github.com/arcology-network/client-svc/dappcontainer"
	"github.com/arcology-network/client-svc/mock"
	"github.com/arcology-network/client-svc/service"
	"github.com/arcology-network/client-svc/service/workers"
	"github.com/arcology-network/common-lib/types"
	"github.com/arcology-network/component-lib/actor"
	"github.com/arcology-network/component-lib/kafka"

	clog "github.com/arcology-network/component-lib/log"
	"github.com/arcology-network/component-lib/streamer"
	mevmTypes "github.com/arcology-network/mevm/geth/core/types"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Block struct {
	Height    int
	Timestamp int
	Txs       []string
}

type clientService struct {
	impl   *service.ClientServiceImpl
	broker *streamer.StatefulStreamer

	concurrency int
	groupid     string
	loopMode    bool
	mockNode    *mock.FullNode
	receiptsDB  map[string]*mevmTypes.Receipt
	blocksDB    map[int]*Block
	latestBlock int
}

func (cservice *clientService) RegisterNewClient(ctx context.Context, request *pb.NewClientRequest) (*pb.NewClientResponse, error) {
	log.WithFields(log.Fields{
		"request": request,
	}).Debug("RegisterNewClient")

	err := cservice.impl.RegisterNewClient(request)
	status := 0
	if err != nil {
		status = 1
	}
	return &pb.NewClientResponse{
		Status: int32(status),
	}, nil
}

func (cservice *clientService) UnregisterClient(ctx context.Context, request *pb.UnregisterClientRequest) (*pb.UnregisterClientResponse, error) {
	log.WithFields(log.Fields{
		"request": request,
	}).Debug("UnregisterClient")

	err := cservice.impl.UnregisterClient(request)
	status := 0
	if err != nil {
		status = 1
	}
	return &pb.UnregisterClientResponse{
		Status: int32(status),
	}, nil
}

func (cservice *clientService) NewTransactions(ctx context.Context, request *pb.NewTransactionsRequest) (*pb.NewTransactionsResponse, error) {
	// err := cservice.impl.OnNewTransactionsReceived(request)
	status := 0
	// if err != nil {
	// 	status = 1
	// }

	for _, txs := range request.Transactions {
		for _, t := range txs.List {
			tx := t.RawTransaction

			cservice.SendTx(tx)
		}
	}
	defer cservice.impl.OnNewReceiptsReceived([]dappcontainer.Receipt{})

	// go func() {
	// 	if cservice.loopMode {
	// 		receipts := cservice.mockNode.ExecuteTxs(request)
	// 		cservice.impl.OnNewReceiptsReceived(receipts)
	// 	} else {
	// 		for _, txs := range request.Transactions {
	// 			for _, t := range txs.List {
	// 				tx := t.RawTransaction
	// 				//hash := t.Hash

	// 				cservice.sendTx(sendingTx)
	// 			}
	// 		}
	// 	}
	// }()

	return &pb.NewTransactionsResponse{
		Status: int32(status),
	}, nil
}

func (cservice *clientService) SendTx(tx []byte) {
	sendingTx := make([]byte, len(tx)+1)
	bz := 0
	bz += copy(sendingTx[bz:], []byte{types.TxType_Eth})
	bz += copy(sendingTx[bz:], tx)

	streamerTx := actor.Message{
		Name:   actor.MsgTxLocal,
		Height: 0,
		Round:  0,
		Data:   sendingTx,
	}
	cservice.broker.Send(actor.MsgTxLocal, &streamerTx)
}

func (cservice *clientService) SendTxs(txs [][]byte) {
	if cservice.loopMode {
		receipts := cservice.mockNode.Run(txs)
		txs := make([]string, 0, len(receipts))
		for _, r := range receipts {
			fmt.Printf("Receipt hash: %v\n", r.TxHash.String())
			cservice.receiptsDB[r.TxHash.String()] = r
			txs = append(txs, r.TxHash.String())
		}
		cservice.blocksDB[cservice.mockNode.GetBlockNo()] = &Block{
			Height:    cservice.mockNode.GetBlockNo(),
			Timestamp: cservice.mockNode.GetTimestamp(),
			Txs:       txs,
		}
		cservice.latestBlock = cservice.mockNode.GetBlockNo()
	} else {
		for _, tx := range txs {
			cservice.SendTx(tx)
		}
	}
}

func (cservice *clientService) GetReceipt(hash string) *mevmTypes.Receipt {
	if hash[:2] != "0x" {
		hash = "0x" + hash
	}
	r, ok := cservice.receiptsDB[hash]
	if ok {
		return r
	}
	return nil
}

func (cservice *clientService) GetBlock(height int) *Block {
	if height == -1 {
		height = cservice.latestBlock
	}
	if block, ok := cservice.blocksDB[height]; ok {
		return block
	}
	return nil
}

func (cservice *clientService) StartStreamer() {
	http.Handle("/streamer", promhttp.Handler())
	go http.ListenAndServe(":19002", nil)

	gob.Register([][]byte{})

	//00 initializer
	initializer := actor.NewActor(
		"initializer",
		cservice.broker,
		[]string{actor.MsgStarting},
		[]string{
			actor.MsgStartSub,
		},
		[]int{1},
		workers.NewInitializer(cservice.concurrency, cservice.groupid),
	)
	initializer.Connect(streamer.NewDisjunctions(initializer, 1))

	/*
		receiveMseeages := []string{
			actor.MsgInclusive,
			actor.MsgReceipts,
			actor.MsgBlockCompleted,
		}
		receiveTopics := []string{
			viper.GetString("inclusive-txs"),
			viper.GetString("msgexch"),
			viper.GetString("receipts-scs"),
		}
		//01 kafkaDownloader
		kafkaDownloader := actor.NewActor(
			"kafkaDownloader",
			cservice.broker,
			[]string{actor.MsgStartSub},
			receiveMseeages,
			[]int{1000, 1000, 1000},
			kafka.NewKafkaDownloader(cservice.concurrency, cservice.groupid, receiveTopics, receiveMseeages),
		)
		kafkaDownloader.Connect(streamer.NewDisjunctions(kafkaDownloader, 1000))
	*/
	//02 collector
	collector := actor.NewActor(
		"collector",
		cservice.broker,
		[]string{
			actor.MsgTxLocal,
		},
		[]string{actor.MsgTxLocals},
		[]int{10},
		workers.NewTxCollector(cservice.concurrency, cservice.groupid, viper.GetInt("txnums"), viper.GetInt("waits")),
	)
	collector.Connect(streamer.NewDisjunctions(collector, 100))

	/*
		//03 aggre
		aggreSelector := actor.NewActor(
			"aggreSelector",
			cservice.broker,
			[]string{
				actor.MsgReceipts,
				actor.MsgInclusive,
				actor.MsgBlockCompleted,
			},
			[]string{actor.MsgSelectedReceipts},
			[]int{1},
			workers.NewAggreSelector(cservice.concurrency, cservice.groupid),
		)
		aggreSelector.Connect(streamer.NewDisjunctions(aggreSelector, 1))

		//04 dispatcher
		dispatcher := actor.NewActor(
			"dispatcher",
			cservice.broker,
			[]string{
				actor.MsgSelectedReceipts,
				actor.MsgBlockCompleted,
			},
			[]string{},
			[]int{},
			workers.NewDispatcher(cservice.concurrency, cservice.groupid, cservice.impl),
		)
		dispatcher.Connect(streamer.NewConjunctions(dispatcher))
	*/
	relations := map[string]string{}
	relations[actor.MsgTxLocals] = viper.GetString("local-txs")

	//05 kafkaUploader
	kafkaUploader := actor.NewActor(
		"kafkaUploader",
		cservice.broker,
		[]string{
			actor.MsgTxLocals,
		},
		[]string{},
		[]int{},
		kafka.NewKafkaUploader(cservice.concurrency, cservice.groupid, relations, viper.GetString("mqaddr2")),
	)
	kafkaUploader.Connect(streamer.NewDisjunctions(kafkaUploader, 1))

	//starter
	selfStarter := streamer.NewDefaultProducer(
		"selfStarter",
		[]string{
			actor.MsgStarting,
			//actor.MsgClientHandle,
			actor.MsgTxLocal,
		},
		[]int{1, 1000},
	)
	cservice.broker.RegisterProducer(selfStarter)

	cservice.broker.Serve()

	//start signel
	streamerStarting := actor.Message{
		Name:   actor.MsgStarting,
		Height: 0,
		Round:  0,
		Data:   "start",
	}
	cservice.broker.Send(actor.MsgStarting, &streamerStarting)

}

func newClientService() pb.ClientServiceServer {
	clog.InitLog("client.log", viper.GetString("logcfg"), "client", viper.GetString("nname"), viper.GetInt("nidx"))

	clientSvc := clientService{
		impl:        service.NewClientServiceImpl(),
		broker:      streamer.NewStatefulStreamer(),
		concurrency: viper.GetInt("concurrency"),
		groupid:     "client" + viper.GetString("insid"),
		loopMode:    viper.GetBool("loop"),
	}

	if !clientSvc.loopMode {
		clientSvc.StartStreamer()
		log.Info("Start client-svc in non-loop mode")

		// if viper.GetBool("draw") {
		// 	clog.CompleteMetaInfo("client")
		// }

	} else {
		log.Info("Start client-svc in loop mode")
		clientSvc.mockNode = mock.NewFullNode()
		clientSvc.receiptsDB = make(map[string]*mevmTypes.Receipt)
		clientSvc.blocksDB = make(map[int]*Block)
		clientSvc.latestBlock = -1
	}

	return &clientSvc
}
