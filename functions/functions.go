package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// DockerClient defines the interface for Docker client methods used in this application.
type DockerClient interface {
	// ContainerInspect retrieves detailed information about a container by its ID.
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)

	// ContainerList lists all containers matching the provided options.
	ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
}

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
		CleanUp(cli, event.ID)
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
func IsJobPattern(cli DockerClient, containerID string, jobPatterns []string) bool {
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

	for _, pattern := range jobPatterns {
		matched, err := regexp.MatchString(pattern, containerJSON.Name)
		if err != nil {
			log.Printf("Failed to match container name %s with pattern %s: %v", containerJSON.Name, pattern, err)
			continue
		}

		if matched {
			log.Printf("Container %s matched job pattern.\n", containerID)
			return true
		}
	}

	return false
}

// cleanUp performs cleanup tasks for the specified container ID, including stopping and removing containers, networks, and volumes.
//
// Parameters:
// - cli: The Docker client instance.
// - containerID: The ID of the container to clean up.
func CleanUp(cli *client.Client, containerID string) {
	log.Println("Starting cleanup...")

	ctx := context.Background()
	removeOptions := container.RemoveOptions{Force: true}
	stopOptions := container.StopOptions{}

	if containerID == "" {
		log.Println("No job ID provided, skipping cleanup.")
		return
	}

	if IsDockerComposeUp(cli) {
		log.Println("Docker Compose is managing containers. Running 'docker-compose down'...")
		err := RunDockerComposeDown()
		if err != nil {
			log.Printf("Failed to run 'docker-compose down': %v", err)
		}
	}

	if err := CleanupContainers(cli, ctx, containerID, removeOptions, stopOptions); err != nil {
		log.Printf("Container cleanup failed: %v", err)
	}

	if err := CleanupNetworks(cli, ctx, containerID); err != nil {
		log.Printf("Network cleanup failed: %v", err)
	}

	if err := CleanupVolumes(cli, ctx, containerID); err != nil {
		log.Printf("Volume cleanup failed: %v", err)
	}

	log.Println("Cleanup completed.")
}

// cleanupContainers stops and removes containers associated with the specified container ID.
//
// Parameters:
// - cli: The Docker client instance.
// - ctx: The context for API calls.
// - containerID: The ID of the container to remove.
// - removeOptions: Options for removing containers.
// - stopOptions: Options for stopping containers.
//
// Returns:
// - error: An error if container cleanup fails.
func CleanupContainers(cli *client.Client, ctx context.Context, containerID string, removeOptions container.RemoveOptions, stopOptions container.StopOptions) error {
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, container := range containers {
		if container.Labels["com.github.ci.job.id"] == containerID {
			if container.State == "running" {
				if err := cli.ContainerStop(ctx, container.ID, stopOptions); err != nil {
					log.Printf("Failed to stop container %s: %v", container.ID, err)
					continue
				}
			}
			if err := cli.ContainerRemove(ctx, container.ID, removeOptions); err != nil {
				log.Printf("Failed to remove container %s: %v", container.ID, err)
			}
		}
	}
	return nil
}

// cleanupNetworks removes networks associated with the specified container ID.
//
// Parameters:
// - cli: The Docker client instance.
// - ctx: The context for API calls.
// - containerID: The ID of the container.
//
// Returns:
// - error: An error if network cleanup fails.
func CleanupNetworks(cli *client.Client, ctx context.Context, containerID string) error {
	networks, err := cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	for _, network := range networks {
		if len(network.Containers) == 0 && network.Labels["com.github.ci.job.id"] == containerID {
			if err := cli.NetworkRemove(ctx, network.ID); err != nil {
				log.Printf("Failed to remove network %s: %v", network.ID, err)
			}
		}
	}
	return nil
}

// cleanupVolumes removes volumes associated with the specified container ID.
//
// Parameters:
// - cli: The Docker client instance.
// - ctx: The context for API calls.
// - containerID: The ID of the container.
//
// Returns:
// - error: An error if volume cleanup fails.
func CleanupVolumes(cli *client.Client, ctx context.Context, containerID string) error {
	volumeFilter := filters.NewArgs()
	volumeFilter.Add("label", fmt.Sprintf("com.github.ci.job.id=%s", containerID))

	volumes, err := cli.VolumeList(ctx, volume.ListOptions{Filters: volumeFilter})
	if err != nil {
		return fmt.Errorf("failed to list volumes: %w", err)
	}

	for _, volume := range volumes.Volumes {
		if volume.Labels["com.github.ci.job.id"] == containerID {
			if err := cli.VolumeRemove(ctx, volume.Name, true); err != nil {
				log.Printf("Failed to remove volume %s: %v", volume.Name, err)
			}
		}
	}
	return nil
}

// isDockerComposeUp checks if Docker Compose is managing any containers.
//
// Parameters:
// - cli: The Docker client instance.
//
// Returns:
// - bool: True if Docker Compose is managing containers; otherwise, false.
func IsDockerComposeUp(cli DockerClient) bool {
	ctx := context.Background()
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		log.Printf("Failed to list containers: %v", err)
		return false
	}

	for _, container := range containers {
		if _, ok := container.Labels["com.docker.compose.project"]; ok {
			return true
		}
	}
	return false
}

// runDockerComposeDown executes the 'docker-compose down' command to stop and remove Docker Compose-managed containers.
//
// Returns:
// - error: An error if the command fails to execute.
func RunDockerComposeDown() error {
	cmd := exec.Command("docker-compose", "down")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
