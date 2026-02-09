// converter.go
package main

import (
	"fmt"
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

	container := taskDef.ContainerDefinitions[0] // Take first container

	// Deployment / PodSpec
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:  *container.Name,
			Image: *container.Image,
			Ports: convertPorts(container.PortMappings),
			Env:   convertEnvVars(container.Environment),
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"cpu":    cpuToQuantity(&container.Cpu),      // ECS CPU → millicores
					"memory": memoryToQuantity(container.Memory), // ECS MB → K8s Mi
				},
			},
		}},
	}

	manifests.Deployment = podSpec

	// ConfigMap for non-sensitive env vars
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-config", *container.Name),
		},
		Data: make(map[string]string),
	}
	for _, env := range container.Environment {
		if env.Value != nil && !strings.HasPrefix(*env.Name, "AWS") { // Skip AWS secrets
			configMap.Data[*env.Name] = *env.Value
		}
	}
	if len(configMap.Data) > 0 {
		manifests.ConfigMap = configMap
	}

	// Service (if port mappings exist)
	if len(container.PortMappings) > 0 {
		port := int32(8080) // Default
		if len(container.PortMappings) > 0 {
			port = *container.PortMappings[0].ContainerPort
		}
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: *container.Name,
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": *container.Name,
				},
				Ports: []corev1.ServicePort{{
					Port: port,
				}},
			},
		}
		manifests.Service = service
	}

	return manifests, nil
}

func convertPorts(portMappings []types.PortMapping) []corev1.ContainerPort {
	var ports []corev1.ContainerPort
	for _, pm := range portMappings {
		ports = append(ports, corev1.ContainerPort{
			ContainerPort: *pm.ContainerPort,
			Protocol:      corev1.ProtocolTCP,
		})
	}
	return ports
}

func convertEnvVars(envs []types.KeyValuePair) []corev1.EnvVar {
	var vars []corev1.EnvVar
	for _, env := range envs {
		vars = append(vars, corev1.EnvVar{
			Name:  *env.Name,
			Value: *env.Value,
		})
	}
	return vars
}

func cpuToQuantity(cpu *int32) resource.Quantity {
	if cpu == nil {
		return resource.MustParse("100m") // Default: 100 millicores
	}
	// ECS: 1 CPU unit = 1024 millicores (Kubernetes cores)
	cores := resource.NewMilliQuantity(int64(*cpu)*1024, resource.DecimalSI)
	return *cores
}

// NEW: Proper Memory conversion (ECS MB → Kubernetes Mi)
func memoryToQuantity(memory *int32) resource.Quantity {
	if memory == nil {
		return resource.MustParse("128Mi") // Default
	}
	// ECS Memory is in MB, Kubernetes uses MiB (very close, safe conversion)
	mib := resource.NewQuantity(int64(*memory)*1024*1024, resource.BinarySI)
	return *mib
}
