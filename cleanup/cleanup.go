// The `package cleanup` in the provided code is a Go package that contains functions for cleaning up
// Docker resources associated with a specific container ID. The main function `CleanUp` orchestrates
// the cleanup process by calling functions to stop and remove containers, networks, and volumes. Here
// is a breakdown of the key functions in the package:
package cleanup

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// CleanUp performs cleanup tasks for the specified job ID, including stopping and removing containers, networks, and volumes.
//
// Parameters:
// - cli: The Docker client instance.
// - jobID: The job ID associated with the job.
func CleanUp(cli *client.Client, jobID string) {
	log.Println("Starting cleanup...")

	if jobID == "" {
		log.Println("No job ID provided, skipping cleanup.")
		return
	}

	ctx := context.Background()
	removeOptions := container.RemoveOptions{Force: true}
	stopOptions := container.StopOptions{Timeout: new(int)}
	*stopOptions.Timeout = 10

	// Clean up containers
	containerErr := CleanupContainers(cli, ctx, jobID, removeOptions, stopOptions)

	// Clean up networks
	networkErr := CleanupNetworks(cli, ctx, jobID)

	// Clean up volumes
	volumeErr := CleanupVolumes(cli, ctx, jobID)

	// Logs outputs
	if containerErr == nil && networkErr == nil && volumeErr == nil {
		log.Println("No resources found to clean up.")
	} else {
		log.Println("Cleanup completed with errors.")
		if containerErr != nil {
			log.Printf("Container cleanup error: %v", containerErr)
		}
		if networkErr != nil {
			log.Printf("Network cleanup error: %v", networkErr)
		}
		if volumeErr != nil {
			log.Printf("Volume cleanup error: %v", volumeErr)
		}
	}
}

// CleanupContainers stops and removes containers associated with the specified job ID.
//
// Parameters:
// - cli: The Docker client instance.
// - ctx: The context for API calls.
// - jobID: The job ID associated with the job.
// - removeOptions: Options for removing containers.
// - stopOptions: Options for stopping containers.
//
// Returns:
// - error: An error if container cleanup fails.
func CleanupContainers(cli *client.Client, ctx context.Context, jobID string, removeOptions container.RemoveOptions, stopOptions container.StopOptions) error {
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	cleaned := false
	for _, container := range containers {
		if IsJobContainer(container, jobID) || IsComposeContainer(container) {
			cleaned = true
			// Stop the container if it is running
			if container.State == "running" {
				log.Printf("Stopping container %s", container.ID)
				err := cli.ContainerStop(ctx, container.ID, stopOptions)
				if err != nil {
					log.Printf("Failed to stop container %s: %v", container.ID, err)
					continue // Proceed to the next container
				}
			}

			// Remove the container
			log.Printf("Removing container %s", container.ID)
			err := cli.ContainerRemove(ctx, container.ID, removeOptions)
			if err != nil {
				log.Printf("Failed to remove container %s: %v", container.ID, err)
			}
		}
	}

	if !cleaned {
		log.Println("No containers found to clean up.")
		return nil
	}
	return nil
}

// CleanupNetworks removes networks associated with the specified job ID.
//
// Parameters:
// - cli: The Docker client instance.
// - ctx: The context for API calls.
// - jobID: The job ID associated with the job.
//
// Returns:
// - error: An error if network cleanup fails.
func CleanupNetworks(cli *client.Client, ctx context.Context, jobID string) error {
	const maxRetries = 5
	const retryDelay = 2 * time.Second

	for retry := 0; retry < maxRetries; retry++ {
		networks, err := cli.NetworkList(ctx, network.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list networks: %w", err)
		}

		cleaned := false
		for _, network := range networks {
			if strings.Contains(network.Name, jobID) || strings.Contains(network.Name, "docker_default") {
				cleaned = true
				log.Printf("Removing network %s", network.ID)
				if err := cli.NetworkRemove(ctx, network.ID); err != nil {
					log.Printf("Failed to remove network %s: %v", network.ID, err)
				} else {
					log.Printf("Network %s removed successfully.", network.ID)
				}
			}
		}

		if cleaned {
			return nil
		}

		if retry < maxRetries-1 {
			log.Printf("No networks found, retrying in %s...", retryDelay)
			time.Sleep(retryDelay)
		} else {
			log.Println("No networks found to clean up.")
			return nil
		}
	}

	return nil
}

// CleanupVolumes removes volumes associated with the specified job ID.
//
// Parameters:
// - cli: The Docker client instance.
// - ctx: The context for API calls.
// - jobID: The job ID associated with the job.
//
// Returns:
// - error: An error if volume cleanup fails.
func CleanupVolumes(cli *client.Client, ctx context.Context, jobID string) error {
	volumeFilter := filters.NewArgs()
	volumeFilter.Add("name", fmt.Sprintf(".*%s.*", jobID))

	volumes, err := cli.VolumeList(ctx, volume.ListOptions{Filters: volumeFilter})
	if err != nil {
		return fmt.Errorf("failed to list volumes: %w", err)
	}

	cleaned := false
	for _, volume := range volumes.Volumes {
		cleaned = true
		log.Printf("Removing volume %s", volume.Name)
		if err := cli.VolumeRemove(ctx, volume.Name, true); err != nil {
			log.Printf("Failed to remove volume %s: %v", volume.Name, err)
		}
	}

	if !cleaned {
		log.Println("No volumes found to clean up.")
		return nil
	}
	return nil
}

// The function `IsJobContainer` checks if a container is associated with a specific job ID based on
// its labels and names.
func IsJobContainer(container types.Container, jobID string) bool {
	return container.Labels["com.github.ci.job.id"] == jobID || container.Names[0] == fmt.Sprintf("/%s", jobID)
}

// The IsComposeContainer function checks if a Docker container is part of a Compose project based on
// its labels and name.
func IsComposeContainer(container types.Container) bool {
	// Docker Compose containers often have labels like 'com.docker.compose.project'
	return container.Labels["com.docker.compose.project"] != "" || strings.Contains(container.Names[0], "_")
}
