package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// Check https://access.redhat.com/documentation/en-us/red_hat_advanced_cluster_management_for_kubernetes/2.9/html/clusters/cluster_mce_overview#hosted-sizing-guidance
// for more details on the sizing guidance

// TODO Keep in mind that this is based on performance and scale regression fitting and is subject to change in the future as more testing
// is done and more data is collected
const (
	controlPlaneCPU     float64 = 8   // CPUs required for OpenShift Control Plane
	controlPlaneMemory  float64 = 2.5 // GB Memory for OpenShift Control Plane
	cpuRequestPerHCP    float64 = 5   // CPUs required per HCP
	memoryRequestPerHCP float64 = 18  // GB memory required per HCP
	podsPerHCP          float64 = 75  // Pods per HCP

	incrementalCPUUsagePer1KQPS float64 = 9.0 // Incremental CPU usage per 1K QPS
	incrementalMemUsagePer1KQPS float64 = 2.5 // Incremental Memory usage per 1K QPS

	idleCPUUsage    float64 = 2.9  // Idle CPU usage (unit vCPU)
	idleMemoryUsage float64 = 11.1 // Idle memory usage (unit GiB)
)

type ServerResources struct {
	WorkerCPUs        float64
	WorkerMemory      float64
	MaxPods           float64
	PodCount          float64
	APIRate           float64
	EtcdStorage       float64
	MaxHCPs           float64
	UseLoadBased      bool
	CalculationMethod int
	HCPlimit          string
	totalNodes        float64
	totalHCPs         float64
}

// Linear regression constants for ETCD storage calculation derived from performance and scale regression fitting
const (
	etcdStorageSlope  float64 = 6.66e-4
	etcdStorageOffset float64 = 0.103
)

func calculateETCDStorage(podCount float64) float64 {
	return etcdStorageSlope*podCount + etcdStorageOffset
}

func calculateMaxHCPs(workerCPUs, workerMemory, maxPods, apiRate float64, useLoadBased bool) (float64, string) {
	//print all values for debugging
	fmt.Printf("workerCPUs: %f\n", workerCPUs)
	fmt.Printf("workerMemory: %f\n", workerMemory)
	fmt.Printf("maxPods: %f\n", maxPods)
	fmt.Printf("apiRate: %f\n", apiRate)
	fmt.Printf("Using Load-based: %t\n", useLoadBased)

	var limitHCP string

	maxHCPsByCPU := (workerCPUs - controlPlaneCPU) / cpuRequestPerHCP
	maxHCPsByMemory := (workerMemory - controlPlaneMemory) / memoryRequestPerHCP
	maxHCPsByPods := maxPods / podsPerHCP

	var maxHCPsByCPUUsage, maxHCPsByMemoryUsage float64
	if useLoadBased {
		maxHCPsByCPUUsage = workerCPUs / (idleCPUUsage + (apiRate/1000)*incrementalCPUUsagePer1KQPS)
		maxHCPsByMemoryUsage = workerMemory / (idleMemoryUsage + (apiRate/1000)*incrementalMemUsagePer1KQPS)
	} else {
		maxHCPsByCPUUsage = maxHCPsByCPU
		maxHCPsByMemoryUsage = maxHCPsByMemory
	}

	// Return the minimum of all the calculated values (this is the maximum number of HCPs that can be hosted)
	// This considers the most constrained resource as the limiting factor (e.g., CPU, Memory, Pods, etc.)
	minHCPs := math.Min(maxHCPsByCPU, math.Min(maxHCPsByCPUUsage, math.Min(maxHCPsByMemory, math.Min(maxHCPsByMemoryUsage, maxHCPsByPods))))

	// Return which resources is limiting the maximum number of HCPs that can be hosted
	if maxHCPsByCPUUsage == minHCPs || maxHCPsByCPU == minHCPs {
		limitHCP = "CPU"
	}
	if maxHCPsByMemory == minHCPs || maxHCPsByMemoryUsage == minHCPs {
		limitHCP = "Memory"
	}
	if maxHCPsByPods == minHCPs {
		limitHCP = "Pods"
	}

	return minHCPs, limitHCP
}

func promptForInput(promptLabel string) float64 {
	prompt := promptui.Prompt{
		Label: promptLabel,
		Validate: func(input string) error {
			if _, err := strconv.ParseFloat(input, 64); err != nil {
				return fmt.Errorf("invalid number")
			}
			return nil
		},
	}

	result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		os.Exit(1)
	}

	value, _ := strconv.ParseFloat(result, 64)
	return value
}

func promptForSelection(promptLabel string, items []string) int {
	prompt := promptui.Select{
		Label: promptLabel,
		Items: items,
	}

	_, result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		os.Exit(1)
	}

	for i, item := range items {
		if item == result {
			return i
		}
	}
	return -1
}

