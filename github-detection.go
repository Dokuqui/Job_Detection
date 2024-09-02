package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/docker/client"
	"job-detection.is/github-gitlab/events"
)

type Config struct {
	JobPatterns []string `json:"jobPattern"`
}

// main is the entry point of the application. It:
// 1. Loads configuration from "jobPattern.json".
// 2. Creates a Docker client with environment variables.
// 3. Starts monitoring Docker events in a separate goroutine.
// 4. Handles system signals (SIGINT, SIGTERM) for graceful shutdown.
func main() {
	config, err := events.LoadConfig("../patterns/jobPattern.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	eventCh, errCh := events.MonitorContainerEvents(cli)

	go func() {
		for {
			select {
			case event := <-eventCh:
				events.HandleEvent(cli, event, config.JobPatterns)
			case err := <-errCh:
				log.Printf("Error in Docker event monitoring: %v", err)
			}
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	if err := cli.Close(); err != nil {
		log.Printf("Error closing Docker client: %v", err)
	}
}
