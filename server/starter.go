package server

import (
	"context"
	"net"

	tmCommon "github.com/arcology-network/3rd-party/tm/common"
	pb "github.com/arcology-network/client-svc/clientsvc"
	"github.com/arcology-network/common-lib/types"
	"github.com/arcology-network/component-lib/rpc"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start client service Daemon",
	RunE:  startCmd,
}

func init() {
	flags := StartCmd.Flags()

	flags.String("mqaddr", "localhost:9092", "host:port of kafka ")
	flags.String("mqaddr2", "localhost:9092", "host:port of kafka ")

	flags.String("receipts-scs", "receipts-scs", "topic for send  receipts of smartcontracts")

	flags.String("msgexch", "msgexch", "topic for msg exchange")

	flags.String("inclusive-txs", "inclusive-txs", "topic for receive txlist")

	flags.Int("concurrency", 4, "num of threads")

	flags.String("log", "log", "topic for send log")

	flags.String("local-txs", "local-txs", "topic for received txs from local")

	flags.String("logcfg", "./log.toml", "log conf path")
	flags.Bool("loop", false, "loop mode")

	flags.Uint64("insid", 1, "instance id of client-svc,range 1 to 255")

	flags.Bool("draw", false, "draw flow graph")
	flags.Int("nidx", 0, "node index in cluster")
	flags.String("nname", "node1", "node name in cluster")

	flags.Int("txnums", 1000, "tx nums per package")
	flags.Int("waits", 10, "wait seconds ")

	flags.String("zkUrl", "127.0.0.1:2181", "url of zookeeper")
	flags.String("rpcSrv", "127.0.0.1:8975", "client rpc service")
	flags.String("dbSrv", "127.0.0.1:8976", "storage rpc service")
}

var clientSvc pb.ClientServiceServer

func sendTransactions(ctx context.Context, args *types.SendTransactionArgs, reply *types.SendTransactionReply) error {
	clientSvc.(*clientService).SendTxs(args.Txs)
	reply.Status = 0
	return nil
}

func startCmd(cmd *cobra.Command, args []string) error {
	conn, err := net.Listen("tcp", ":50000")
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Failed to start service")
		return err
	}

	clientSvc = newClientService()
	// if viper.GetBool("draw") {
	// 	return nil
	// }
	server := grpc.NewServer()
	pb.RegisterClientServiceServer(server, clientSvc)
	go server.Serve(conn)

	// rpcx service
	rpc.InitZookeeperRpcServer(viper.GetString("rpcSrv"), "client", []string{viper.GetString("zkUrl")}, nil, []interface{}{sendTransactions})

	// Wait forever
	tmCommon.TrapSignal(func() {
		// Cleanup
	})

	return nil
}
