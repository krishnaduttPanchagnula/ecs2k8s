package validators

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// ValidAWSRegions contains all valid AWS regions
var ValidAWSRegions = map[string]bool{
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

// RegionValidator validates AWS region input
type RegionValidator struct {
	Region string
}

// ValidateFormat checks if region has valid format
func (rv *RegionValidator) ValidateFormat() error {
	if rv.Region == "" {
		return fmt.Errorf("region cannot be empty")
	}

	rv.Region = strings.TrimSpace(rv.Region)

	// Basic format check: xx-xxxx-x
	if !strings.Contains(rv.Region, "-") {
		return fmt.Errorf("invalid region format: %s (expected format: xx-xxxx-x)", rv.Region)
	}

	parts := strings.Split(rv.Region, "-")
	if len(parts) != 3 {
		return fmt.Errorf("invalid region format: %s (expected format: xx-xxxx-x)", rv.Region)
	}

	return nil
}

// ValidateKnownRegion checks if region is in known regions list
func (rv *RegionValidator) ValidateKnownRegion() error {
	if !ValidAWSRegions[rv.Region] {
		return fmt.Errorf("region %s is not in known regions list (may be new region)", rv.Region)
	}
	return nil
}

// ValidateWithAWS checks if region is accessible via AWS API
func (rv *RegionValidator) ValidateWithAWS(ctx context.Context, ec2Client *ec2.Client) error {
	if ec2Client == nil {
		return fmt.Errorf("EC2 client is nil")
	}

	_, err := ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{
		RegionNames: []string{rv.Region},
	})
	if err != nil {
		return fmt.Errorf("failed to validate region with AWS: %w", err)
	}

	return nil
}

// Validate performs complete validation
func (rv *RegionValidator) Validate(ctx context.Context, ec2Client *ec2.Client) error {
	if err := rv.ValidateFormat(); err != nil {
		return err
	}

	if err := rv.ValidateKnownRegion(); err != nil {
		log.Printf("Warning: %v", err)
	}

	if ec2Client != nil {
		if err := rv.ValidateWithAWS(ctx, ec2Client); err != nil {
			log.Printf("Warning: %v", err)
		}
	}

	return nil
}

// ClusterValidator validates ECS cluster
type ClusterValidator struct {
	ClusterName string
}

// ValidateName checks if cluster name is not empty
func (cv *ClusterValidator) ValidateName() error {
	if cv.ClusterName == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}

	cv.ClusterName = strings.TrimSpace(cv.ClusterName)

	if cv.ClusterName == "" {
		return fmt.Errorf("cluster name cannot be empty or whitespace only")
	}

	return nil
}

// ValidateFormat checks if cluster name has valid format
func (cv *ClusterValidator) ValidateFormat() error {
	if !isValidClusterName(cv.ClusterName) {
		return fmt.Errorf("invalid cluster name format: %s (must contain only alphanumeric characters, hyphens, and underscores)", cv.ClusterName)
	}
	return nil
}

// ValidateExists checks if cluster exists
func (cv *ClusterValidator) ValidateExists(ctx context.Context, ecsClient *ecs.Client) error {
	if ecsClient == nil {
		return fmt.Errorf("ECS client is nil")
	}

	output, err := ecsClient.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{cv.ClusterName},
	})
	if err != nil {
		return fmt.Errorf("failed to describe cluster: %w", err)
	}

	if len(output.Clusters) == 0 {
		return fmt.Errorf("cluster %s not found", cv.ClusterName)
	}

	return nil
}

// ValidateActive checks if cluster is in ACTIVE state
func (cv *ClusterValidator) ValidateActive(ctx context.Context, ecsClient *ecs.Client) error {
	if ecsClient == nil {
		return fmt.Errorf("ECS client is nil")
	}

	output, err := ecsClient.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{cv.ClusterName},
	})
	if err != nil {
		return fmt.Errorf("failed to describe cluster: %w", err)
	}

	if len(output.Clusters) == 0 {
		return fmt.Errorf("cluster %s not found", cv.ClusterName)
	}

	status := string(*output.Clusters[0].Status)
	if status != "ACTIVE" {
		return fmt.Errorf("cluster %s is not active (status: %s)", cv.ClusterName, status)
	}

	return nil
}

// Validate performs complete validation
func (cv *ClusterValidator) Validate(ctx context.Context, ecsClient *ecs.Client) error {
	if err := cv.ValidateName(); err != nil {
		return err
	}

	if err := cv.ValidateFormat(); err != nil {
		return err
	}

	if ecsClient != nil {
		if err := cv.ValidateExists(ctx, ecsClient); err != nil {
			return err
		}

		if err := cv.ValidateActive(ctx, ecsClient); err != nil {
			log.Printf("Warning: %v", err)
		}
	}

	return nil
}

