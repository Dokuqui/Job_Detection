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
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
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

	// Clean up services
	serviceErr := CleanupServices(cli, ctx, jobID)

	// Logs outputs
	if containerErr == nil && networkErr == nil && volumeErr == nil && serviceErr == nil {
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
		if serviceErr != nil {
			log.Printf("Service cleanup error: %v", serviceErr)
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
	const maxRetries = 3
	const pollInterval = 3 * time.Second
	const cleanupDelay = 10 * time.Second

	// Wait for a while to ensure the job's after_script section has completed
	log.Printf("Waiting for %v before starting cleanup...", cleanupDelay)
	time.Sleep(cleanupDelay)

	for retry := 0; retry < maxRetries; retry++ {
		containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		var activeContainers []types.Container
		cleaned := false

		for _, container := range containers {
			if IsJobContainer(container, jobID) || IsComposeContainer(container, jobID) {
				log.Printf("Checking container %s (State: %s)", container.ID, container.State)

				if container.State == "running" {
					activeContainers = append(activeContainers, container)
				} else if container.State == "exited" {
					log.Printf("Container %s has exited. Proceeding to remove.", container.ID)
					cleaned = true
					if err := cli.ContainerRemove(ctx, container.ID, removeOptions); err != nil {
						log.Printf("Failed to remove container %s: %v", container.ID, err)
					}
				}
			}
		}

		if len(activeContainers) > 0 {
			log.Printf("Detected active containers for job %s or related Compose containers. Waiting for them to stop...", jobID)
			for _, container := range activeContainers {
				log.Printf("Stopping container %s", container.ID)
				if err := cli.ContainerStop(ctx, container.ID, stopOptions); err != nil {
					log.Printf("Failed to stop container %s: %v", container.ID, err)
				}
			}
			time.Sleep(pollInterval)
		} else {
			if cleaned {
				log.Println("Cleanup completed for stopped containers.")
			} else {
				log.Println("No containers found to clean up.")
			}
			break
		}
	}

	// Check if there are still active containers after all retries
	activeContainersAfterCleanup := false
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers after cleanup: %w", err)
	}

	for _, container := range containers {
		if IsJobContainer(container, jobID) || IsComposeContainer(container, jobID) {
			if container.State == "running" || container.State == "exited" {
				activeContainersAfterCleanup = true
				break
			}
		}
	}

	if activeContainersAfterCleanup {
		log.Printf("Failed to clean up all containers related to job %s or Docker Compose.", jobID)
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
	const maxRetries = 2
	const retryDelay = 2 * time.Second

	// Create a set to keep track of networks associated with the containers
	associatedNetworks := make(map[string]struct{})

	for retry := 0; retry < maxRetries; retry++ {
		containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		// Collect networks associated with containers related to the jobID
		for _, container := range containers {
			if IsJobContainer(container, jobID) || IsComposeContainer(container, jobID) {
				for networkName := range container.NetworkSettings.Networks {
					associatedNetworks[networkName] = struct{}{}
				}
			}
		}

		networks, err := cli.NetworkList(ctx, network.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list networks: %w", err)
		}

		cleaned := false
		for _, network := range networks {
			// Check if the network is in the list of associated networks
			if _, exists := associatedNetworks[network.Name]; !exists {
				// Network is not associated with any relevant container
				log.Printf("Removing unused network %s", network.ID)
				if err := cli.NetworkRemove(ctx, network.ID); err != nil {
					log.Printf("Failed to remove network %s: %v", network.ID, err)
				} else {
					log.Printf("Network %s removed successfully.", network.ID)
					cleaned = true
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
	const maxRetries = 2
	const retryDelay = 2 * time.Second

	// Create a set to keep track of volumes associated with the containers
	associatedVolumes := make(map[string]struct{})

	for retry := 0; retry < maxRetries; retry++ {
		containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		// Collect volumes associated with containers related to the jobID
		for _, container := range containers {
			if IsJobContainer(container, jobID) || IsComposeContainer(container, jobID) {
				for _, mount := range container.Mounts {
					// Add volume name to the map
					if mount.Name != "" {
						associatedVolumes[mount.Name] = struct{}{}
					}
				}
			}
		}

		volumes, err := cli.VolumeList(ctx, volume.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list volumes: %w", err)
		}

		cleaned := false
		for _, volume := range volumes.Volumes {
			// Check if the volume is in the list of associated volumes
			if _, exists := associatedVolumes[volume.Name]; !exists {
				// Volume is not associated with any relevant container
				log.Printf("Removing unused volume %s", volume.Name)
				if err := cli.VolumeRemove(ctx, volume.Name, true); err != nil {
					log.Printf("Failed to remove volume %s: %v", volume.Name, err)
				} else {
					log.Printf("Volume %s removed successfully.", volume.Name)
					cleaned = true
				}
			}
		}

		if cleaned {
			log.Println("Cleanup completed for volumes.")
			return nil
		}

		if retry < maxRetries-1 {
			log.Printf("No volumes found, retrying in %s...", retryDelay)
			time.Sleep(retryDelay)
		} else {
			log.Println("No volumes found to clean up.")
			return nil
		}
	}

	return nil
}

// CleanupServices stops and removes services associated with the specified job ID.
//
// Parameters:
// - cli: The Docker client instance.
// - ctx: The context for API calls.
// - jobID: The job ID associated with the job.
//
// Returns:
// - error: An error if service cleanup fails.
func CleanupServices(cli *client.Client, ctx context.Context, jobID string) error {
	const maxRetries = 3
	const retryDelay = 3 * time.Second

	for retry := 0; retry < maxRetries; retry++ {
		services, err := cli.ServiceList(ctx, types.ServiceListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list services: %w", err)
		}

		cleaned := false

		for _, service := range services {
			if IsJobService(service, jobID) {
				log.Printf("Stopping and removing service %s (ID: %s)", service.Spec.Name, service.ID)

				// Remove the service
				if err := cli.ServiceRemove(ctx, service.ID); err != nil {
					log.Printf("Failed to remove service %s: %v", service.Spec.Name, err)
					return err
				} else {
					log.Printf("Service %s removed successfully.", service.Spec.Name)
					cleaned = true
				}
			}
		}

		if cleaned {
			log.Println("Service cleanup completed.")
			return nil
		}

		if retry < maxRetries-1 {
			log.Printf("No services found, retrying in %s...", retryDelay)
			time.Sleep(retryDelay)
		} else {
			log.Println("No services found to clean up.")
			return nil
		}
	}

	return nil
}

// The function `IsJobContainer` checks if a container is associated with a specific job ID based on
// its labels and names.
func IsJobContainer(container types.Container, jobID string) bool {
	// Check labels for job ID
	if container.Labels["com.github.ci.job.id"] == jobID {
		log.Printf("Detected job container %s based on label.", container.ID)
		return true
	}

	// Check if the container name matches the jobID
	containerName := strings.TrimPrefix(container.Names[0], "/")
	if containerName == jobID {
		log.Printf("Detected job container %s based on name.", container.ID)
		return true
	}

	if strings.Contains(containerName, jobID) {
		log.Printf("Detected job container %s based on name containing jobID.", container.ID)
		return true
	}

	// Check if the container was created within the expected time frame for the job
	createdTime := time.Unix(container.Created, 0)
	if time.Since(createdTime) < 24*time.Hour {
		log.Printf("Container %s created recently and might be related to job %s.", container.ID, jobID)
		return true
	}

	return false
}

// The IsComposeContainer function checks if a Docker container is part of a Compose project based on
// its labels and name.
func IsComposeContainer(container types.Container, jobID string) bool {
	// Check for Docker Compose project label
	if projectLabel, exists := container.Labels["com.docker.compose.project"]; exists && projectLabel != "" {
		log.Printf("Detected Docker Compose container %s related to project %s.", container.ID, projectLabel)
		return true
	}

	// Check for service or project name patterns in container names
	if strings.Contains(container.Names[0], jobID) {
		log.Printf("Detected container %s related to jobID %s.", container.ID, jobID)
		return true
	}

	// Additional checks for Docker Compose networks or services
	if strings.Contains(container.Names[0], "docker_default") ||
		strings.Contains(container.Names[0], "_") ||
		container.Labels["com.docker.compose.service"] != "" {
		log.Printf("Detected container %s possibly related to Docker Compose.", container.ID)
		return true
	}

	return false
}

// IsJobService checks if a service is associated with the specified job ID.
//
// Parameters:
// - service: The Docker service object.
// - jobID: The job ID associated with the job.
//
// Returns:
// - bool: True if the service is associated with the job ID.
func IsJobService(service swarm.Service, jobID string) bool {
	// Check labels for job ID
	if service.Spec.Labels["com.github.ci.job.id"] == jobID {
		log.Printf("Detected service %s based on label.", service.Spec.Name)
		return true
	}

	// Check if the service name matches the jobID
	if strings.Contains(service.Spec.Name, jobID) {
		log.Printf("Detected service %s based on name containing jobID.", service.Spec.Name)
		return true
	}

	return false
}
