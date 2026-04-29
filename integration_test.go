package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// TestGenerateAndValidateManifests generates manifests from a mock ECS task definition
// and writes them to a temp directory for validation with kubectl.
func TestGenerateAndValidateManifests(t *testing.T) {
	cpu := int32(512)
	memory := int32(1024)
	containerPort := int32(8080)

	taskDef := &types.TaskDefinition{
		TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789:task-definition/my-web-app:1"),
		ExecutionRoleArn:  aws.String("arn:aws:iam::123456789:role/ecsTaskExecutionRole"),
		TaskRoleArn:       aws.String("arn:aws:iam::123456789:role/myAppRole"),
		ContainerDefinitions: []types.ContainerDefinition{
			{
				Name:   aws.String("web"),
				Image:  aws.String("nginx:latest"),
				Cpu:    cpu,
				Memory: &memory,
				PortMappings: []types.PortMapping{
					{
						ContainerPort: &containerPort,
						Protocol:      types.TransportProtocolTcp,
					},
				},
				Environment: []types.KeyValuePair{
					{Name: aws.String("APP_ENV"), Value: aws.String("production")},
					{Name: aws.String("LOG_LEVEL"), Value: aws.String("info")},
					{Name: aws.String("AWS_REGION"), Value: aws.String("us-east-1")},
					{Name: aws.String("SECRET_KEY"), Value: aws.String("mysecret123")},
				},
			},
		},
	}

	manifests, err := convertTaskDefToK8s(taskDef)
	if err != nil {
		t.Fatalf("convertTaskDefToK8s failed: %v", err)
	}

	tmpDir := filepath.Join(os.TempDir(), "ecs2k8s-test-manifests")
	os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	taskDefName := "my-web-app"
	if err := writeManifests(tmpDir, taskDefName, manifests); err != nil {
		t.Fatalf("writeManifests failed: %v", err)
	}

	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		content, _ := os.ReadFile(filepath.Join(tmpDir, e.Name()))
		fmt.Printf("\n=== %s ===\n%s\n", e.Name(), string(content))
	}

	for _, c := range manifests.Deployment.Containers {
		memStr := c.Resources.Limits.Memory().String()
		t.Logf("Container %s: memory=%s, cpu=%s", c.Name, memStr, c.Resources.Limits.Cpu().String())
	}
}

// TestMultiContainerTaskDef tests a multi-container task definition
func TestMultiContainerTaskDef(t *testing.T) {
	cpu1 := int32(256)
	memory1 := int32(512)
	port1 := int32(8080)
	cpu2 := int32(128)
	memory2 := int32(256)
	port2 := int32(3000)

	taskDef := &types.TaskDefinition{
		TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789:task-definition/multi-app:1"),
		ContainerDefinitions: []types.ContainerDefinition{
			{
				Name:   aws.String("frontend"),
				Image:  aws.String("nginx:latest"),
				Cpu:    cpu1,
				Memory: &memory1,
				PortMappings: []types.PortMapping{
					{ContainerPort: &port1, Protocol: types.TransportProtocolTcp},
				},
				Environment: []types.KeyValuePair{
					{Name: aws.String("APP_NAME"), Value: aws.String("frontend")},
				},
			},
			{
				Name:   aws.String("backend"),
				Image:  aws.String("node:18-alpine"),
				Cpu:    cpu2,
				Memory: &memory2,
				PortMappings: []types.PortMapping{
					{ContainerPort: &port2, Protocol: types.TransportProtocolTcp},
				},
				Environment: []types.KeyValuePair{
					{Name: aws.String("APP_NAME"), Value: aws.String("backend")},
				},
			},
		},
	}

	manifests, err := convertTaskDefToK8s(taskDef)
	if err != nil {
		t.Fatalf("convertTaskDefToK8s failed: %v", err)
	}

	tmpDir := filepath.Join(os.TempDir(), "ecs2k8s-test-multi")
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0o755)

	if err := writeManifests(tmpDir, "multi-app", manifests); err != nil {
		t.Fatalf("writeManifests failed: %v", err)
	}

	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		content, _ := os.ReadFile(filepath.Join(tmpDir, e.Name()))
		fmt.Printf("\n=== %s ===\n%s\n", e.Name(), string(content))
	}
}