// TaskDefinitionValidator validates ECS task definition
type TaskDefinitionValidator struct {
	TaskDefARN string
}

// ValidateName checks if task definition ARN is not empty
func (tv *TaskDefinitionValidator) ValidateName() error {
	if tv.TaskDefARN == "" {
		return fmt.Errorf("task definition ARN cannot be empty")
	}

	tv.TaskDefARN = strings.TrimSpace(tv.TaskDefARN)

	if tv.TaskDefARN == "" {
		return fmt.Errorf("task definition ARN cannot be empty or whitespace only")
	}

	return nil
}

// ValidateARNFormat checks if ARN has valid format
func (tv *TaskDefinitionValidator) ValidateARNFormat() error {
	if !strings.HasPrefix(tv.TaskDefARN, "arn:aws:ecs:") && !isValidTaskDefName(tv.TaskDefARN) {
		return fmt.Errorf("invalid task definition ARN/name format: %s", tv.TaskDefARN)
	}
	return nil
}

// ValidateExists checks if task definition exists
func (tv *TaskDefinitionValidator) ValidateExists(ctx context.Context, ecsClient *ecs.Client) error {
	if ecsClient == nil {
		return fmt.Errorf("ECS client is nil")
	}

	_, err := ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &tv.TaskDefARN,
	})
	if err != nil {
		return fmt.Errorf("task definition %s not found: %w", tv.TaskDefARN, err)
	}

	return nil
}

// Validate performs complete validation
func (tv *TaskDefinitionValidator) Validate(ctx context.Context, ecsClient *ecs.Client) error {
	if err := tv.ValidateName(); err != nil {
		return err
	}

	if err := tv.ValidateARNFormat(); err != nil {
		return err
	}

	if ecsClient != nil {
		if err := tv.ValidateExists(ctx, ecsClient); err != nil {
			return err
		}
	}

	return nil
}

// ManifestValidator validates generated Kubernetes manifests
type ManifestValidator struct {
	ManifestPath string
	Content      []byte
}

// ValidatePath checks if manifest path is not empty
func (mv *ManifestValidator) ValidatePath() error {
	if mv.ManifestPath == "" {
		return fmt.Errorf("manifest path cannot be empty")
	}
	return nil
}

// ValidateYAML checks if content is valid YAML
func (mv *ManifestValidator) ValidateYAML() error {
	if len(mv.Content) == 0 {
		return fmt.Errorf("manifest content cannot be empty")
	}

	// Basic YAML validation - check for common patterns
	content := string(mv.Content)

	if !strings.Contains(content, "apiVersion:") {
		return fmt.Errorf("missing apiVersion field in manifest")
	}

	if !strings.Contains(content, "kind:") {
		return fmt.Errorf("missing kind field in manifest")
	}

	if !strings.Contains(content, "metadata:") {
		return fmt.Errorf("missing metadata field in manifest")
	}

	return nil
}

// ValidateKubernetesKind checks if manifest has valid Kubernetes kind
func (mv *ManifestValidator) ValidateKubernetesKind() error {
	content := string(mv.Content)

	validKinds := []string{
		"Deployment",
		"Service",
		"ConfigMap",
		"Secret",
		"Pod",
		"StatefulSet",
		"DaemonSet",
	}

	foundKind := false
	for _, kind := range validKinds {
		if strings.Contains(content, fmt.Sprintf("kind: %s", kind)) {
			foundKind = true
			break
		}
	}

	if !foundKind {
		return fmt.Errorf("invalid or unsupported Kubernetes kind in manifest")
	}

	return nil
}

// Validate performs complete validation
func (mv *ManifestValidator) Validate() error {
	if err := mv.ValidatePath(); err != nil {
		return err
	}

	if err := mv.ValidateYAML(); err != nil {
		return err
	}

	if err := mv.ValidateKubernetesKind(); err != nil {
		return err
	}

	return nil
}

// Helper functions

// isValidClusterName checks if cluster name has valid format
func isValidClusterName(name string) bool {
	if name == "" {
		return false
	}

	// Cluster names can contain alphanumeric characters, hyphens, and underscores
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '-' ||
			ch == '_') {
			return false
		}
	}

	return true
}

// isValidTaskDefName checks if task definition name has valid format
func isValidTaskDefName(name string) bool {
	if name == "" {
		return false
	}

	// Task definition names can contain alphanumeric characters, hyphens, and underscores
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '-' ||
			ch == '_') {
			return false
		}
	}

	return true
}
