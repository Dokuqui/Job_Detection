// The `package events` is defining a Go package that contains functions and types related to
// monitoring Docker container events and handling them based on specified job patterns. The package
// includes functions for loading configuration from a JSON file, monitoring Docker container events,
// handling events by logging messages and initiating cleanup, and checking if a container name matches
// specified job patterns using regular expressions.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"job-detection.is/github-gitlab/cleanup"
)

// Config holds the configuration for job patterns.
type Config struct {
	// JobPatterns contains regular expressions to match container names.
	JobPatterns []string `json:"jobPattern"`
}

// loadConfig loads the configuration from a JSON file.
//
// Parameters:
// - filename: The path to the JSON configuration file.
//
// Returns:
// - *Config: The configuration structure.
// - error: An error if the file could not be opened or the JSON could not be decoded.
func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// monitorContainerEvents sets up Docker event monitoring and returns channels for events and errors.
//
// Parameters:
// - cli: The Docker client instance.
//
// Returns:
// - <-chan events.Message: Channel for Docker container events.
// - <-chan error: Channel for errors occurring while monitoring Docker events.
func MonitorContainerEvents(cli *client.Client) (<-chan events.Message, <-chan error) {
	eventCh := make(chan events.Message)
	errCh := make(chan error)

	go func() {
		defer close(eventCh)
		defer close(errCh)

		ctx := context.Background()

		args := filters.NewArgs()
		args.Add("type", "container")
		options := events.ListOptions{
			Filters: args,
		}

		eventChan, eventErrChan := cli.Events(ctx, options)

		for {
			select {
			case event := <-eventChan:
				eventCh <- event
			case err := <-eventErrChan:
				errCh <- fmt.Errorf("error while receiving Docker events: %v", err)
				return
			}
		}
	}()

	return eventCh, errCh
}

// handleEvent processes Docker container events and performs actions based on the event type.
//
// Parameters:
// - cli: The Docker client instance.
// - event: The Docker container event to handle.
// - jobPatterns: List of job patterns to match against container names.
//
// Actions:
// - Logs messages when containers start or stop.
// - Initiates cleanup when containers stop.
func HandleEvent(cli *client.Client, event events.Message, jobPatterns []string) {
	if !IsJobPattern(cli, event.ID, jobPatterns) {
		return
	}

	switch event.Action {
	case "start":
		log.Printf("GitLab job container %s started.\n", event.ID)
	case "die":
		log.Printf("GitLab job container %s finished.\n", event.ID)
		cleanup.CleanUp(cli, event.ID)
	}
}

// isJobPattern checks if the container name matches any of the specified job patterns.
//
// Parameters:
// - cli: The Docker client instance.
// - containerID: The ID of the container to inspect.
// - jobPatterns: List of job patterns to match against container names.
//
// Returns:
// - bool: True if the container name matches any pattern; otherwise, false.
func IsJobPattern(cli *client.Client, containerID string, jobPatterns []string) bool {
	ctx := context.Background()
	containerJSON, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if client.IsErrNotFound(err) {
			log.Printf("Container %s not found.", containerID)
			return false
		}
		log.Printf("Failed to inspect container %s: %v", containerID, err)
		return false
	}

	return MatchContainerName(containerJSON.Name, jobPatterns)
}

// The MatchContainerName function checks if a container name matches any of the provided job patterns
// using regular expressions.
func MatchContainerName(containerName string, jobPatterns []string) bool {
	for _, pattern := range jobPatterns {
		matched, err := regexp.MatchString(pattern, containerName)
		if err != nil {
			log.Printf("Failed to match container name %s with pattern %s: %v", containerName, pattern, err)
			continue
		}

		if matched {
			log.Printf("Container %s matched job pattern.\n", containerName)
			return true
		}
	}
	return false
}