// TestHelmChartGeneration tests generating a Helm chart
func TestHelmChartGeneration(t *testing.T) {
	cpu := int32(512)
	memory := int32(1024)
	port := int32(8080)

	taskDef := &types.TaskDefinition{
		TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789:task-definition/api-service:3"),
		TaskRoleArn:       aws.String("arn:aws:iam::123456789:role/apiServiceRole"),
		ContainerDefinitions: []types.ContainerDefinition{
			{
				Name:   aws.String("api"),
				Image:  aws.String("myrepo/api-service:v2.1.0"),
				Cpu:    cpu,
				Memory: &memory,
				PortMappings: []types.PortMapping{
					{ContainerPort: &port, Protocol: types.TransportProtocolTcp},
				},
				Environment: []types.KeyValuePair{
					{Name: aws.String("APP_ENV"), Value: aws.String("production")},
					{Name: aws.String("DB_HOST"), Value: aws.String("mydb.cluster.rds.amazonaws.com")},
					{Name: aws.String("SECRET_DB_PASSWORD"), Value: aws.String("s3cur3p@ss")},
				},
			},
		},
	}

	manifests, err := convertTaskDefToK8s(taskDef)
	if err != nil {
		t.Fatalf("convertTaskDefToK8s failed: %v", err)
	}

	taskDefName := "api-service"
	taskDefInfo, err := convertTaskDefToInfo(taskDef, taskDefName)
	if err != nil {
		t.Fatalf("convertTaskDefToInfo failed: %v", err)
	}
	taskDefInfo.Manifests = manifests

	tmpDir := filepath.Join(os.TempDir(), "ecs2k8s-test-helm")
	os.RemoveAll(tmpDir)
	os.MkdirAll(filepath.Join(tmpDir, "my-cluster"), 0o755)

	clusterOutputDir := filepath.Join(tmpDir, "my-cluster")
	if err := writeManifests(clusterOutputDir, taskDefName, manifests); err != nil {
		t.Fatalf("writeManifests failed: %v", err)
	}

	if err := CreateHelmChart("my-cluster", []*TaskDefInfo{taskDefInfo}, tmpDir); err != nil {
		t.Fatalf("CreateHelmChart failed: %v", err)
	}

	// Print Helm chart structure
	filepath.Walk(filepath.Join(tmpDir, "my-cluster", "helm"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(tmpDir, path)
		if info.IsDir() {
			fmt.Printf("%s/\n", rel)
		} else {
			content, _ := os.ReadFile(path)
			fmt.Printf("\n=== %s ===\n%s\n", rel, string(content))
		}
		return nil
	})
}

// TestKustomizeGeneration tests generating Kustomize structure
func TestKustomizeGeneration(t *testing.T) {
	cpu := int32(256)
	memory := int32(512)
	port := int32(3000)

	taskDef := &types.TaskDefinition{
		TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789:task-definition/worker-service:2"),
		ContainerDefinitions: []types.ContainerDefinition{
			{
				Name:   aws.String("worker"),
				Image:  aws.String("myrepo/worker:latest"),
				Cpu:    cpu,
				Memory: &memory,
				PortMappings: []types.PortMapping{
					{ContainerPort: &port, Protocol: types.TransportProtocolTcp},
				},
				Environment: []types.KeyValuePair{
					{Name: aws.String("QUEUE_URL"), Value: aws.String("https://sqs.us-east-1.amazonaws.com/123456789/my-queue")},
				},
			},
		},
	}

	manifests, err := convertTaskDefToK8s(taskDef)
	if err != nil {
		t.Fatalf("convertTaskDefToK8s failed: %v", err)
	}

	taskDefName := "worker-service"
	taskDefInfo, err := convertTaskDefToInfo(taskDef, taskDefName)
	if err != nil {
		t.Fatalf("convertTaskDefToInfo failed: %v", err)
	}
	taskDefInfo.Manifests = manifests

	tmpDir := filepath.Join(os.TempDir(), "ecs2k8s-test-kustomize")
	os.RemoveAll(tmpDir)
	os.MkdirAll(filepath.Join(tmpDir, "my-cluster"), 0o755)

	if err := CreateKustomizeChart("my-cluster", []*TaskDefInfo{taskDefInfo}, tmpDir); err != nil {
		t.Fatalf("CreateKustomizeChart failed: %v", err)
	}

	filepath.Walk(filepath.Join(tmpDir, "my-cluster", "kustomize"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(tmpDir, path)
		if info.IsDir() {
			fmt.Printf("%s/\n", rel)
		} else {
			content, _ := os.ReadFile(path)
			fmt.Printf("\n=== %s ===\n%s\n", rel, string(content))
		}
		return nil
	})
}
