// The `package main` declaration at the beginning of a Go source file indicates that the file is part
// of the main package. In Go, the `main` package is special because it defines a standalone executable
// program. When you create a Go program that you intend to run as an executable, you typically place
// the `main` function in a file within the `main` package.
package main_test

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"job-detection.is/github-gitlab/functions"
)

// MockDockerClient is a mock implementation of the Docker client.
// It uses testify's mock package to define the methods and their expected behavior.
// The MockDockerClient type is a struct that includes a mock object for testing purposes.
// @property  - The `MockDockerClient` struct is likely used for mocking Docker client behavior in unit
// tests. It embeds a `mock.Mock` field, which suggests that it is using a mocking library like
// `github.com/stretchr/testify/mock` for creating mock objects. This struct can be used to mock
// methods
type MockDockerClient struct {
	mock.Mock
}

// ContainerInspect mocks the ContainerInspect method of the Docker client.
// It returns a types.ContainerJSON object and an error based on the defined expectations.
// The `func (m *MockDockerClient) ContainerInspect(ctx context.Context, containerID string)
// (types.ContainerJSON, error)` method in the `MockDockerClient` struct is defining a mock
// implementation for the `ContainerInspect` method of the Docker client. This method is used for
// testing purposes to simulate the behavior of the actual `ContainerInspect` method from the Docker
// client.
func (m *MockDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	args := m.Called(ctx, containerID)
	return args.Get(0).(types.ContainerJSON), args.Error(1)
}

// ContainerList mocks the ContainerList method of the Docker client.
// It returns a slice of types.Container and an error based on the defined expectations.
// The `func (m *MockDockerClient) ContainerList(ctx context.Context, options container.ListOptions)
// ([]types.Container, error)` method in the `MockDockerClient` struct is defining a mock
// implementation for the `ContainerList` method of the Docker client. This method is used for testing
// purposes to simulate the behavior of the actual `ContainerList` method from the Docker client.
func (m *MockDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	args := m.Called(ctx, options)
	return args.Get(0).([]types.Container), args.Error(1)
}

// TestLoadConfig tests the LoadConfig function.
// It checks if the configuration is loaded correctly from a JSON file and matches the expected structure.
// The TestLoadConfig function tests the LoadConfig function by comparing the loaded configuration with
// an expected configuration.
func TestLoadConfig(t *testing.T) {
	expectedConfig := &functions.Config{
		JobPatterns: []string{
			"^/runner-.*-project-.*-concurrent-.*-.*-build$",
			"^/runner-.*-project-.*-concurrent-.*-.*-test$",
			"^/runner-.*-project-.*-concurrent-.*-.*-deploy$",
		},
	}

	config, err := functions.LoadConfig("../patterns/jobPattern.json")
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, config)
}

// TestIsJobPattern tests the IsJobPattern function.
// It verifies that the function correctly identifies if a container name matches any of the specified job patterns.
// The function `TestIsJobPattern` tests whether a given container name matches any of the specified
// job patterns.
func TestIsJobPattern(t *testing.T) {
	mockDockerClient := new(MockDockerClient)

	containerID := "test-container-id"
	containerName := "/runner-123-project-456-concurrent-789-0-build"
	jobPatterns := []string{
		"^/runner-.*-project-.*-concurrent-.*-.*-build$",
		"^/runner-.*-project-.*-concurrent-.*-.*-test$",
		"^/runner-.*-project-.*-concurrent-.*-.*-deploy$",
	}

	mockDockerClient.On("ContainerInspect", mock.Anything, containerID).Return(
		types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				Name: containerName,
			},
		}, nil,
	)

	result := functions.IsJobPattern(mockDockerClient, containerID, jobPatterns)
	assert.True(t, result)
	mockDockerClient.AssertExpectations(t)
}

// TestIsDockerComposeUp tests the IsDockerComposeUp function.
// It verifies that the function correctly identifies if Docker Compose is managing any containers.
// The function `TestIsDockerComposeUp` tests whether Docker Compose is up using a mock Docker client.
func TestIsDockerComposeUp(t *testing.T) {
	mockDockerClient := new(MockDockerClient)

	mockDockerClient.On("ContainerList", mock.Anything, mock.Anything).Return(
		[]types.Container{
			{
				Labels: map[string]string{
					"com.docker.compose.project": "example",
				},
			},
		}, nil,
	)

	result := functions.IsDockerComposeUp(mockDockerClient)
	assert.True(t, result)
	mockDockerClient.AssertExpectations(t)
}
