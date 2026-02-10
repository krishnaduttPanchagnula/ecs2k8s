package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ServiceAccountConfig holds configuration for creating a ServiceAccount with IRSA
type ServiceAccountConfig struct {
	Name           string
	Namespace      string
	RoleARN        string
	Annotations    map[string]string
}

// ContainerResources tracks resource requirements for a container
type ContainerResources struct {
	Name   string
	CPU    string
	Memory string
	Ports  []int32
}

type K8sManifests struct {
	Deployment     *corev1.PodSpec       `json:"deployment,omitempty"`
	ConfigMaps     []*corev1.ConfigMap   `json:"configmaps,omitempty"`
	Secrets        []*corev1.Secret      `json:"secrets,omitempty"`
	Services       []*corev1.Service     `json:"services,omitempty"`
	ServiceAccount *corev1.ServiceAccount `json:"serviceaccount,omitempty"`
	Containers     []ContainerResources  `json:"containers,omitempty"`
}

// TaskDefInfo represents a task definition with its converted K8s manifests
type TaskDefInfo struct {
	Name            string
	Image           string
	Containers      []ContainerConfig
	Manifests       K8sManifests
	ExecutionRoleArn string
	TaskRoleArn     string
}

// ContainerConfig represents configuration for a single container
type ContainerConfig struct {
	Name    string
	Image   string
	CPU     string
	Memory  string
	Ports   []int32
	EnvVars map[string]string
}

func convertTaskDefToK8s(taskDef *types.TaskDefinition) (K8sManifests, error) {
	manifests := K8sManifests{}

	if taskDef.ContainerDefinitions == nil || len(taskDef.ContainerDefinitions) == 0 {
		return manifests, fmt.Errorf("no container definitions found")
	}

	// Log info about container count
	if len(taskDef.ContainerDefinitions) > 1 {
		log.Printf("Info: Task definition has %d containers, converting all", len(taskDef.ContainerDefinitions))
	}

	var containers []corev1.Container
	var containerResources []ContainerResources
	var configMaps []*corev1.ConfigMap
	var secrets []*corev1.Secret
	var services []*corev1.Service
	var serviceAccount *corev1.ServiceAccount

	for i, container := range taskDef.ContainerDefinitions {
		// Validate required fields
		if container.Name == nil || *container.Name == "" {
			log.Printf("Warning: Container %d missing Name field, skipping", i)
			continue
		}
		if container.Image == nil || *container.Image == "" {
			log.Printf("Warning: Container %s missing Image field, skipping", *container.Name)
			continue
		}

		containerName := *container.Name

		// Convert ports
		ports := convertPorts(container.PortMappings)

		// Convert environment variables
		envVars := convertEnvVars(container.Environment)

		// Convert resources
		// Note: Cpu is int32 (not a pointer), Memory is *int32
		cpuVal := container.Cpu
		cpuQty := cpuToQuantity(&cpuVal)
		memoryQty := memoryToQuantity(container.Memory)

		// Create container spec
		c := corev1.Container{
			Name:  containerName,
			Image: *container.Image,
			Ports: ports,
			Env:   envVars,
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    cpuQty,
					corev1.ResourceMemory: memoryQty,
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    cpuQty,
					corev1.ResourceMemory: memoryQty,
				},
			},
		}
		containers = append(containers, c)

		// Track container resources for Helm values
		portList := make([]int32, 0)
		for _, p := range ports {
			portList = append(portList, p.ContainerPort)
		}

		resources := ContainerResources{
			Name:   containerName,
			CPU:    cpuQty.String(),
			Memory: memoryQty.String(),
			Ports:  portList,
		}
		containerResources = append(containerResources, resources)

		// Create ConfigMap for non-sensitive env vars
		if cm := createConfigMap(containerName, container.Environment); cm != nil {
			configMaps = append(configMaps, cm)
		}

		// Create Secret for AWS/sensitive env vars
		if secret := createSecret(containerName, container.Environment); secret != nil {
			secrets = append(secrets, secret)
		}

		// Create Service for this container if it has port mappings
		if len(container.PortMappings) > 0 {
			if svc := createService(containerName, container.PortMappings); svc != nil {
				services = append(services, svc)
			}
		}
	}

	if len(containers) == 0 {
		return manifests, fmt.Errorf("no valid containers to convert")
	}

	// Create PodSpec with all containers
	podSpec := &corev1.PodSpec{
		Containers: containers,
	}

	// Create ServiceAccount for image pull and IAM role support
	if serviceAccount = createServiceAccount("", taskDef.TaskRoleArn, taskDef.ExecutionRoleArn); serviceAccount != nil {
		// Attach ServiceAccount to PodSpec
		podSpec.ServiceAccountName = serviceAccount.Name
	}

	manifests.Deployment = podSpec
	manifests.ConfigMaps = configMaps
	manifests.Secrets = secrets
	manifests.Services = services
	manifests.ServiceAccount = serviceAccount
	manifests.Containers = containerResources

	return manifests, nil
}

