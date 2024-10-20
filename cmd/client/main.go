package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/afshin-deriv/playground/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	log.Println("Starting gRPC client")
	conn, err := grpc.Dial("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), grpc.WithTimeout(5*time.Second))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	log.Println("Connected to gRPC server")

	client := pb.NewPlaygroundServiceClient(conn)

	stream, err := client.ExecuteCode(context.Background())
	if err != nil {
		log.Fatalf("Error creating stream: %v", err)
	}
	log.Println("Stream created successfully")

	// Send initial code
	fmt.Println("Enter the language:")
	language := readLine()
	fmt.Println("Enter the code:")
	code := readMultiLine()

	log.Printf("Sending initial code. Language: %s", language)
	if err := stream.Send(&pb.ExecuteRequest{
		Language: language,
		Code:     code,
	}); err != nil {
		log.Fatalf("Error sending initial code: %v", err)
	}

	// Handle input/output
	go func() {
		fmt.Println("Enter input (or 'exit' to quit):")
		for {
			input := readLine()
			if input == "exit" {
				log.Println("Closing send stream")
				stream.CloseSend()
				return
			}
			log.Printf("Sending input: %s", input)
			if err := stream.Send(&pb.ExecuteRequest{Input: input}); err != nil {
				log.Printf("Error sending input: %v", err)
				return
			}
		}
	}()

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			log.Println("Stream closed by server")
			break
		}
		if err != nil {
			log.Fatalf("Error receiving response: %v", err)
		}
		log.Printf("Received output: %s", resp.Output)
		fmt.Print(resp.Output)
	}
}

func readLine() string {
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return line[:len(line)-1]
}

func readMultiLine() string {
	var lines string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		if text == "" {
			break
		}
		lines += text + "\n"
	}
	return lines
}
