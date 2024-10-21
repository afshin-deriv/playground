package main

import (
	"fmt"
	"log"
	"net"

	"github.com/afshin-deriv/playground/pb"
	"github.com/afshin-deriv/playground/pkg/executor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type server struct {
	pb.UnimplementedPlaygroundServiceServer
}

func (s *server) ExecuteCode(stream pb.PlaygroundService_ExecuteCodeServer) error {
	log.Println("ExecuteCode called")
	input := make(chan string)
	output := make(chan string)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			in, err := stream.Recv()
			if err != nil {
				log.Printf("Error receiving from stream: %v", err)
				return
			}
			if in.Code != "" {
				// New code execution request
				log.Printf("Received code execution request. Language: %s", in.Language)
				execReq := executor.ExecRequest{
					Language: in.Language,
					Code:     in.Code,
				}
				go func() {
					if err := executor.ExecuteInteractiveCode(stream.Context(), execReq, input, output); err != nil {
						log.Printf("Error executing code: %v", err)
						output <- fmt.Sprintf("Error: %v\n", err)
					}
					log.Println("Code execution completed")
					close(output)
				}()
			} else {
				// Regular input
				log.Printf("Received input: %s", in.Input)
				input <- in.Input
			}
		}
	}()

	for {
		select {
		case out, ok := <-output:
			if !ok {
				// output channel was closed, which means execution has finished
				return nil
			}
			if err := stream.Send(&pb.ExecuteResponse{Output: out}); err != nil {
				log.Printf("Error sending to stream: %v", err)
				return err
			}
		case <-done:
			log.Println("Stream closed by client")
			return nil
		case <-stream.Context().Done():
			log.Println("Context canceled")
			return stream.Context().Err()
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterPlaygroundServiceServer(s, &server{})

	// Enable reflection
	reflection.Register(s)

	log.Println("gRPC server listening on :50052")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
