// The `package cleanup` in the provided code is a Go package that contains functions for cleaning up
// Docker resources associated with a specific container ID. The main function `CleanUp` orchestrates
// the cleanup process by calling functions to stop and remove containers, networks, and volumes. Here
// is a breakdown of the key functions in the package:
package cleanup

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// CleanUp performs cleanup tasks for the specified container ID, including stopping and removing containers, networks, and volumes.
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

// CleanupContainers stops and removes containers associated with the specified container ID.
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

// CleanupNetworks removes networks associated with the specified container ID.
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

// CleanupVolumes removes volumes associated with the specified container ID.
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

// IsDockerComposeUp checks if Docker Compose is managing any containers.
//
// Parameters:
// - cli: The Docker client instance.
//
// Returns:
// - bool: True if Docker Compose is managing containers; otherwise, false.
func IsDockerComposeUp(cli *client.Client) bool {
	ctx := context.Background()
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		log.Printf("Failed to list containers: %v", err)
		return false
	}

	return IsDockerComposeManaged(containers)
}

// The function `IsDockerComposeManaged` checks if any container in a list has the label
// "com.docker.compose.project" indicating it is managed by Docker Compose.
func IsDockerComposeManaged(containers []types.Container) bool {
	for _, container := range containers {
		if _, ok := container.Labels["com.docker.compose.project"]; ok {
			return true
		}
	}
	return false
}

// RunDockerComposeDown executes the 'docker-compose down' command to stop and remove Docker Compose-managed containers.
//
// Returns:
// - error: An error if the command fails to execute.
func RunDockerComposeDown() error {
	cmd := exec.Command("docker-compose", "down")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
