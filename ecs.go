package main

import (
	"context"
	"fmt"

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
			clusters = append(clusters, clusterName)
		}
	}

	if len(clusters) == 0 {
		return nil, fmt.Errorf("no ECS clusters found in this region")
	}

	return clusters, nil
}

func selectCluster(clusters []string) (string, error) {
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
	return clusterName, err
}

// listTaskDefinitions lists the task definition ARNs that are actually used
// by services in the provided cluster. It lists services in the cluster,
// describes those services and collects their TaskDefinition ARNs, returning
// a deduplicated list.
func listTaskDefinitions(ctx context.Context, client *ecs.Client, clusterName string) ([]string, error) {
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
			return nil, err
		}
		serviceArns = append(serviceArns, page.ServiceArns...)
	}

	if len(serviceArns) == 0 {
		return nil, fmt.Errorf("no services found in cluster %s", clusterName)
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
			return nil, err
		}

		for _, svc := range descOutput.Services {
			if svc.TaskDefinition != nil && *svc.TaskDefinition != "" {
				taskDefSet[*svc.TaskDefinition] = struct{}{}
			}
		}

		// Note: if there are failures, you can inspect descOutput.Failures to log or handle them.
	}

	// 3) Convert set to slice
	var taskDefs []string
	for arn := range taskDefSet {
		taskDefs = append(taskDefs, arn)
	}

	if len(taskDefs) == 0 {
		return nil, fmt.Errorf("no task definitions found for services in cluster %s", clusterName)
	}

	return taskDefs, nil
}

func getTaskDefinition(ctx context.Context, client *ecs.Client, taskDefArn string) (*types.TaskDefinition, error) {
	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefArn),
	}

	output, err := client.DescribeTaskDefinition(ctx, input)
	if err != nil {
		return nil, err
	}

	return output.TaskDefinition, nil
}
