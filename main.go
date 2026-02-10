package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/spf13/cobra"

	"ecs2eks/validators"
)

// validAWSRegions contains all valid AWS regions
var validAWSRegions = map[string]bool{
	"us-east-1":      true,
	"us-east-2":      true,
	"us-west-1":      true,
	"us-west-2":      true,
	"eu-west-1":      true,
	"eu-west-2":      true,
	"eu-west-3":      true,
	"eu-central-1":   true,
	"eu-north-1":     true,
	"ap-south-1":     true,
	"ap-southeast-1": true,
	"ap-southeast-2": true,
	"ap-northeast-1": true,
	"ap-northeast-2": true,
	"ap-northeast-3": true,
	"ca-central-1":   true,
	"sa-east-1":      true,
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "ecs2k8s",
		Short: "Convert AWS ECS task definitions to Kubernetes manifests",
		Long: `ecs2k8s converts AWS ECS clusters and task definitions into equivalent
Kubernetes manifests (Deployment, Service, ConfigMap, Secret) and optionally
generates a Helm chart or Kustomize structure for easy deployment and management.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			region, _ := cmd.Flags().GetString("region")
			if region == "" {
				return fmt.Errorf("region flag is required")
			}

			if err := validateRegion(region); err != nil {
				return err
			}

			createHelm, _ := cmd.Flags().GetBool("create-helm")
			createKustomize, _ := cmd.Flags().GetBool("create-kustomize")

			return runEcs2K8s(region, createHelm, createKustomize)
		},
	}

	rootCmd.Flags().StringP("region", "r", "", "AWS region (required)")
	rootCmd.Flags().BoolP("create-helm", "H", false, "Create Helm chart (default: false)")
	rootCmd.Flags().BoolP("create-kustomize", "K", false, "Create Kustomize structure with base and overlays (default: false)")

	err := rootCmd.MarkFlagRequired("region")
	if err != nil {
		log.Fatalf("Failed to mark flag as required: %v", err)
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// validateRegion checks if the provided region is a valid AWS region using validators package
func validateRegion(region string) error {
	rv := &validators.RegionValidator{Region: region}

	// Validate format first
	if err := rv.ValidateFormat(); err != nil {
		return err
	}

	// Check against known regions (non-fatal warning if unknown)
	if err := rv.ValidateKnownRegion(); err != nil {
		log.Printf("Warning: %v", err)
	}

	return nil
}

// validateRegionWithAWS checks region with AWS API
func validateRegionWithAWS(ctx context.Context, region string, ec2Client *ec2.Client) error {
	rv := &validators.RegionValidator{Region: region}
	return rv.ValidateWithAWS(ctx, ec2Client)
}

// validateAWSCredentials attempts to verify AWS credentials are configured
func validateAWSCredentials(ctx context.Context, client *ecs.Client) error {
	// Try a simple API call with invalid cluster to verify credentials
	// If credentials are missing, this will return an auth error
	_, err := client.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "NoCredentialProviders") ||
			strings.Contains(errStr, "InvalidClientTokenId") ||
			strings.Contains(errStr, "UnrecognizedClientException") {
			return fmt.Errorf("AWS credentials not configured or invalid: %w", err)
		}
		// Other errors are acceptable (e.g., permission denied) as they mean credentials exist
		log.Printf("Note: AWS credential validation returned: %v (this may be normal)", err)
	}

	return nil
}

// createOutputDirectory creates the output directory with proper error handling
func createOutputDirectory(outputDir string) error {
	if outputDir == "" {
		return fmt.Errorf("output directory path cannot be empty")
	}

	// Check if directory already exists
	info, err := os.Stat(outputDir)
	if err == nil {
		// Directory exists
		if !info.IsDir() {
			return fmt.Errorf("path exists but is not a directory: %s", outputDir)
		}
		log.Printf("Output directory already exists: %s", outputDir)
		return nil
	}

	if !os.IsNotExist(err) {
		// Some other error occurred
		return fmt.Errorf("failed to stat output directory: %w", err)
	}

	// Directory doesn't exist, create it
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	log.Printf("Created output directory: %s", outputDir)
	return nil
}

func runEcs2K8s(region string, createHelm bool, createKustomize bool) error {
	ctx := context.Background()

	log.Printf("Loading AWS configuration for region: %s", region)
	log.Printf("Create Helm chart: %v", createHelm)
	log.Printf("Create Kustomize structure: %v", createKustomize)

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create ECS client
	ecsClient := ecs.NewFromConfig(cfg)

	// Validate AWS credentials
	log.Printf("Validating AWS credentials...")
	if err := validateAWSCredentials(ctx, ecsClient); err != nil {
		return err
	}

	// 1. Discover ECS clusters
	log.Printf("Discovering ECS clusters in region %s...", region)
	clusters, err := listClusters(ctx, ecsClient)
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	log.Printf("Found %d cluster(s)", len(clusters))

	// 2. Interactive cluster selection
	selectedCluster, err := selectCluster(clusters)
	if err != nil {
		return fmt.Errorf("cluster selection failed: %w", err)
	}

	log.Printf("Selected cluster: %s", selectedCluster)

	// 2a. Validate selected cluster
	if err := validateSelectedCluster(ctx, selectedCluster, ecsClient); err != nil {
		return fmt.Errorf("cluster validation failed: %w", err)
	}

	// 3. Create output directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	outputDir := filepath.Join(cwd, selectedCluster)
	log.Printf("Output directory: %s", outputDir)

	if err := createOutputDirectory(outputDir); err != nil {
		return err
	}

	// 4. Process task definitions
	log.Printf("Retrieving task definitions from cluster %s...", selectedCluster)
	taskDefs, err := listTaskDefinitions(ctx, ecsClient, selectedCluster)
	if err != nil {
		return fmt.Errorf("failed to list task definitions: %w", err)
	}

	if len(taskDefs) == 0 {
		log.Printf("No task definitions found in cluster %s. Nothing to convert.", selectedCluster)
		return nil
	}

	log.Printf("Found %d task definition(s) to convert", len(taskDefs))

	successCount := 0
	failureCount := 0
	var taskDefInfos []*TaskDefInfo

	for _, taskDefArn := range taskDefs {
		if taskDefArn == "" {
			log.Printf("Warning: Empty task definition ARN encountered, skipping")
			failureCount++
			continue
		}

		// Validate task definition ARN before fetching
		if err := validateTaskDefArn(ctx, taskDefArn, ecsClient); err != nil {
			log.Printf("Warning: Task definition validation failed for %s: %v (attempting to continue)", taskDefArn, err)
		}

		taskDef, err := getTaskDefinition(ctx, ecsClient, taskDefArn)
		if err != nil {
			log.Printf("Error: Failed to get task definition %s: %v", taskDefArn, err)
			failureCount++
			continue
		}

		if taskDef == nil {
			log.Printf("Error: Task definition %s is nil", taskDefArn)
			failureCount++
			continue
		}

		// Extract task definition name
		taskDefName := extractTaskDefName(taskDefArn)
		if taskDefName == "" {
			log.Printf("Error: Could not extract task definition name from ARN: %s", taskDefArn)
			failureCount++
			continue
		}

		// Convert to TaskDefInfo for Helm support
		taskDefInfo, err := convertTaskDefToInfo(taskDef, taskDefName)
		if err != nil {
			log.Printf("Error: Failed to convert task definition %s to info: %v", taskDefName, err)
			failureCount++
			continue
		}

		taskDefInfo.Manifests = K8sManifests{}

		// Generate K8s manifests
		manifests, err := convertTaskDefToK8s(taskDef)
		if err != nil {
			log.Printf("Error: Failed to convert task definition %s: %v", taskDefArn, err)
			failureCount++
			continue
		}

		taskDefInfo.Manifests = manifests

		// Write manifests to files
		if err := writeManifests(outputDir, taskDefName, manifests); err != nil {
			log.Printf("Error: Failed to write manifests for %s: %v", taskDefName, err)
			failureCount++
		} else {
			log.Printf("✓ Generated manifests for %s", taskDefName)
			successCount++
			taskDefInfos = append(taskDefInfos, taskDefInfo)
		}
	}

	// 5. Create Helm chart if requested
	if createHelm && len(taskDefInfos) > 0 {
		log.Printf("Creating Helm chart for cluster: %s", selectedCluster)
		if err := CreateHelmChart(selectedCluster, taskDefInfos, outputDir); err != nil {
			log.Printf("Error: Failed to create Helm chart: %v", err)
			return err
		}
	}

	// 6. Create Kustomize structure if requested
	if createKustomize && len(taskDefInfos) > 0 {
		log.Printf("Creating Kustomize structure for cluster: %s", selectedCluster)
		if err := CreateKustomizeChart(selectedCluster, taskDefInfos, outputDir); err != nil {
			log.Printf("Error: Failed to create Kustomize structure: %v", err)
			return err
		}
	}

	// Summary
	log.Printf("\n")
	log.Printf("========================================")
	log.Printf("Conversion Summary")
	log.Printf("========================================")
	log.Printf("Successfully converted: %d task definition(s)", successCount)
	log.Printf("Failed: %d task definition(s)", failureCount)
	log.Printf("Output directory: %s", outputDir)
	if createHelm {
		log.Printf("Helm chart: %s/helm/%s", selectedCluster, selectedCluster)
	}
	if createKustomize {
		log.Printf("Kustomize structure: %s/kustomize/%s", selectedCluster, selectedCluster)
	}
	log.Printf("========================================\n")

	if successCount == 0 {
		return fmt.Errorf("no task definitions were successfully converted")
	}

	log.Printf("✅ Conversion complete!")
	return nil
}

// validateSelectedCluster validates the selected cluster using validators package
func validateSelectedCluster(ctx context.Context, clusterName string, ecsClient *ecs.Client) error {
	cv := &validators.ClusterValidator{ClusterName: clusterName}

	// Validate name and format
	if err := cv.ValidateName(); err != nil {
		return err
	}

	if err := cv.ValidateFormat(); err != nil {
		return err
	}

	// Validate cluster exists and is active
	if ecsClient != nil {
		if err := cv.ValidateExists(ctx, ecsClient); err != nil {
			return err
		}

		if err := cv.ValidateActive(ctx, ecsClient); err != nil {
			log.Printf("Warning: Cluster status check: %v", err)
		}
	}

	return nil
}

// validateTaskDefArn validates task definition ARN using validators package
func validateTaskDefArn(ctx context.Context, taskDefArn string, ecsClient *ecs.Client) error {
	tv := &validators.TaskDefinitionValidator{TaskDefARN: taskDefArn}

	// Validate name and format
	if err := tv.ValidateName(); err != nil {
		return err
	}

	if err := tv.ValidateARNFormat(); err != nil {
		return err
	}

	// Validate task definition exists (optional with ECS client)
	if ecsClient != nil {
		if err := tv.ValidateExists(ctx, ecsClient); err != nil {
			log.Printf("Warning: Task definition existence check: %v", err)
		}
	}

	return nil
}

// validateManifest validates generated Kubernetes manifest using validators package
func validateManifest(manifestPath string, content []byte) error {
	mv := &validators.ManifestValidator{
		ManifestPath: manifestPath,
		Content:      content,
	}
	return mv.Validate()
}
