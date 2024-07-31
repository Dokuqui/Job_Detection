// The `package cleanup` is likely a Go package that contains functions related to cleaning up
// resources or managing cleanup operations. In the provided code snippet, there is a test function
// `TestIsDockerContainerManaged` that tests whether a Docker container is managed by Docker Compose
// based on its labels. The test cases define different scenarios with containers having specific
// labels, and the test function checks if the expected result matches the actual result of the
// `IsDockerComposeManaged` function.
package cleanup

import (
	"testing"

	"github.com/docker/docker/api/types"
	"gotest.tools/v3/assert"
)

// The function `TestIsDockerContainerManaged` tests whether a Docker container is managed by Docker
// Compose based on its labels.
func TestIsDockerContainerManaged(t *testing.T) {
	testCases := []struct {
		name       string
		containers []types.Container
		expected   bool
	}{
		{
			name: "Docker Compose managed container",
			containers: []types.Container{
				{
					Labels: map[string]string{
						"com.docker.compose.project": "example",
					},
				},
			},
			expected: true,
		},
		{
			name: "No Docker Compose managed containers",
			containers: []types.Container{
				{
					Labels: map[string]string{},
				},
			},
			expected: false,
		},
		{
			name:       "Empty container list",
			containers: []types.Container{},
			expected:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsDockerComposeManaged(tc.containers)
			assert.Equal(t, tc.expected, result)
		})
	}
}
