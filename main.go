package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/spf13/cobra"
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
Kubernetes manifests (Deployment, Service, ConfigMap, Secret).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			region, _ := cmd.Flags().GetString("region")
			if region == "" {
				return fmt.Errorf("region flag is required")
			}
			return runEcs2K8s(region)
		},
	}

	rootCmd.Flags().StringP("region", "r", "", "AWS region (required)")
	err := rootCmd.MarkFlagRequired("region")
	if err != nil {
		log.Fatalf("Failed to mark flag as required: %v", err)
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// validateRegion checks if the provided region is a valid AWS region
func validateRegion(region string) error {
	if region == "" {
		return fmt.Errorf("region cannot be empty")
	}

	region = strings.TrimSpace(region)

	if validAWSRegions[region] {
		return nil
	}

	// If not in our hardcoded list, log a warning but allow it
	// (new regions may have been added since this was written)
	log.Printf("Warning: Region %s is not in the known regions list. Proceeding anyway.", region)
	return nil
}

// validateAWSCredentials attempts to verify AWS credentials are configured
func validateAWSCredentials(ctx context.Context, cfg *config.Config) error {
	ecsClient := ecs.NewFromConfig(*cfg)

	// Try a simple API call with invalid cluster to verify credentials
	// If credentials are missing, this will return an auth error
	_, err := ecsClient.ListClusters(ctx, &ecs.ListClustersInput{})
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

// ensureOutputDir creates the output directory if it doesn't exist
func ensureOutputDir(outputDir string) error {
	if outputDir == "" {
		return fmt.Errorf("output directory path cannot be empty")
	}

	// Check if directory already exists
	info, err := os.Stat(outputDir)
	if err == nil {
		// Directory exists
		if !info.IsDir() {
			return fmt.Errorf("output path %s exists but is not a directory", outputDir)
		}

		// Check if writable
		testFile := filepath.Join(outputDir, ".write_test")
		if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
			return fmt.Errorf("output directory %s is not writable: %w", outputDir, err)
		}
		if err := os.Remove(testFile); err != nil {
			log.Printf("Warning: Failed to remove test file: %v", err)
		}

		log.Printf("Using existing output directory: %s", outputDir)
		return nil
	}

	// Directory doesn't exist, create it
	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check output directory: %w", err)
	}

	log.Printf("Creating output directory: %s", outputDir)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	return nil
}

func runEcs2K8s(region string) error {
	ctx := context.Background()

	// Validate region
	if err := validateRegion(region); err != nil {
		return err
	}

	log.Printf("Loading AWS configuration for region: %s", region)

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Validate AWS credentials
	log.Printf("Validating AWS credentials...")
	if err := validateAWSCredentials(ctx, &cfg); err != nil {
		return err
	}

	// Create ECS client
	ecsClient := ecs.NewFromConfig(cfg)

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

	// 3. Create output directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	outputDir := filepath.Join(cwd, selectedCluster)
	log.Printf("Output directory: %s", outputDir)

	if err := ensureOutputDir(outputDir); err != nil {
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

	for _, taskDefArn := range taskDefs {
		taskDef, err := getTaskDefinition(ctx, ecsClient, taskDefArn)
		if err != nil {
			log.Printf("Error: Failed to get task definition %s: %v", taskDefArn, err)
			failureCount++
			continue
		}

		// Generate K8s manifests
		manifests, err := convertTaskDefToK8s(taskDef)
		if err != nil {
			log.Printf("Error: Failed to convert task definition %s: %v", taskDefArn, err)
			failureCount++
			continue
		}

		// Write manifests to files
		taskDefName := extractTaskDefName(taskDefArn)
		if taskDefName == "" {
			log.Printf("Error: Could not extract task definition name from ARN: %s", taskDefArn)
			failureCount++
			continue
		}

		if err := writeManifests(outputDir, taskDefName, manifests); err != nil {
			log.Printf("Error: Failed to write manifests for %s: %v", taskDefName, err)
			failureCount++
		} else {
			log.Printf("✓ Generated manifests for %s", taskDefName)
			successCount++
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
	log.Printf("========================================\n")

	if successCount == 0 {
		return fmt.Errorf("no task definitions were successfully converted")
	}

	log.Printf("✅ Conversion complete!")
	return nil
}
