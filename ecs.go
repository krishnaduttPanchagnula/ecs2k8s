package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/manifoldco/promptui"
)

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

func listTaskDefinitions(ctx context.Context, client *ecs.Client, clusterName string) ([]string, error) {
	var taskDefs []string
	input := &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: aws.String(clusterName),
		MaxResults:   aws.Int32(100),
		Status:       types.TaskDefinitionStatusActive,
	}

	paginator := ecs.NewListTaskDefinitionsPaginator(client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		taskDefs = append(taskDefs, page.TaskDefinitionArns...)
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
