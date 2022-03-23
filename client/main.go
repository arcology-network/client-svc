package main

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	"github.com/arcology-network/client-svc/dappcontainer"
)

func main() {
	conn, err := grpc.Dial("localhost:50001", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		fmt.Println("grpc.Dial error")
	}
	defer conn.Close()

	c := dappcontainer.NewDAppContainerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream, err := c.NewReceipts(ctx)
	if err != nil {
		fmt.Println("c.NewReceipts error")
	}

	for n := 0; n < 5; n++ {
		err := stream.Send(&dappcontainer.Receipt{
			Txhash: []byte{byte(n)},
		})
		if err != nil {
			fmt.Println("stream.Send error")
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		fmt.Println("stream.CloseAndRecv error")
	}

	fmt.Println(resp.Status)
}
