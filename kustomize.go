package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// KustomizeConfig represents the structure for kustomization.yaml
type KustomizeConfig struct {
	APIVersion   string                   `yaml:"apiVersion"`
	Kind         string                   `yaml:"kind"`
	Metadata     map[string]interface{}   `yaml:"metadata,omitempty"`
	Bases        []string                 `yaml:"bases,omitempty"`
	Resources    []string                 `yaml:"resources,omitempty"`
	Patches      []map[string]interface{} `yaml:"patches,omitempty"`
	Namespace    string                   `yaml:"namespace,omitempty"`
	Images       []map[string]interface{} `yaml:"images,omitempty"`
	CommonLabels map[string]string        `yaml:"commonLabels,omitempty"`
}

// NamespacePatch represents a namespace patch for overlays
type NamespacePatch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

// createKustomizeStructure creates a kustomize directory structure with base and overlays
func createKustomizeStructure(clusterName string, taskDefInfos []*TaskDefInfo, outputDir string) error {
	if !strings.Contains(outputDir, clusterName) {
		outputDir = filepath.Join(outputDir, clusterName)
	}

	kustomizeBasePath := filepath.Join(outputDir, "kustomize", clusterName, "base")
	overlaysPath := filepath.Join(outputDir, "kustomize", clusterName, "overlays")

	// Create directory structure
	directories := []string{
		kustomizeBasePath,
		filepath.Join(overlaysPath, "dev"),
		filepath.Join(overlaysPath, "staging"),
		filepath.Join(overlaysPath, "prod"),
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create kustomize directory %s: %w", dir, err)
		}
		log.Printf("Created Kustomize directory: %s", dir)
	}

	// Create base kustomization
	if err := createBaseKustomization(kustomizeBasePath, taskDefInfos); err != nil {
		return fmt.Errorf("failed to create base kustomization: %w", err)
	}

	// Create overlay kustomizations
	overlayNamespaces := map[string]string{
		"dev":     "development",
		"staging": "staging",
		"prod":    "production",
	}

	for overlayName, namespace := range overlayNamespaces {
		overlayPath := filepath.Join(overlaysPath, overlayName)
		if err := createOverlayKustomization(overlayPath, overlayName, namespace, taskDefInfos); err != nil {
			return fmt.Errorf("failed to create %s overlay: %w", overlayName, err)
		}
	}

	// Create root kustomization that can be used to build all overlays
	if err := createRootKustomization(filepath.Join(outputDir, "kustomize", clusterName), clusterName); err != nil {
		return fmt.Errorf("failed to create root kustomization: %w", err)
	}

	log.Printf("✓ Created Kustomize structure at: %s", filepath.Join(outputDir, "kustomize", clusterName))
	return nil
}

