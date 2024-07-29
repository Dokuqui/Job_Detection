package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/docker/client"
	"job-detection.is/github-gitlab/functions"
)

type Config struct {
	JobPatterns []string `json:"jobPattern"`
}

func main() {
	config, err := functions.LoadConfig("/patterns/jobPattern.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	eventCh, errCh := functions.MonitorContainerEvents(cli)

	go func() {
		for {
			select {
			case event := <-eventCh:
				functions.HandleEvent(cli, event, config.JobPatterns)
			case err := <-errCh:
				log.Printf("Error in Docker event monitoring: %v", err)
			}
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	cli.Close()
}
