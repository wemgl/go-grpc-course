package main

import (
	"context"
	"fmt"
	"github.com/wemgl/greet/greetpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"io"
	"log"
	"os"
	"strconv"
	"time"
)

const certFile = "ssl/ca.crt"

func main() {
	var opts []grpc.DialOption
	if lookupEnv, ok := os.LookupEnv("TLS_ENABLED"); ok {
		tlsEnabled, err := strconv.ParseBool(lookupEnv)
		if err != nil {
			log.Fatalf("failed to parse TLS_ENABLED environment variable: %v", err)
		}
		if tlsEnabled {
			// Certificate Authority trust certificate
			creds, err := credentials.NewClientTLSFromFile(certFile, "")
			if err != nil {
				log.Fatalf("failed to read CA trust certificate file: %v", err)
			}
			log.Printf("client connected with TLS: %t", tlsEnabled)
			opts = append(opts, grpc.WithTransportCredentials(creds))
		} else {
			log.Println("tls isn't enabled")
			opts = append(opts, grpc.WithInsecure())
		}
	} else {
		log.Println("couldn't lookup env")
		opts = append(opts, grpc.WithInsecure())
	}

	conn, err := grpc.Dial("localhost:50051", opts...)
	if err != nil {
		log.Fatalf("failed to connect to server")
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Fatalf("failed to close client connection: %v", err)
		}
	}()
	c := greetpb.NewGreetServiceClient(conn)
	doUnary(c)
	// doServerStreaming(c)
	// doClientStreaming(c)
	// doBiDirectionalStreaming(c)
	// doUnaryWithDeadline(c, time.Second*1) // should complete
	// doUnaryWithDeadline(c, time.Second*5) // should timeout
}

func doBiDirectionalStreaming(c greetpb.GreetServiceClient) {
	fmt.Println("Starting to do Bi-Directional Streaming RPC…")
	// create a stream by invoking the client
	stream, err := c.GreetEveryone(context.Background())
	if err != nil {
		log.Fatalf("failed to retrieve stream from server: %v", err)
	}
	waitc := make(chan struct{})
	go func() {
		for _, greeting := range makeGreetings() {
			log.Print("sending message: ", greeting.String())
			err := stream.Send(&greetpb.GreetEveryoneRequest{
				Greeting: greeting,
			})
			if err != nil {
				log.Fatalf("failed to send greeting for %s: %v", greeting.GetFirstName(), err)
			}
		}
		if err := stream.CloseSend(); err != nil {
			log.Fatalf("failed to close stream: %v", err)
		}
	}()
	go func() {
		for {
			recv, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Fatalf("failed to receive value from server: %v", err)
			}
			log.Println("Received ", recv.GetResult())
		}
		waitc <- struct{}{}
	}()
	<-waitc
}

func doClientStreaming(c greetpb.GreetServiceClient) {
	fmt.Println("Starting to do Client Streaming RPC…")
	stream, err := c.LongGreet(context.Background())
	if err != nil {
		log.Fatalf("failed to start streaming long greets: %v", err)
	}
	for _, greeting := range makeGreetings() {
		err := stream.Send(&greetpb.LongGreetRequest{
			Greeting: greeting,
		})
		if err != nil {
			log.Fatalf("failed to send request for %s: %v", greeting.GetFirstName(), err)
		}
	}
	recv, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("failed to get long greet request")
	}
	log.Printf("Hi Riverdale!:\n%s", recv.GetResult())
}

func makeGreetings() (greetings []*greetpb.Greeting) {
	names := []string{
		"Archie",
		"Reggie",
		"Veronica",
		"Betty",
		"JugHead",
	}
	for _, name := range names {
		greetings = append(greetings, &greetpb.Greeting{
			FirstName: name,
		})
	}
	return
}

func doServerStreaming(c greetpb.GreetServiceClient) {
	fmt.Println("Starting to do a Server Streaming RCP…")
	req := &greetpb.GreetManyTimesRequest{
		Greeting: &greetpb.Greeting{
			FirstName: "Wembley",
			LastName:  "Leach",
		},
	}
	resStream, err := c.GreetManyTimes(context.Background(), req)
	if err != nil {
		fmt.Printf("greet rpc failed: %v", err)
		return
	}
	for {
		msg, err := resStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("error while reading stream: %v", err)
		}
		log.Printf("Reponse from greet many times: %s", msg.GetResult())
	}
}

func doUnary(c greetpb.GreetServiceClient) {
	fmt.Println("Starting to do a Unary RCP…")
	ctx := context.Background()
	req := &greetpb.GreetRequest{
		Greeting: &greetpb.Greeting{
			FirstName: "Wembley",
			LastName:  "Leach",
		},
	}
	greet, err := c.Greet(ctx, req)
	if err != nil {
		fmt.Printf("greet rpc failed: %v", err)
		return
	}
	fmt.Println(greet.GetResult())
}

func doUnaryWithDeadline(c greetpb.GreetServiceClient, timeout time.Duration) {
	fmt.Println("Starting to do a Unary with deadline RCP…")
	req := &greetpb.GreetWithDeadlineRequest{
		Greeting: &greetpb.Greeting{
			FirstName: "Wembley",
			LastName:  "Leach",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	greet, err := c.GreetWithDeadline(ctx, req)
	if err != nil {
		if errStat, ok := status.FromError(err); ok {
			if errStat.Code() == codes.DeadlineExceeded {
				log.Print("timeout hit! deadline was exceeded")
			} else {
				log.Fatalf("unexpected error: %v", errStat.Err())
			}
		} else {
			log.Fatalf("greet rpc failed: %v", err)
		}
	}

	fmt.Println(greet.GetResult())
}
