package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/spf13/cobra"
)

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
		return
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runEcs2K8s(region string) error {
	ctx := context.Background()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create ECS client
	ecsClient := ecs.NewFromConfig(cfg)

	// 1. Discover ECS clusters
	clusters, err := listClusters(ctx, ecsClient)
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	// 2. Interactive cluster selection
	selectedCluster, err := selectCluster(clusters)
	if err != nil {
		return err
	}

	fmt.Printf("Selected cluster: %s\n", selectedCluster)

	// 3. Create output directory (use absolute path)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	outputDir := filepath.Join(cwd, selectedCluster)
	fmt.Printf("[DEBUG] Output directory: %s\n", outputDir)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 4. Process task definitions
	taskDefs, err := listTaskDefinitions(ctx, ecsClient, selectedCluster)
	if err != nil {
		return fmt.Errorf("failed to list task definitions: %w", err)
	}

	for _, taskDefArn := range taskDefs {
		taskDef, err := getTaskDefinition(ctx, ecsClient, taskDefArn)
		if err != nil {
			fmt.Printf("Warning: failed to get task definition %s: %v\n", taskDefArn, err)
			continue
		}

		// Generate K8s manifests
		manifests, err := convertTaskDefToK8s(taskDef)
		if err != nil {
			fmt.Printf("Warning: failed to convert task definition %s: %v\n", taskDefArn, err)
			continue
		}

		// Write manifests to files
		taskDefName := extractTaskDefName(taskDefArn)
		if err := writeManifests(outputDir, taskDefName, manifests); err != nil {
			fmt.Printf("Warning: failed to write manifests for %s: %v\n", taskDefName, err)
		} else {
			fmt.Printf("✓ Generated manifests for %s in %s/\n", taskDefName, outputDir)
		}
	}

	fmt.Printf("\n✅ Conversion complete! Manifests saved in %s/\n", outputDir)
	return nil
}