// createServiceAccount creates a Kubernetes ServiceAccount with IRSA annotations
// If taskRoleArn is provided, it's preferred over executionRoleArn
func createServiceAccount(taskDefName string, taskRoleArn, executionRoleArn *string) *corev1.ServiceAccount {
	if taskDefName == "" {
		taskDefName = "default"
	}

	saName := fmt.Sprintf("%s-sa", taskDefName)

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: "default",
		},
	}

	// Prefer task role ARN (application permissions) over execution role ARN (ECS task execution permissions)
	var roleARN string
	if taskRoleArn != nil && *taskRoleArn != "" {
		roleARN = *taskRoleArn
		log.Printf("Info: Using ECS Task Role ARN for ServiceAccount: %s", roleARN)
	} else if executionRoleArn != nil && *executionRoleArn != "" {
		roleARN = *executionRoleArn
		log.Printf("Info: Using ECS Execution Role ARN for ServiceAccount: %s", roleARN)
	}

	// Only create ServiceAccount if we have a role ARN
	if roleARN != "" {
		if sa.Annotations == nil {
			sa.Annotations = make(map[string]string)
		}
		// Add IRSA annotation for EKS to associate IAM role with ServiceAccount
		sa.Annotations["eks.amazonaws.com/role-arn"] = roleARN
		log.Printf("✓ Created ServiceAccount %s with IRSA annotation for role: %s", saName, roleARN)
		return sa
	}

	log.Printf("Info: No ECS IAM role found for task definition %s, ServiceAccount will not have IRSA annotation", taskDefName)
	// Return ServiceAccount anyway - it can still be used for basic configuration
	return sa
}

// createImagePullSecret creates an image pull secret for private registries (optional helper)
func createImagePullSecret(secretName, registryURL, username, password, email string) *corev1.Secret {
	if secretName == "" || registryURL == "" {
		return nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}

	// Note: In practice, you'd construct a proper .dockerconfigjson here
	// For now, this is a placeholder
	log.Printf("Info: Image pull secret creation for %s would require registry credentials", registryURL)
	return secret
}

func createConfigMap(containerName string, envVars []types.KeyValuePair) *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-config", containerName),
		},
		Data: make(map[string]string),
	}

	for _, env := range envVars {
		if env.Name == nil || *env.Name == "" {
			log.Printf("Warning: Environment variable has empty Name, skipping")
			continue
		}
		if env.Value == nil {
			log.Printf("Warning: Environment variable %s has nil Value, skipping", *env.Name)
			continue
		}

		// Include non-sensitive env vars (exclude AWS and common secret prefixes)
		if !isSecretEnvVar(*env.Name) {
			configMap.Data[*env.Name] = *env.Value
		}
	}

	if len(configMap.Data) > 0 {
		return configMap
	}

	log.Printf("Info: No non-sensitive environment variables to include in ConfigMap for %s", containerName)
	return nil
}

func createSecret(containerName string, envVars []types.KeyValuePair) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-secret", containerName),
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: make(map[string]string),
	}

	for _, env := range envVars {
		if env.Name == nil || *env.Name == "" {
			continue
		}
		if env.Value == nil {
			continue
		}

		// Include sensitive/AWS env vars
		if isSecretEnvVar(*env.Name) {
			secret.StringData[*env.Name] = *env.Value
		}
	}

	if len(secret.StringData) > 0 {
		return secret
	}

	log.Printf("Info: No sensitive environment variables to include in Secret for %s", containerName)
	return nil
}

func isSecretEnvVar(name string) bool {
	secretPrefixes := []string{
		"AWS",
		"SECRET",
		"PASSWORD",
		"TOKEN",
		"KEY",
		"PRIVATE",
		"ACCESS",
		"AUTH",
		"CERT",
	}

	for _, prefix := range secretPrefixes {
		if strings.HasPrefix(strings.ToUpper(name), prefix) {
			return true
		}
	}
	return false
}