var (
	workerCPUs   float64
	workerMemory float64
	maxPods      float64
	podCount     float64
	totalNodes   float64
	apiRate      float64
	useLoadBased bool
	interactive  bool
	discoverMode bool
)

var rootCmd = &cobra.Command{
	Use:   "hcp-sizer",
	Short: "HCP Sizer is a tool to calculate HCP cluster sizing",
	RunE: func(cmd *cobra.Command, args []string) error {
		// First, check if we're in non-interactive mode and validate required arguments
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

		// If discover mode is enabled, use the discover function
		if discoverMode {
			clientset, err := InitializeKubernetesClientForExternalUse()
			if err != nil {
				return fmt.Errorf("failed to initialize Kubernetes client: %v", err)
			}

			nodeResources, err := FetchClusterDataTwo(clientset)
			if err != nil {
				return fmt.Errorf("failed to fetch cluster data: %v", err)
			}

			// Use the discovered values from the first node
			if len(nodeResources) > 0 {
				workerCPUs = nodeResources[0].CPU
				workerMemory = nodeResources[0].Memory
				maxPods = float64(nodeResources[0].MaxPods)
				// We can't discover podCount and totalNodes from the cluster
				// These will need to be provided by the user
			}
		}

		// Only show prompts if explicitly in interactive mode
		if interactive {
			if workerCPUs == 0 {
				workerCPUs = promptForInput("Enter worker node CPU cores")
			}
			if workerMemory == 0 {
				workerMemory = promptForInput("Enter worker node memory (GB)")
			}
			if maxPods == 0 {
				maxPods = promptForInput("Enter max pods per node")
			}
			if podCount == 0 {
				podCount = promptForInput("Enter total number of pods")
			}
			if totalNodes == 0 {
				totalNodes = promptForInput("Enter total number of nodes")
			}
			if !useLoadBased {
				useLoadBased = promptForSelection("Use load-based calculation?", []string{"Yes", "No"}) == 0
			}
			if useLoadBased && apiRate == 0 {
				apiRate = promptForInput("Enter API rate (requests per second)")
			}
		}

		// Calculate and display results
		serverResources := &ServerResources{
			WorkerCPUs:   workerCPUs,
			WorkerMemory: workerMemory,
			MaxPods:      maxPods,
			PodCount:     podCount,
			totalNodes:   totalNodes,
			UseLoadBased: useLoadBased,
			APIRate:      apiRate,
		}

		serverResources.MaxHCPs, serverResources.HCPlimit = calculateMaxHCPs(serverResources.WorkerCPUs, serverResources.WorkerMemory, serverResources.MaxPods, serverResources.APIRate, serverResources.UseLoadBased)
		serverResources.EtcdStorage = calculateETCDStorage(serverResources.PodCount)
		serverResources.totalHCPs = serverResources.totalNodes * math.Floor(serverResources.MaxHCPs)

		yellow := color.New(color.FgYellow)
		italicYellow := yellow.Add(color.Italic)

		// Print the results
		italicYellow.Printf("Maximum HCPs that can be hosted per node: %.2f\n", math.Floor(serverResources.MaxHCPs))
		italicYellow.Printf("Estimated HCPs that can be hosted in the hosting cluster: %.2f\n", math.Floor(serverResources.totalHCPs))
		italicYellow.Printf("Estimated HCP ETCD Storage Requirement: %.3f GiB\n", serverResources.EtcdStorage)
		italicYellow.Printf("Limiting Resource: %s\n", serverResources.HCPlimit)

		return nil
	},
}

func init() {
	rootCmd.Flags().Float64Var(&workerCPUs, "worker-cpus", 0, "Number of CPU cores per worker node")
	rootCmd.Flags().Float64Var(&workerMemory, "worker-memory", 0, "Memory in GB per worker node")
	rootCmd.Flags().Float64Var(&maxPods, "max-pods", 0, "Maximum number of pods per node")
	rootCmd.Flags().Float64Var(&podCount, "pod-count", 0, "Total number of pods in the cluster")
	rootCmd.Flags().Float64Var(&totalNodes, "total-nodes", 0, "Total number of nodes in the cluster")
	rootCmd.Flags().Float64Var(&apiRate, "api-rate", 0, "API rate in requests per second (required for load-based calculation)")
	rootCmd.Flags().BoolVar(&useLoadBased, "load-based", false, "Use load-based calculation")
	rootCmd.Flags().BoolVar(&interactive, "interactive", true, "Run in interactive mode and prompt for any missing values. If omitted or set to false (use --interactive=false), all required arguments must be provided via flags")
	rootCmd.Flags().BoolVar(&discoverMode, "discover", false, "Discover cluster resources automatically")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
