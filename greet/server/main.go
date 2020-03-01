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
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type server struct{}

func (s *server) GreetWithDeadline(
	ctx context.Context,
	req *greetpb.GreetWithDeadlineRequest,
) (*greetpb.GreetWithDeadlineResponse, error) {
	for i := 0; i < 3; i++ {
		if ctx.Err() == context.Canceled {
			// the client cancelled the request
			log.Println("client cancelled the request")
			return nil, status.Error(
				codes.Canceled,
				"the client cancelled the request",
			)
		}
		time.Sleep(time.Second * 1)
	}
	fn := req.GetGreeting().GetFirstName()
	result := fmt.Sprintf("Hello %s", fn)
	res := &greetpb.GreetWithDeadlineResponse{
		Result: result,
	}
	return res, nil
}

func (s *server) GreetEveryone(stream greetpb.GreetService_GreetEveryoneServer) error {
	for {
		recv, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("failed to receive streamed value from client: %v", err)
		}
		firstName := recv.GetGreeting().GetFirstName()
		err = stream.Send(&greetpb.GreetEveryoneResponse{
			Result: fmt.Sprintf("Hello %s", firstName),
		})
		if err != nil {
			return fmt.Errorf("error while sending greeting to client: %v", err)
		}
	}
}

func (s *server) LongGreet(stream greetpb.GreetService_LongGreetServer) error {
	var res strings.Builder
	for {
		if request, err := stream.Recv(); err != nil {
			if err == io.EOF {
				err := stream.SendAndClose(&greetpb.LongGreetResponse{
					Result: strings.TrimRight(res.String(), " "),
				})
				if err != nil {
					return fmt.Errorf("failed to send greeting to client: %v", err)
				}
				return nil
			}
			return fmt.Errorf("error while reading client stream: %v", err)
		} else {
			greeting := request.GetGreeting()
			res.WriteString(fmt.Sprintf("%s! ", greeting.GetFirstName()))
		}
	}
}

func (s *server) GreetManyTimes(req *greetpb.GreetManyTimesRequest, stream greetpb.GreetService_GreetManyTimesServer) error {
	firstName := req.GetGreeting().GetFirstName()
	for i := 0; i < 10; i++ {
		res := &greetpb.GreetManyTimesResponse{
			Result: fmt.Sprintf("Hello #%d, %s", i, firstName),
		}
		err := stream.Send(res)
		if err != nil {
			return fmt.Errorf("failed to send stream response: %v", err)
		}
		time.Sleep(time.Second)
	}
	return nil
}

func (s *server) Greet(ctx context.Context, req *greetpb.GreetRequest) (*greetpb.GreetResponse, error) {
	fn := req.GetGreeting().GetFirstName()
	result := fmt.Sprintf("Hello %s", fn)
	res := &greetpb.GreetResponse{
		Result: result,
	}
	return res, nil
}

const (
	certFile = "ssl/server.crt"
	keyFile  = "ssl/server.pem"
)

func main() {
	var opts []grpc.ServerOption
	if lookupEnv, ok := os.LookupEnv("TLS_ENABLED"); ok {
		tlsEnabled, err := strconv.ParseBool(lookupEnv)
		if err != nil {
			log.Fatalf("failed to parse TLS_ENABLED environment variable: %v", err)
		}
		if tlsEnabled {
			creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
			if err != nil {
				log.Fatalf("failed loading SSL certificates: %v", err)
			}
			log.Println("run server with TLS enabled")
			opts = append(opts, grpc.Creds(creds))
		} else {
			log.Println("tls isn't enabled")
			opts = append(opts, grpc.EmptyServerOption{})
		}
	} else {
		log.Println("couldn't lookup env")
		opts = append(opts, grpc.EmptyServerOption{})
	}

	lis, err := net.Listen("tcp", "0.0.0.0:50051")
	if err != nil {
		log.Fatalf("failed to create TCP listener: %v", err)
	}

	s := grpc.NewServer(opts...)
	greetpb.RegisterGreetServiceServer(s, &server{})

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to server responses: %v", err)
	}
}
