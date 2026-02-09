package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/manifoldco/promptui"
)

// listClusters lists ECS clusters in the region (by name)
func listClusters(ctx context.Context, client *ecs.Client) ([]string, error) {
	var clusters []string
	input := &ecs.ListClustersInput{MaxResults: aws.Int32(100)}

	paginator := ecs.NewListClustersPaginator(client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, arn := range page.ClusterArns {
			clusterName := extractClusterName(arn)
			if clusterName == "" {
				log.Printf("Warning: Failed to extract cluster name from ARN: %s", arn)
				continue
			}
			clusters = append(clusters, clusterName)
		}
	}

	if len(clusters) == 0 {
		return nil, fmt.Errorf("no ECS clusters found in this region")
	}

	return clusters, nil
}

func selectCluster(clusters []string) (string, error) {
	if len(clusters) == 0 {
		return "", fmt.Errorf("no clusters available to select")
	}

	prompt := promptui.Select{
		Label: "Select ECS cluster",
		Items: clusters,
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}?",
			Active:   "➤ {{ . }}",
			Inactive: "  {{ . }}",
		},
	}

	_, clusterName, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("cluster selection failed: %w", err)
	}

	if clusterName == "" {
		return "", fmt.Errorf("selected cluster name is empty")
	}

	return clusterName, nil
}

// listTaskDefinitions lists the task definition ARNs that are actually used
// by services in the provided cluster. It lists services in the cluster,
// describes those services and collects their TaskDefinition ARNs, returning
// a deduplicated list.
func listTaskDefinitions(ctx context.Context, client *ecs.Client, clusterName string) ([]string, error) {
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name cannot be empty")
	}

	// 1) List services in the cluster
	var serviceArns []string
	listInput := &ecs.ListServicesInput{
		Cluster:    aws.String(clusterName),
		MaxResults: aws.Int32(100),
	}

	svcPaginator := ecs.NewListServicesPaginator(client, listInput)
	for svcPaginator.HasMorePages() {
		page, err := svcPaginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list services: %w", err)
		}
		serviceArns = append(serviceArns, page.ServiceArns...)
	}

	if len(serviceArns) == 0 {
		log.Printf("Info: No services found in cluster %s (cluster may be empty)", clusterName)
		return []string{}, nil
	}

	// 2) Describe services in batches and collect TaskDefinition ARNs
	taskDefSet := make(map[string]struct{})
	const batchSize = 10 // DescribeServices accepts up to 10 services per call
	for i := 0; i < len(serviceArns); i += batchSize {
		j := i + batchSize
		if j > len(serviceArns) {
			j = len(serviceArns)
		}
		batch := serviceArns[i:j]

		descInput := &ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterName),
			Services: batch,
		}

		descOutput, err := client.DescribeServices(ctx, descInput)
		if err != nil {
			return nil, fmt.Errorf("failed to describe services: %w", err)
		}

		// Handle service description failures
		if len(descOutput.Failures) > 0 {
			for _, failure := range descOutput.Failures {
				log.Printf("Warning: Failed to describe service %s: %s",
					aws.ToString(failure.Arn),
					aws.ToString(failure.Reason))
			}
		}

		for _, svc := range descOutput.Services {
			if svc.TaskDefinition == nil || *svc.TaskDefinition == "" {
				log.Printf("Warning: Service %s has empty task definition", aws.ToString(svc.ServiceArn))
				continue
			}
			taskDefSet[*svc.TaskDefinition] = struct{}{}
		}
	}

	// 3) Convert set to slice
	var taskDefs []string
	for arn := range taskDefSet {
		if arn == "" {
			log.Printf("Warning: Empty task definition ARN found in set, skipping")
			continue
		}
		taskDefs = append(taskDefs, arn)
	}

	if len(taskDefs) == 0 {
		log.Printf("Warning: No task definitions found for services in cluster %s", clusterName)
		return []string{}, nil
	}

	return taskDefs, nil
}

func getTaskDefinition(ctx context.Context, client *ecs.Client, taskDefArn string) (*types.TaskDefinition, error) {
	if taskDefArn == "" {
		return nil, fmt.Errorf("task definition ARN cannot be empty")
	}

	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefArn),
	}

	output, err := client.DescribeTaskDefinition(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe task definition %s: %w", taskDefArn, err)
	}

	if output.TaskDefinition == nil {
		return nil, fmt.Errorf("task definition %s returned nil from AWS API", taskDefArn)
	}

	return output.TaskDefinition, nil
}
