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

func TestIsJobContainer(t *testing.T) {
	testCases := []struct {
		name      string
		container types.Container
		jobID     string
		expected  bool
	}{
		{
			name: "Container with matching job ID label",
			container: types.Container{
				Labels: map[string]string{
					"com.github.ci.job.id": "1234",
				},
				Names: []string{"/other"},
			},
			jobID:    "1234",
			expected: true,
		},
		{
			name: "Container with matching job ID name",
			container: types.Container{
				Labels: map[string]string{},
				Names:  []string{"/1234"},
			},
			jobID:    "1234",
			expected: true,
		},
		{
			name: "Container with non-matching job ID label and name",
			container: types.Container{
				Labels: map[string]string{
					"com.github.ci.job.id": "5678",
				},
				Names: []string{"/5678"},
			},
			jobID:    "1234",
			expected: false,
		},
		{
			name: "Container with no job ID label and non-matching name",
			container: types.Container{
				Labels: map[string]string{},
				Names:  []string{"/5678"},
			},
			jobID:    "1234",
			expected: false,
		},
		{
			name: "Container with no job ID label and name is empty string",
			container: types.Container{
				Labels: map[string]string{},
				Names:  []string{""},
			},
			jobID:    "1234",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsJobContainer(tc.container, tc.jobID)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsComposeContainer(t *testing.T) {
	testCases := []struct {
		name      string
		container types.Container
		jobID     string
		expected  bool
	}{
		{
			name: "Container with Docker Compose project label",
			container: types.Container{
				Labels: map[string]string{
					"com.docker.compose.project": "example",
				},
				Names: []string{"/other"},
			},
			jobID:    "1234",
			expected: true,
		},
		{
			name: "Container with Compose-related name",
			container: types.Container{
				Labels: map[string]string{},
				Names:  []string{"/example_app_1"},
			},
			jobID:    "1234",
			expected: true,
		},
		{
			name: "Container without Compose project label and name without underscore",
			container: types.Container{
				Labels: map[string]string{},
				Names:  []string{"/other"},
			},
			jobID:    "1234",
			expected: false,
		},
		{
			name: "Container with name as empty string and no Compose project label",
			container: types.Container{
				Labels: map[string]string{},
				Names:  []string{""},
			},
			jobID:    "1234",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsComposeContainer(tc.container, tc.jobID)
			assert.Equal(t, tc.expected, result)
		})
	}
}
