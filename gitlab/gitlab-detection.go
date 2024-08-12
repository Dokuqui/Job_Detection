/*
Package main provides a command-line application for monitoring and managing
Docker containers based on GitLab CI job patterns.

It performs the following tasks:
- Loads configuration from a JSON file.
- Creates a Docker client to interact with the Docker API.
- Monitors Docker container events (e.g., start and die).
- Handles system signals to gracefully shutdown.
- Cleans up Docker containers, networks, and volumes based on specific job patterns.
*/
// The `package main` declaration in Go indicates that this file is the entry point of the application.
// It defines a command-line application for monitoring and managing Docker containers based on GitLab
// CI job patterns. The main function is the starting point of the program and performs the following
// tasks:
package gitlab

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/docker/client"
)

// main is the entry point of the application. It:
// 1. Loads configuration from "jobPattern.json".
// 2. Creates a Docker client with environment variables.
// 3. Starts monitoring Docker events in a separate goroutine.
// 4. Handles system signals (SIGINT, SIGTERM) for graceful shutdown.
func main() {
	config, err := LoadConfig("../patterns/jobPatterns.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	eventCh, errCh := MonitorContainerEvents(cli)

	go func() {
		for {
			select {
			case event := <-eventCh:
				HandleEvent(cli, event, config.JobPatterns)
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
