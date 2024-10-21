package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/afshin-deriv/playground/pb"
	"github.com/afshin-deriv/playground/pkg/executor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type server struct {
	pb.UnimplementedPlaygroundServiceServer
}

func (s *server) ExecuteCode(stream pb.PlaygroundService_ExecuteCodeServer) error {
	input := make(chan string, 10)
	output := make(chan string, 10)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			in, err := stream.Recv()
			if err != nil {
				slog.Error("receiving from stream: %v", "error", err)
				return
			}
			if in.Code != "" {
				slog.Error("received code execution request. Language: %s", "error", in.Language)
				execReq := executor.ExecRequest{
					Language: in.Language,
					Code:     in.Code,
				}
				go func() {
					if err := executor.ExecuteInteractiveCode(stream.Context(), execReq, input, output); err != nil {
						slog.Error("executing code: %v", "error", err)
						output <- fmt.Sprintf("Error: %v\n", err)
					}
					slog.Debug("code execution completed")
					close(output)
				}()
			} else {
				slog.Debug("received input: %s", "info", in.Input)
				input <- in.Input
			}
		}
	}()

	for {
		select {
		case out, ok := <-output:
			if !ok {
				return nil
			}
			if err := stream.Send(&pb.ExecuteResponse{Output: out}); err != nil {
				slog.Error("sending to stream: %v", "error", err)
				return err
			}
		case <-done:
			slog.Debug("stream closed by client")
			return nil
		case <-stream.Context().Done():
			slog.Debug("context canceled")
			return stream.Context().Err()
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		slog.Error("failed to listen: %v", "error", err)
	}

	s := grpc.NewServer()
	pb.RegisterPlaygroundServiceServer(s, &server{})

	// Enable reflection for gRPC CLI debugging
	reflection.Register(s)

	// Set up graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		slog.Info("shutting down gRPC server...")
		s.GracefulStop()
	}()

	slog.Info("gRPC server listening on :50052")
	if err := s.Serve(lis); err != nil {
		slog.Error("Failed to serve: %v", "error", err)
	}
}
