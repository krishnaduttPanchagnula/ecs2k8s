// converter.go
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type K8sManifests struct {
	Deployment *corev1.PodSpec   `json:"deployment,omitempty"`
	ConfigMap  *corev1.ConfigMap `json:"configmap,omitempty"`
	Secret     *corev1.Secret    `json:"secret,omitempty"`
	Service    *corev1.Service   `json:"service,omitempty"`
}

func convertTaskDefToK8s(taskDef *types.TaskDefinition) (K8sManifests, error) {
	manifests := K8sManifests{}

	if taskDef.ContainerDefinitions == nil || len(taskDef.ContainerDefinitions) == 0 {
		return manifests, fmt.Errorf("no container definitions found")
	}

	// Log warning if multiple containers exist
	if len(taskDef.ContainerDefinitions) > 1 {
		log.Printf("Warning: Task definition has %d containers, converting all", len(taskDef.ContainerDefinitions))
	}

	var containers []corev1.Container
	var configMaps []*corev1.ConfigMap
	var services []*corev1.Service

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

		c := corev1.Container{
			Name:  *container.Name,
			Image: *container.Image,
			Ports: convertPorts(container.PortMappings),
			Env:   convertEnvVars(container.Environment),
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"cpu":    cpuToQuantity(&container.Cpu),
					"memory": memoryToQuantity(container.Memory),
				},
			},
		}
		containers = append(containers, c)

		// Create ConfigMap for this container
		cm := createConfigMap(*container.Name, container.Environment)
		if cm != nil {
			configMaps = append(configMaps, cm)
		}

		// Create Service for this container if it has port mappings
		if len(container.PortMappings) > 0 {
			svc := createService(*container.Name, container.PortMappings)
			if svc != nil {
				services = append(services, svc)
			}
		}
	}

	if len(containers) == 0 {
		return manifests, fmt.Errorf("no valid containers to convert")
	}

	// Deployment / PodSpec
	podSpec := &corev1.PodSpec{
		Containers: containers,
	}
	manifests.Deployment = podSpec

	// Use the first ConfigMap if available
	if len(configMaps) > 0 {
		manifests.ConfigMap = configMaps[0]
	}

	// Use the first Service if available
	if len(services) > 0 {
		manifests.Service = services[0]
	}

	return manifests, nil
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

		// Skip AWS secrets
		if !strings.HasPrefix(*env.Name, "AWS") {
			configMap.Data[*env.Name] = *env.Value
		}
	}

	if len(configMap.Data) > 0 {
		return configMap
	}

	log.Printf("Info: No non-AWS environment variables to include in ConfigMap for %s", containerName)
	return nil
}

func createService(containerName string, portMappings []types.PortMapping) *corev1.Service {
	if len(portMappings) == 0 {
		return nil
	}

	// Get the first valid port
	var servicePort int32 = 8080 // Default

	for _, pm := range portMappings {
		if pm.ContainerPort != nil && *pm.ContainerPort > 0 && *pm.ContainerPort <= 65535 {
			servicePort = *pm.ContainerPort
			break
		}
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: containerName,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": containerName,
			},
			Ports: []corev1.ServicePort{{
				Port: servicePort,
			}},
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

	// Cap at reasonable maximum (e.g., 16 cores = 16000 millicores)
	if *cpu > 16000 {
		log.Printf("Warning: CPU value %d exceeds reasonable maximum (16000m), capping at 16000m", *cpu)
		return resource.MustParse("16000m")
	}

	// ECS: 1 CPU unit = 1024 millicores (Kubernetes cores)
	cores := resource.NewMilliQuantity(int64(*cpu), resource.DecimalSI)
	return *cores
}

func memoryToQuantity(memory *int32) resource.Quantity {
	if memory == nil || *memory <= 0 {
		log.Printf("Warning: Invalid or missing memory value, using default 128Mi")
		return resource.MustParse("128Mi")
	}

	// Cap at reasonable maximum (e.g., 256GB = 262144 MB)
	if *memory > 262144 {
		log.Printf("Warning: Memory value %d exceeds reasonable maximum (256Gi), capping at 256Gi", *memory)
		return resource.MustParse("256Gi")
	}

	// ECS Memory is in MB, Kubernetes uses MiB (very close, safe conversion)
	mib := resource.NewQuantity(int64(*memory)*1024*1024, resource.BinarySI)
	return *mib
}