func createService(containerName string, portMappings []types.PortMapping) *corev1.Service {
	if len(portMappings) == 0 {
		return nil
	}

	// Collect all valid ports
	var servicePorts []corev1.ServicePort
	var primaryPort int32 = 8080

	for i, pm := range portMappings {
		if pm.ContainerPort == nil {
			continue
		}

		port := *pm.ContainerPort
		if port < 1 || port > 65535 {
			log.Printf("Warning: Invalid port number %d for container %s, skipping", port, containerName)
			continue
		}

		// Use first valid port as primary
		if i == 0 || primaryPort == 8080 {
			primaryPort = port
		}

		servicePorts = append(servicePorts, corev1.ServicePort{
			Port:       port,
			TargetPort: intstr.FromInt32(port),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	if len(servicePorts) == 0 {
		return nil
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: containerName,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": containerName,
			},
			Ports: servicePorts,
			Type:  corev1.ServiceTypeClusterIP,
		},
	}

	return service
}

func convertPorts(portMappings []types.PortMapping) []corev1.ContainerPort {
	var ports []corev1.ContainerPort
	for _, pm := range portMappings {
		if pm.ContainerPort == nil {
			log.Printf("Warning: Port mapping has nil ContainerPort, skipping")
			continue
		}

		port := *pm.ContainerPort
		if port < 1 || port > 65535 {
			log.Printf("Warning: Invalid port number %d (must be 1-65535), skipping", port)
			continue
		}

		ports = append(ports, corev1.ContainerPort{
			ContainerPort: port,
			Protocol:      corev1.ProtocolTCP,
		})
	}
	return ports
}

func convertEnvVars(envs []types.KeyValuePair) []corev1.EnvVar {
	var vars []corev1.EnvVar
	for _, env := range envs {
		if env.Name == nil || *env.Name == "" {
			log.Printf("Warning: Environment variable has empty Name, skipping")
			continue
		}
		if env.Value == nil {
			log.Printf("Warning: Environment variable %s has nil Value, skipping", *env.Name)
			continue
		}

		vars = append(vars, corev1.EnvVar{
			Name:  *env.Name,
			Value: *env.Value,
		})
	}
	return vars
}

func cpuToQuantity(cpu *int32) resource.Quantity {
	if cpu == nil || *cpu <= 0 {
		log.Printf("Warning: Invalid or missing CPU value, using default 100m")
		return resource.MustParse("100m")
	}

	cpuVal := *cpu

	// Cap at reasonable maximum (16 cores = 16000 millicores)
	if cpuVal > 16000 {
		log.Printf("Warning: CPU value %d exceeds reasonable maximum (16000m), capping at 16000m", cpuVal)
		return resource.MustParse("16000m")
	}

	// ECS CPU units directly map to millicores
	cores := resource.NewMilliQuantity(int64(cpuVal), resource.DecimalSI)
	return *cores
}

func memoryToQuantity(memory *int32) resource.Quantity {
	if memory == nil || *memory <= 0 {
		log.Printf("Warning: Invalid or missing memory value, using default 128Mi")
		return resource.MustParse("128Mi")
	}

	memVal := *memory

	// Cap at reasonable maximum (256GB)
	if memVal > 262144 {
		log.Printf("Warning: Memory value %d exceeds reasonable maximum (262144 MB = 256GB), capping at 256Gi", memVal)
		return resource.MustParse("256Gi")
	}

	// ECS Memory is in MB, Kubernetes uses MiB
	mib := resource.NewQuantity(int64(memVal)*1024*1024, resource.BinarySI)
	return *mib
}

// convertTaskDefToInfo converts an ECS task definition to TaskDefInfo
func convertTaskDefToInfo(taskDef *types.TaskDefinition, taskDefName string) (*TaskDefInfo, error) {
	if taskDef == nil {
		return nil, fmt.Errorf("task definition cannot be nil")
	}

	taskDefInfo := &TaskDefInfo{
		Name:            taskDefName,
		Containers:      []ContainerConfig{},
		ExecutionRoleArn: "",
		TaskRoleArn:      "",
	}

	// Capture IAM role ARNs
	if taskDef.ExecutionRoleArn != nil {
		taskDefInfo.ExecutionRoleArn = *taskDef.ExecutionRoleArn
	}
	if taskDef.TaskRoleArn != nil {
		taskDefInfo.TaskRoleArn = *taskDef.TaskRoleArn
	}

	for _, container := range taskDef.ContainerDefinitions {
		if container.Name == nil || *container.Name == "" {
			continue
		}

		image := ""
		if container.Image != nil {
			image = *container.Image
		}

		cpu := ""
		if container.Cpu > 0 {
			cpuVal := container.Cpu
			cpuQty := cpuToQuantity(&cpuVal)
			cpu = cpuQty.String()
		}

		memory := ""
		if container.Memory != nil && *container.Memory > 0 {
			memQty := memoryToQuantity(container.Memory)
			memory = memQty.String()
		}

		// Extract ports
		var ports []int32
		for _, pm := range container.PortMappings {
			if pm.ContainerPort != nil {
				ports = append(ports, *pm.ContainerPort)
			}
		}

		// Extract environment variables
		envVars := make(map[string]string)
		for _, env := range container.Environment {
			if env.Name != nil && env.Value != nil {
				envVars[*env.Name] = *env.Value
			}
		}

		containerConfig := ContainerConfig{
			Name:    *container.Name,
			Image:   image,
			CPU:     cpu,
			Memory:  memory,
			Ports:   ports,
			EnvVars: envVars,
		}

		taskDefInfo.Containers = append(taskDefInfo.Containers, containerConfig)
	}

	if len(taskDef.ContainerDefinitions) > 0 && taskDef.ContainerDefinitions[0].Image != nil {
		taskDefInfo.Image = *taskDef.ContainerDefinitions[0].Image
	}

	return taskDefInfo, nil
}
