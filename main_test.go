package main

import (
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// TestFlags tests the flag parsing functionality
func TestFlags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedFlags map[string]interface{}
		expectedError bool
	}{
		{
			name: "all flags provided",
			args: []string{
				"--worker-cpus", "8",
				"--worker-memory", "32",
				"--max-pods", "250",
				"--pod-count", "1000",
				"--total-nodes", "3",
				"--load-based",
				"--api-rate", "5000",
			},
			expectedFlags: map[string]interface{}{
				"worker-cpus":   8.0,
				"worker-memory": 32.0,
				"max-pods":      250.0,
				"pod-count":     1000.0,
				"total-nodes":   3.0,
				"load-based":    true,
				"api-rate":      5000.0,
				"interactive":   false,
				"discover":      false,
			},
			expectedError: false,
		},
		{
			name: "interactive mode",
			args: []string{"--interactive"},
			expectedFlags: map[string]interface{}{
				"interactive": true,
				"discover":    false,
			},
			expectedError: false,
		},
		{
			name: "discover mode",
			args: []string{"--discover"},
			expectedFlags: map[string]interface{}{
				"interactive": false,
				"discover":    true,
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags for each test
			workerCPUs = 0
			workerMemory = 0
			maxPods = 0
			podCount = 0
			totalNodes = 0
			apiRate = 0
			useLoadBased = false
			interactive = false
			discoverMode = false

			// Create a new command for testing
			cmd := &cobra.Command{
				Use: "test",
				Run: func(cmd *cobra.Command, args []string) {},
			}

			// Add all flags to the test command
			cmd.Flags().Float64Var(&workerCPUs, "worker-cpus", 0, "")
			cmd.Flags().Float64Var(&workerMemory, "worker-memory", 0, "")
			cmd.Flags().Float64Var(&maxPods, "max-pods", 0, "")
			cmd.Flags().Float64Var(&podCount, "pod-count", 0, "")
			cmd.Flags().Float64Var(&totalNodes, "total-nodes", 0, "")
			cmd.Flags().Float64Var(&apiRate, "api-rate", 0, "")
			cmd.Flags().BoolVar(&useLoadBased, "load-based", false, "")
			cmd.Flags().BoolVar(&interactive, "interactive", false, "")
			cmd.Flags().BoolVar(&discoverMode, "discover", false, "")

			// Execute the command with test args
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			// Verify flag values
			for flag, expectedValue := range tt.expectedFlags {
				switch flag {
				case "worker-cpus":
					assert.Equal(t, expectedValue.(float64), workerCPUs)
				case "worker-memory":
					assert.Equal(t, expectedValue.(float64), workerMemory)
				case "max-pods":
					assert.Equal(t, expectedValue.(float64), maxPods)
				case "pod-count":
					assert.Equal(t, expectedValue.(float64), podCount)
				case "total-nodes":
					assert.Equal(t, expectedValue.(float64), totalNodes)
				case "api-rate":
					assert.Equal(t, expectedValue.(float64), apiRate)
				case "load-based":
					assert.Equal(t, expectedValue.(bool), useLoadBased)
				case "interactive":
					assert.Equal(t, expectedValue.(bool), interactive)
				case "discover":
					assert.Equal(t, expectedValue.(bool), discoverMode)
				}
			}
		})
	}
}

// TestRequiredArgs tests the validation of required arguments
func TestRequiredArgs(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError bool
	}{
		{
			name:          "no arguments",
			args:          []string{},
			expectedError: true,
		},
		{
			name:          "interactive false without args",
			args:          []string{"--interactive=false"},
			expectedError: true,
		},
		{
			name: "missing worker-cpus",
			args: []string{
				"--worker-memory", "32",
				"--max-pods", "250",
				"--pod-count", "1000",
				"--total-nodes", "3",
			},
			expectedError: true,
		},
		{
			name: "missing api-rate with load-based",
			args: []string{
				"--worker-cpus", "8",
				"--worker-memory", "32",
				"--max-pods", "250",
				"--pod-count", "1000",
				"--total-nodes", "3",
				"--load-based",
			},
			expectedError: true,
		},
		{
			name: "all required args provided",
			args: []string{
				"--worker-cpus", "8",
				"--worker-memory", "32",
				"--max-pods", "250",
				"--pod-count", "1000",
				"--total-nodes", "3",
			},
			expectedError: false,
		},
		{
			name: "all required args with load-based",
			args: []string{
				"--worker-cpus", "8",
				"--worker-memory", "32",
				"--max-pods", "250",
				"--pod-count", "1000",
				"--total-nodes", "3",
				"--load-based",
				"--api-rate", "5000",
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags for each test
			workerCPUs = 0
			workerMemory = 0
			maxPods = 0
			podCount = 0
			totalNodes = 0
			apiRate = 0
			useLoadBased = false
			interactive = false
			discoverMode = false

			// Create a new command for testing
			cmd := &cobra.Command{
				Use: "test",
				RunE: func(cmd *cobra.Command, args []string) error {
					// Check if we have required arguments when not in interactive mode
					if !interactive {
						hasRequiredArgs := workerCPUs > 0 && workerMemory > 0 && maxPods > 0 &&
							podCount > 0 && totalNodes > 0 && (useLoadBased && apiRate > 0 || !useLoadBased)

						if !hasRequiredArgs {
							return fmt.Errorf("when not in interactive mode, all required arguments must be provided:\n" +
								"  --worker-cpus\n" +
								"  --worker-memory\n" +
								"  --max-pods\n" +
								"  --pod-count\n" +
								"  --total-nodes\n" +
								"  --load-based (optional)\n" +
								"  --api-rate (required if --load-based is set)")
						}
					}
					return nil
				},
			}

			// Add all flags to the test command
			cmd.Flags().Float64Var(&workerCPUs, "worker-cpus", 0, "")
			cmd.Flags().Float64Var(&workerMemory, "worker-memory", 0, "")
			cmd.Flags().Float64Var(&maxPods, "max-pods", 0, "")
			cmd.Flags().Float64Var(&podCount, "pod-count", 0, "")
			cmd.Flags().Float64Var(&totalNodes, "total-nodes", 0, "")
			cmd.Flags().Float64Var(&apiRate, "api-rate", 0, "")
			cmd.Flags().BoolVar(&useLoadBased, "load-based", false, "")
			cmd.Flags().BoolVar(&interactive, "interactive", false, "")
			cmd.Flags().BoolVar(&discoverMode, "discover", false, "")

			// Execute the command with test args
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.expectedError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "when not in interactive mode, all required arguments must be provided")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestDiscoverMode tests the discover mode behavior
func TestDiscoverMode(t *testing.T) {
	// Create a new command for testing
	cmd := &cobra.Command{
		Use: "test",
		Run: func(cmd *cobra.Command, args []string) {
			// Test discover mode
			clientset, err := InitializeKubernetesClientForExternalUse()
			if err != nil {
				t.Skip("Skipping discover mode test: no valid kubeconfig found")
				return
			}

			nodeResources, err := FetchClusterDataTwo(clientset)
			if err != nil {
				t.Skip("Skipping discover mode test: no valid cluster connection")
				return
			}

			// Verify we got some node resources
			assert.NotEmpty(t, nodeResources)
		},
	}
	cmd.Flags().BoolVar(&discoverMode, "discover", false, "")

	// Execute the command
	cmd.SetArgs([]string{"--discover"})
	err := cmd.Execute()

	// We don't assert on error here because the test might be skipped
	// if there's no valid kubeconfig or cluster connection
	if err != nil {
		t.Logf("Discover mode test skipped: %v", err)
	}
}
