// The `package events` statement at the beginning of a Go file indicates that the code in that file
// belongs to the `events` package. In Go, packages are used to organize and group related code
// together. The `package events` statement specifies that the code in this file is part of the
// `events` package, and other files within the same directory can import and use the code defined in
// this package.
package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLoadConfig tests the LoadConfig function.
// It checks if the configuration is loaded correctly from a JSON file and matches the expected structure.
// The TestLoadConfig function tests the LoadConfig function by comparing the loaded configuration with
// an expected configuration.
func TestLoadConfig(t *testing.T) {
	expectedConfig := &Config{
		JobPatterns: []string{
			"^/runner-.*-project-.*-concurrent-.*-.*-build$",
			"^/runner-.*-project-.*-concurrent-.*-.*-test$",
			"^/runner-.*-project-.*-concurrent-.*-.*-deploy$",
		},
	}

	config, err := LoadConfig("../patterns/jobPattern.json")
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, config)
}

// TestIsJobPattern tests the IsJobPattern function.
// It verifies that the function correctly identifies if a container name matches any of the specified job patterns.
// The function `TestIsJobPattern` tests whether a given container name matches any of the specified
// job patterns.
func TestMatchContainerName(t *testing.T) {
	testCases := []struct {
		name          string
		containerName string
		jobPatterns   []string
		expected      bool
	}{
		{
			name:          "Match with build pattern",
			containerName: "/runner-123-project-456-concurrent-789-0-build",
			jobPatterns: []string{
				"^/runner-.*-project-.*-concurrent-.*-.*-build$",
				"^/runner-.*-project-.*-concurrent-.*-.*-test$",
				"^/runner-.*-project-.*-concurrent-.*-.*-deploy$",
			},
			expected: true,
		},
		{
			name:          "Match with test pattern",
			containerName: "/runner-123-project-456-concurrent-789-0-test",
			jobPatterns: []string{
				"^/runner-.*-project-.*-concurrent-.*-.*-build$",
				"^/runner-.*-project-.*-concurrent-.*-.*-test$",
				"^/runner-.*-project-.*-concurrent-.*-.*-deploy$",
			},
			expected: true,
		},
		{
			name:          "No match",
			containerName: "/runner-123-project-456-concurrent-789-0-other",
			jobPatterns: []string{
				"^/runner-.*-project-.*-concurrent-.*-.*-build$",
				"^/runner-.*-project-.*-concurrent-.*-.*-test$",
				"^/runner-.*-project-.*-concurrent-.*-.*-deploy$",
			},
			expected: false,
		},
		{
			name:          "Empty patterns",
			containerName: "/runner-123-project-456-concurrent-789-0-build",
			jobPatterns:   []string{},
			expected:      false,
		},
		{
			name:          "Invalid regex pattern",
			containerName: "/runner-123-project-456-concurrent-789-0-build",
			jobPatterns: []string{
				"^(runner-.*-project-.*-concurrent-.*-.*-build$)",
			},
			expected: false,
		},
		{
			name:          "No match with special characters",
			containerName: "/runner-123-project-456-concurrent-789-0-build",
			jobPatterns: []string{
				"^/runner-\\d+-project-\\d+-concurrent-\\d+-\\d+-special$",
			},
			expected: false,
		},
		{
			name:          "Pattern matching container name with special characters",
			containerName: "/runner-123-project-456-concurrent-789-0-special",
			jobPatterns: []string{
				"^/runner-\\d+-project-\\d+-concurrent-\\d+-\\d+-special$",
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MatchContainerName(tc.containerName, tc.jobPatterns)
			assert.Equal(t, tc.expected, result)
		})
	}
}