// createBaseKustomization creates the base kustomization.yaml and base manifests
func createBaseKustomization(basePath string, taskDefInfos []*TaskDefInfo) error {
	// Create subdirectories for different resource types
	resourceDirs := []string{
		filepath.Join(basePath, "deployments"),
		filepath.Join(basePath, "services"),
		filepath.Join(basePath, "configmaps"),
		filepath.Join(basePath, "secrets"),
		filepath.Join(basePath, "serviceaccounts"),
	}

	for _, dir := range resourceDirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create base subdirectory %s: %w", dir, err)
		}
	}

	// Write base manifests
	var resourceList []string

	for _, taskDefInfo := range taskDefInfos {
		taskName := taskDefInfo.Name

		// Write deployment
		deployment := generateBaseDeployment(taskName, taskDefInfo)
		deploymentFile := filepath.Join(basePath, "deployments", fmt.Sprintf("%s-deployment.yaml", taskName))
		if data, err := yaml.Marshal(deployment); err == nil {
			if err := os.WriteFile(deploymentFile, data, 0o644); err != nil {
				log.Printf("Warning: Failed to write deployment %s: %v", deploymentFile, err)
			} else {
				resourceList = append(resourceList, fmt.Sprintf("deployments/%s-deployment.yaml", taskName))
			}
		}

		// Write services
		if len(taskDefInfo.Manifests.Services) > 0 {
			for i, svc := range taskDefInfo.Manifests.Services {
				serviceFile := filepath.Join(basePath, "services", fmt.Sprintf("%s-service.yaml", svc.Name))
				if data, err := yaml.Marshal(svc); err == nil {
					if err := os.WriteFile(serviceFile, data, 0o644); err != nil {
						log.Printf("Warning: Failed to write service %s: %v", serviceFile, err)
					} else {
						if i == 0 {
							resourceList = append(resourceList, fmt.Sprintf("services/%s-service.yaml", svc.Name))
						}
					}
				}
			}
		}

		// Write configmaps
		if len(taskDefInfo.Manifests.ConfigMaps) > 0 {
			for i, cm := range taskDefInfo.Manifests.ConfigMaps {
				if cm == nil {
					continue
				}
				configmapFile := filepath.Join(basePath, "configmaps", fmt.Sprintf("%s-configmap-%d.yaml", taskName, i))
				if data, err := yaml.Marshal(cm); err == nil {
					if err := os.WriteFile(configmapFile, data, 0o644); err != nil {
						log.Printf("Warning: Failed to write configmap %s: %v", configmapFile, err)
					} else {
						resourceList = append(resourceList, fmt.Sprintf("configmaps/%s-configmap-%d.yaml", taskName, i))
					}
				}
			}
		}

		// Write secrets
		if len(taskDefInfo.Manifests.Secrets) > 0 {
			for i, secret := range taskDefInfo.Manifests.Secrets {
				if secret == nil {
					continue
				}
				secretFile := filepath.Join(basePath, "secrets", fmt.Sprintf("%s-secret-%d.yaml", taskName, i))
				if data, err := yaml.Marshal(secret); err == nil {
					if err := os.WriteFile(secretFile, data, 0o644); err != nil {
						log.Printf("Warning: Failed to write secret %s: %v", secretFile, err)
					} else {
						resourceList = append(resourceList, fmt.Sprintf("secrets/%s-secret-%d.yaml", taskName, i))
					}
				}
			}
		}

		// Write service accounts
		if taskDefInfo.Manifests.ServiceAccount != nil {
			serviceAccountFile := filepath.Join(basePath, "serviceaccounts", fmt.Sprintf("%s-serviceaccount.yaml", taskName))
			if data, err := yaml.Marshal(taskDefInfo.Manifests.ServiceAccount); err == nil {
				if err := os.WriteFile(serviceAccountFile, data, 0o644); err != nil {
					log.Printf("Warning: Failed to write serviceaccount %s: %v", serviceAccountFile, err)
				} else {
					resourceList = append(resourceList, fmt.Sprintf("serviceaccounts/%s-serviceaccount.yaml", taskName))
				}
			}
		}
	}

	// Create base kustomization.yaml
	baseKustomize := KustomizeConfig{
		APIVersion: "kustomize.config.k8s.io/v1beta1",
		Kind:       "Kustomization",
		Metadata: map[string]interface{}{
			"name": "base",
		},
		Resources: resourceList,
		CommonLabels: map[string]string{
			"managed-by": "ecs2k8s",
		},
	}

	kustomizeFile := filepath.Join(basePath, "kustomization.yaml")
	data, err := yaml.Marshal(baseKustomize)
	if err != nil {
		return fmt.Errorf("failed to marshal base kustomization: %w", err)
	}

	if err := os.WriteFile(kustomizeFile, data, 0o644); err != nil {
		return fmt.Errorf("failed to write base kustomization.yaml: %w", err)
	}

	log.Printf("Created base kustomization at: %s", kustomizeFile)
	return nil
}

