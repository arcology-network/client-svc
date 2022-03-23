package service

import (
	"context"
	"sync"

	"google.golang.org/grpc"

	pbs "github.com/arcology-network/client-svc/clientsvc"
	"github.com/arcology-network/client-svc/dappcontainer"
	pbc "github.com/arcology-network/client-svc/dappcontainer"
	log "github.com/sirupsen/logrus"
)

type dappClient struct {
	conn     *grpc.ClientConn
	rpc      pbc.DAppContainerClient
	dappSubs map[string]struct{} // DApps that subscribed by this client.
}

type lockableTxs struct {
	mu  sync.Mutex
	txs map[string]struct{}
}

type ClientServiceImpl struct {
	cmu          sync.RWMutex
	clients      map[string]*dappClient
	tmu          sync.RWMutex
	pendingTxs   map[string]*lockableTxs
	smu          sync.RWMutex
	contractSubs map[string]string // Contracts that subscribed by clients.
}

func NewClientServiceImpl() *ClientServiceImpl {
	return &ClientServiceImpl{
		clients:      make(map[string]*dappClient),
		pendingTxs:   make(map[string]*lockableTxs),
		contractSubs: make(map[string]string),
	}
}

func (service *ClientServiceImpl) RegisterNewClient(request *pbs.NewClientRequest) error {
	dappSubs := make(map[string]struct{})
	for dappName, sub := range request.SubInfos {
		service.smu.Lock()
		if sub.Type == "contract" {
			service.contractSubs[string(sub.Value)] = request.Address
		} else {
			dappSubs[dappName] = struct{}{}
		}
		service.smu.Unlock()
	}

	service.cmu.RLock()
	_, ok := service.clients[request.Address]
	service.cmu.RUnlock()

	if !ok {
		log.Debugf("Before connect to %s\n", request.Address)
		conn, err := grpc.Dial(request.Address, grpc.WithInsecure(), grpc.WithBlock())
		log.Debugf("After connect to %s\n", request.Address)
		if err != nil {
			return err
		}
		rpc := dappcontainer.NewDAppContainerClient(conn)
		service.cmu.Lock()
		service.clients[request.Address] = &dappClient{
			conn:     conn,
			rpc:      rpc,
			dappSubs: dappSubs,
		}
		service.cmu.Unlock()
	} else {
		service.cmu.Lock()
		service.clients[request.Address].dappSubs = dappSubs
		service.cmu.Unlock()
	}
	return nil
}

func (service *ClientServiceImpl) UnregisterClient(request *pbs.UnregisterClientRequest) error {
	service.cmu.Lock()
	defer service.cmu.Unlock()
	c, ok := service.clients[request.Address]
	if ok {
		delete(service.clients, request.Address)
		c.conn.Close()
		service.tmu.Lock()
		delete(service.pendingTxs, request.Address)
		service.tmu.Unlock()
		service.smu.Lock()
		for k, v := range service.contractSubs {
			if v == request.Address {
				delete(service.contractSubs, k)
			}
		}
		service.smu.Unlock()
	}
	return nil
}

func (service *ClientServiceImpl) OnNewTransactionsReceived(request *pbs.NewTransactionsRequest) error {
	go func() {
		service.cmu.RLock()
		c, ok := service.clients[request.Client]
		service.cmu.RUnlock()
		if !ok {
			log.WithFields(log.Fields{
				"address": request.Client,
			}).Warn("Transactions from unknown client received")
			return
		}

		//log.Debugf("Transaction received: %v", request.Transactions)
		pendingTxs := make(map[string][]string)
		for dappName, txs := range request.Transactions {
			service.smu.RLock()
			if _, ok := c.dappSubs[dappName]; ok {
				for _, tx := range txs.List {
					pendingTxs[request.Client] = append(pendingTxs[request.Client], string(tx.Hash))
				}
			}

			for _, tx := range txs.List {
				if address, ok := service.contractSubs[string(tx.To)]; ok {
					pendingTxs[address] = append(pendingTxs[address], string(tx.Hash))
				}
			}
			service.smu.RUnlock()
		}

		service.tmu.Lock()
		for name, txs := range pendingTxs {
			if _, ok := service.pendingTxs[name]; !ok {
				service.pendingTxs[name] = &lockableTxs{
					txs: make(map[string]struct{}),
				}
			}
			service.pendingTxs[name].mu.Lock()
			for _, tx := range txs {
				service.pendingTxs[name].txs[tx] = struct{}{}
			}
			service.pendingTxs[name].mu.Unlock()
		}
		service.tmu.Unlock()
	}()
	return nil
}

func (service *ClientServiceImpl) OnNewReceiptsReceived(receipts []pbc.Receipt) error {
	log.WithFields(log.Fields{
		"num": len(receipts),
	}).Debug("Receipts received")

	go func() {
		waitForSend := make(map[string][]pbc.Receipt)
		for _, r := range receipts {
			for address, pendings := range service.pendingTxs {
				pendings.mu.Lock()
				if _, ok := pendings.txs[string(r.Txhash)]; ok {
					waitForSend[address] = append(waitForSend[address], r)
					delete(pendings.txs, string(r.Txhash))
				}
				pendings.mu.Unlock()
			}
		}

		service.cmu.RLock()
		defer service.cmu.RUnlock()
		for address, client := range service.clients {
			var rs []pbc.Receipt
			if _, ok := waitForSend[address]; ok {
				rs = waitForSend[address]
			}
			go func(address string, client *dappClient, receipts []pbc.Receipt) {
				if c, ok := service.clients[address]; ok {
					stream, err := c.rpc.NewReceipts(context.Background())
					if err != nil {
						log.WithFields(log.Fields{
							"error":   err,
							"address": address,
						}).Error("Failed to open stream for sending receipts")
						return
					}

					for _, r := range receipts {
						err := stream.Send(&r)
						if err != nil {
							log.WithFields(log.Fields{
								"error":   err,
								"address": address,
							}).Error("Failed to send receipt to client")
						}
					}

					resp, err := stream.CloseAndRecv()
					if err != nil {
						log.WithFields(log.Fields{
							"error":   err,
							"address": address,
						}).Error("Failed to receive response from client")
					} else if resp.Status != 0 {
						log.WithFields(log.Fields{
							"status":  resp.Status,
							"address": address,
						}).Error("Non-zero status received")
					}
				}
			}(address, client, rs)
		}
	}()
	return nil
}
