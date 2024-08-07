# Job_Detection

## Overview

`Job_detection` is a Go-based tool designed to manage and clean up GitLab CI and GitHub Workflows after job is finished. It monitors Docker events to detect when relevant containers start or stop and performs thorough cleanup operations to ensure no residual resources remain.

## Features

- Monitors Docker events for container lifecycle changes.
- Identifies and matches containers based on configurable job patterns.
- Stops and removes containers, networks, and volumes associated with completed GitLab CI jobs.
- Supports Docker Compose setups by running `docker-compose down` for multi-container applications.
- Handles graceful shutdowns to ensure cleanup is performed even when the script terminates unexpectedly.

## How to Run

### Prerequisites

- Go 1.21 or later
- Docker installed and running
- GitLab Runner installed and configured

### Steps

1. **Clone the Repository:**
    ```sh
    git clone git@github.com:Dokuqui/Job_Detection.git
    cd Job_Detection
    ```

2. **Configure Job Patterns:**
    Create or modify the `jobPattern.json` file to specify patterns for identifying relevant containers.
    ```json
    {
      "jobPatterns": [
        "^/runner-.*-project-.*-concurrent-.*-.*-build$",
        "^/runner-.*-project-.*-concurrent-.*-.*-test$",
        "^/runner-.*-project-.*-concurrent-.*-.*-deploy$"
      ]
    }
    ```

3. **Run the Script:**
    ```sh
    go run github-detection.go `or` go run gitlab-detection.go
    ```

## Gitlab Configuration to run

For detailed documentation for gitlab, please visit our [Readme page](../Job_Detection/docs/gitlab-conf.md).

## Tests
Also in each package exists dedicated test for several functions. To run them you can run in your terminal command.

**Run all test:**
```sh
go test -v ./...
```