// createOverlayKustomization creates overlay kustomization files for different environments
func createOverlayKustomization(overlayPath, overlayName, namespace string, taskDefInfos []*TaskDefInfo) error {
	// Create patches subdirectory
	patchesDir := filepath.Join(overlayPath, "patches")
	if err := os.MkdirAll(patchesDir, 0o755); err != nil {
		return fmt.Errorf("failed to create patches directory: %w", err)
	}

	// Create namespace patch for each deployment
	for _, taskDefInfo := range taskDefInfos {
		taskName := taskDefInfo.Name
		patchContent := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
spec:
  template:
    metadata:
      labels:
        environment: %s
`, taskName, namespace, overlayName)

		patchFile := filepath.Join(patchesDir, fmt.Sprintf("%s-namespace-patch.yaml", taskName))
		if err := os.WriteFile(patchFile, []byte(patchContent), 0o644); err != nil {
			log.Printf("Warning: Failed to write patch %s: %v", patchFile, err)
		}
	}

	// Create kustomization.yaml for overlay
	patches := make([]map[string]interface{}, 0)
	for _, taskDefInfo := range taskDefInfos {
		taskName := taskDefInfo.Name
		patches = append(patches, map[string]interface{}{
			"target": map[string]interface{}{
				"kind": "Deployment",
				"name": taskName,
			},
			"path": fmt.Sprintf("patches/%s-namespace-patch.yaml", taskName),
		})
	}

	overlayKustomize := KustomizeConfig{
		APIVersion: "kustomize.config.k8s.io/v1beta1",
		Kind:       "Kustomization",
		Metadata: map[string]interface{}{
			"name": overlayName,
		},
		Bases:     []string{"../../base"},
		Namespace: namespace,
		Patches:   patches,
		CommonLabels: map[string]string{
			"environment": overlayName,
		},
	}

	kustomizeFile := filepath.Join(overlayPath, "kustomization.yaml")
	data, err := yaml.Marshal(overlayKustomize)
	if err != nil {
		return fmt.Errorf("failed to marshal overlay kustomization: %w", err)
	}

	if err := os.WriteFile(kustomizeFile, data, 0o644); err != nil {
		return fmt.Errorf("failed to write overlay kustomization.yaml: %w", err)
	}

	log.Printf("Created %s overlay kustomization at: %s", overlayName, kustomizeFile)
	return nil
}

// createRootKustomization creates a root kustomization for managing all overlays
func createRootKustomization(rootPath, clusterName string) error {
	rootKustomize := KustomizeConfig{
		APIVersion: "kustomize.config.k8s.io/v1beta1",
		Kind:       "Kustomization",
		Metadata: map[string]interface{}{
			"name": clusterName,
		},
		CommonLabels: map[string]string{
			"cluster":    clusterName,
			"managed-by": "ecs2k8s",
		},
	}

	kustomizeFile := filepath.Join(rootPath, "kustomization.yaml")
	data, err := yaml.Marshal(rootKustomize)
	if err != nil {
		return fmt.Errorf("failed to marshal root kustomization: %w", err)
	}

	if err := os.WriteFile(kustomizeFile, data, 0o644); err != nil {
		return fmt.Errorf("failed to write root kustomization.yaml: %w", err)
	}

	log.Printf("Created root kustomization at: %s", kustomizeFile)
	return nil
}

// generateBaseDeployment creates a base deployment manifest
func generateBaseDeployment(taskName string, taskDefInfo *TaskDefInfo) map[string]interface{} {
	deployment := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name": taskName,
			"labels": map[string]string{
				"app": taskName,
			},
		},
		"spec": map[string]interface{}{
			"replicas": 1,
			"selector": map[string]interface{}{
				"matchLabels": map[string]string{
					"app": taskName,
				},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{
						"app": taskName,
					},
				},
				"spec": serializePodSpec(taskDefInfo.Manifests.Deployment),
			},
		},
	}

	return deployment
}

// CreateKustomizeChart is the main entry point for creating Kustomize structure
func CreateKustomizeChart(clusterName string, taskDefInfos []*TaskDefInfo, outputDir string) error {
	return createKustomizeStructure(clusterName, taskDefInfos, outputDir)
}
