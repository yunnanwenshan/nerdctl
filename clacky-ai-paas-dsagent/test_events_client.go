package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	eventsv1 "dsagent/api/events/v1"
)

func main() {
	// Connect to the gRPC server
	conn, err := grpc.Dial("localhost:9000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Create events client
	client := eventsv1.NewEventsServiceClient(conn)

	// Test GetSystemEvents (without filters)
	fmt.Println("Testing GetSystemEvents...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := client.GetSystemEvents(ctx, &eventsv1.GetSystemEventsRequest{})
	if err != nil {
		log.Printf("Failed to call GetSystemEvents: %v", err)
	} else {
		fmt.Println("Successfully connected to GetSystemEvents stream")
		
		// Try to receive a few events or timeout
		go func() {
			for i := 0; i < 5; i++ {
				event, err := stream.Recv()
				if err == io.EOF {
					fmt.Println("Stream ended")
					break
				}
				if err != nil {
					log.Printf("Error receiving event: %v", err)
					break
				}
				fmt.Printf("Received event: Type=%s, Status=%s, Topic=%s\n", 
					event.EventType, event.Status, event.Topic)
			}
		}()
		
		// Wait a few seconds to receive events
		time.Sleep(3 * time.Second)
	}

	// Test GetFilteredSystemEvents (with filters)
	fmt.Println("\nTesting GetFilteredSystemEvents...")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	
	filtersReq := &eventsv1.GetFilteredSystemEventsRequest{
		Filters: []string{"type=container"},
	}

	stream2, err := client.GetFilteredSystemEvents(ctx2, filtersReq)
	if err != nil {
		log.Printf("Failed to call GetFilteredSystemEvents: %v", err)
	} else {
		fmt.Println("Successfully connected to GetFilteredSystemEvents stream")
		
		// Try to receive a few events or timeout
		go func() {
			for i := 0; i < 3; i++ {
				event, err := stream2.Recv()
				if err == io.EOF {
					fmt.Println("Filtered stream ended")
					break
				}
				if err != nil {
					log.Printf("Error receiving filtered event: %v", err)
					break
				}
				fmt.Printf("Received filtered event: Type=%s, Status=%s, Topic=%s\n", 
					event.EventType, event.Status, event.Topic)
			}
		}()
		
		// Wait a few seconds to receive events  
		time.Sleep(3 * time.Second)
	}

	fmt.Println("Test completed successfully!")
}