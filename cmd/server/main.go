package main

import (
	"log"
	"net"

	"github.com/afshin-deriv/playground/pb"
	"github.com/afshin-deriv/playground/pkg/executor"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedPlaygroundServiceServer
}

func (s *server) ExecuteCode(stream pb.PlaygroundService_ExecuteCodeServer) error {
	log.Println("ExecuteCode called")
	input := make(chan string)
	output := make(chan string)

	go func() {
		for {
			in, err := stream.Recv()
			if err != nil {
				log.Printf("Error receiving from stream: %v", err)
				close(input)
				return
			}
			log.Printf("Received input: %s", in.Input)
			input <- in.Input
		}
	}()

	go func() {
		for out := range output {
			if err := stream.Send(&pb.ExecuteResponse{Output: out}); err != nil {
				log.Printf("Error sending to stream: %v", err)
				return
			}
		}
	}()

	req, err := stream.Recv()
	if err != nil {
		log.Printf("Error receiving initial request: %v", err)
		return err
	}

	log.Printf("Received code execution request. Language: %s", req.Language)

	execReq := executor.ExecRequest{
		Language: req.Language,
		Code:     req.Code,
	}

	if err := executor.ExecuteInteractiveCode(stream.Context(), execReq, input, output); err != nil {
		log.Printf("Error executing code: %v", err)
		return err
	}

	log.Println("Code execution completed")
	return nil
}

func main() {
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterPlaygroundServiceServer(s, &server{})
	log.Println("gRPC server listening on :50052")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
