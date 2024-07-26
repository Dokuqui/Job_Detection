package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "os/exec"
    "os/signal"
    "regexp"
    "syscall"

    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/events"
    "github.com/docker/docker/api/types/filters"
    "github.com/docker/docker/api/types/network"
    "github.com/docker/docker/api/types/volume"
    "github.com/docker/docker/client"
)

type Config struct {
    JobPatterns []string `json:"jobPattern"`
}

func main() {
    config, err := loadConfig("/patterns/jobPattern.json")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        log.Fatalf("Failed to create Docker client: %v", err)
    }

    eventCh, errCh := monitorContainerEvents(cli)

    go func() {
        for {
            select {
            case event := <-eventCh:
                handleEvent(cli, event, config.JobPatterns)
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

func loadConfig(filename string) (*Config, error) {
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

func monitorContainerEvents(cli *client.Client) (<-chan events.Message, <-chan error) {
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

func handleEvent(cli *client.Client, event events.Message, jobPatterns []string) {
    if !isJobPatternContainer(cli, event.ID, jobPatterns) {
        return
    }

    switch event.Action {
    case "start":
        log.Printf("GitLab job container %s started.\n", event.ID)
    case "die":
        log.Printf("GitLab job container %s finished.\n", event.ID)
        cleanUp(cli, event.ID)
    }
}

func isJobPatternContainer(cli *client.Client, containerID string, jobPatterns []string) bool {
    ctx := context.Background()
    containerJSON, err := cli.ContainerInspect(ctx, containerID)
    if err != nil {
        log.Printf("Failed to inspect container %s: %v", containerID, err)
        return false
    }

    for _, pattern := range jobPatterns {
        matched, err := regexp.MatchString(pattern, containerJSON.Name)
        if err != nil {
            log.Printf("Failed to match container name %s: %v", containerJSON.Name, err)
            continue
        }

        if matched {
            log.Printf("Container %s matched job pattern.\n", containerID)
            return true
        }
    }

    return false
}

func cleanUp(cli *client.Client, containerID string) {
    log.Println("Starting cleanup...")

    ctx := context.Background()
    removeOptions := container.RemoveOptions{Force: true}
    stopOptions := container.StopOptions{}

    if containerID == "" {
        log.Println("No job ID provided, skipping cleanup.")
        return
    }

    if isDockerComposeUp(cli) {
        log.Println("Docker Compose is managing containers. Running 'docker-compose down'...")
        err := runDockerComposeDown()
        if err != nil {
            log.Printf("Failed to run 'docker-compose down': %v", err)
        }
    }

    if err := cleanupContainers(cli, ctx, containerID, removeOptions, stopOptions); err != nil {
        log.Printf("Cleanup failed: %v", err)
    }

    if err := cleanupNetworks(cli, ctx, containerID); err != nil {
        log.Printf("Network cleanup failed: %v", err)
    }

    if err := cleanupVolumes(cli, ctx, containerID); err != nil {
        log.Printf("Volume cleanup failed: %v", err)
    }

    log.Println("Cleanup completed.")
}

func cleanupContainers(cli *client.Client, ctx context.Context, containerID string, removeOptions container.RemoveOptions, stopOptions container.StopOptions) error {
    containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
    if err != nil {
        return fmt.Errorf("failed to list containers: %w", err)
    }

    for _, container := range containers {
        if container.Labels["com.gitlab.ci.job.id"] == containerID {
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

func cleanupNetworks(cli *client.Client, ctx context.Context, containerID string) error {
    networks, err := cli.NetworkList(ctx, network.ListOptions{})
    if err != nil {
        return fmt.Errorf("failed to list networks: %w", err)
    }

    for _, network := range networks {
        if len(network.Containers) == 0 && network.Labels["com.gitlab.ci.job.id"] == containerID {
            if err := cli.NetworkRemove(ctx, network.ID); err != nil {
                log.Printf("Failed to remove network %s: %v", network.ID, err)
            }
        }
    }

    return nil
}

func cleanupVolumes(cli *client.Client, ctx context.Context, containerID string) error {
    volumeFilter := filters.NewArgs()
    volumeFilter.Add("label", fmt.Sprintf("com.gitlab.ci.job.id=%s", containerID))

    volumes, err := cli.VolumeList(ctx, volume.ListOptions{Filters: volumeFilter})
    if err != nil {
        return fmt.Errorf("failed to list volumes: %w", err)
    }

    for _, volume := range volumes.Volumes {
        if volume.Labels["com.gitlab.ci.job.id"] == containerID {
            if err := cli.VolumeRemove(ctx, volume.Name, true); err != nil {
                log.Printf("Failed to remove volume %s: %v", volume.Name, err)
            }
        }
    }

    return nil
}

func isDockerComposeUp(cli *client.Client) bool {
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

func runDockerComposeDown() error {
    cmd := exec.Command("docker-compose", "down")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    ciJobID := os.Getenv("CI_JOB_ID")
    log.Printf("CI_JOB_ID during docker-compose down: %s", ciJobID)

    cmd.Env = append(os.Environ(), fmt.Sprintf("CI_JOB_ID=%s", ciJobID))
    return cmd.Run()
}
