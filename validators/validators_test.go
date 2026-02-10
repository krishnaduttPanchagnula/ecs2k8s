package validators

import (
	"testing"
)

// TestRegionValidatorFormat tests region format validation
func TestRegionValidatorFormat(t *testing.T) {
	tests := []struct {
		name    string
		region  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid region",
			region:  "us-east-1",
			wantErr: false,
		},
		{
			name:    "valid region eu-west-1",
			region:  "eu-west-1",
			wantErr: false,
		},
		{
			name:    "empty region",
			region:  "",
			wantErr: true,
			errMsg:  "region cannot be empty",
		},
		{
			name:    "region with spaces",
			region:  "  us-east-1  ",
			wantErr: false,
		},
		{
			name:    "invalid format no hyphens",
			region:  "useast1",
			wantErr: true,
			errMsg:  "invalid region format",
		},
		{
			name:    "invalid format too many parts",
			region:  "us-east-1-extra",
			wantErr: true,
			errMsg:  "invalid region format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rv := &RegionValidator{Region: tt.region}
			err := rv.ValidateFormat()

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFormat() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateFormat() error = %v, want %s", err, tt.errMsg)
				}
			}
		})
	}
}

// TestRegionValidatorKnownRegion tests known region validation
func TestRegionValidatorKnownRegion(t *testing.T) {
	tests := []struct {
		name    string
		region  string
		wantErr bool
	}{
		{
			name:    "known region us-east-1",
			region:  "us-east-1",
			wantErr: false,
		},
		{
			name:    "known region eu-west-1",
			region:  "eu-west-1",
			wantErr: false,
		},
		{
			name:    "known region ap-northeast-1",
			region:  "ap-northeast-1",
			wantErr: false,
		},
		{
			name:    "unknown region",
			region:  "unknown-region-1",
			wantErr: true,
		},
		{
			name:    "made up region",
			region:  "xx-yyyy-1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rv := &RegionValidator{Region: tt.region}
			err := rv.ValidateKnownRegion()

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKnownRegion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestClusterValidatorName tests cluster name validation
func TestClusterValidatorName(t *testing.T) {
	tests := []struct {
		name       string
		clusterArg string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "valid cluster name",
			clusterArg: "my-cluster",
			wantErr:    false,
		},
		{
			name:       "valid cluster name with underscores",
			clusterArg: "my_cluster",
			wantErr:    false,
		},
		{
			name:       "valid cluster name with numbers",
			clusterArg: "cluster123",
			wantErr:    false,
		},
		{
			name:       "empty cluster name",
			clusterArg: "",
			wantErr:    true,
			errMsg:     "cluster name cannot be empty",
		},
		{
			name:       "whitespace only",
			clusterArg: "   ",
			wantErr:    true,
			errMsg:     "cannot be empty or whitespace only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv := &ClusterValidator{ClusterName: tt.clusterArg}
			err := cv.ValidateName()

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateName() error = %v, want to contain %s", err, tt.errMsg)
				}
			}
		})
	}
}

// TestClusterValidatorFormat tests cluster format validation
func TestClusterValidatorFormat(t *testing.T) {
	tests := []struct {
		name       string
		clusterArg string
		wantErr    bool
	}{
		{
			name:       "valid format",
			clusterArg: "my-cluster",
			wantErr:    false,
		},
		{
			name:       "valid format with underscores",
			clusterArg: "my_cluster_name",
			wantErr:    false,
		},
		{
			name:       "valid format with numbers",
			clusterArg: "cluster-123",
			wantErr:    false,
		},
		{
			name:       "invalid format with spaces",
			clusterArg: "my cluster",
			wantErr:    true,
		},
		{
			name:       "invalid format with special chars",
			clusterArg: "my@cluster!",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv := &ClusterValidator{ClusterName: tt.clusterArg}
			err := cv.ValidateFormat()

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestTaskDefinitionValidatorName tests task definition name validation
func TestTaskDefinitionValidatorName(t *testing.T) {
	tests := []struct {
		name      string
		taskDefID string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid task definition name",
			taskDefID: "my-task",
			wantErr:   false,
		},
		{
			name:      "valid ARN format",
			taskDefID: "arn:aws:ecs:us-east-1:123456789:task-definition/my-task:1",
			wantErr:   false,
		},
		{
			name:      "empty task definition",
			taskDefID: "",
			wantErr:   true,
			errMsg:    "task definition ARN cannot be empty",
		},
		{
			name:      "whitespace only",
			taskDefID: "   ",
			wantErr:   true,
			errMsg:    "cannot be empty or whitespace only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv := &TaskDefinitionValidator{TaskDefARN: tt.taskDefID}
			err := tv.ValidateName()

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateName() error = %v, want to contain %s", err, tt.errMsg)
				}
			}
		})
	}
}

// TestTaskDefinitionValidatorARNFormat tests task definition ARN format validation
func TestTaskDefinitionValidatorARNFormat(t *testing.T) {
	tests := []struct {
		name      string
		taskDefID string
		wantErr   bool
	}{
		{
			name:      "valid ARN format",
			taskDefID: "arn:aws:ecs:us-east-1:123456789:task-definition/my-task:1",
			wantErr:   false,
		},
		{
			name:      "valid task name",
			taskDefID: "my-task",
			wantErr:   false,
		},
		{
			name:      "valid task name with numbers",
			taskDefID: "my-task-123",
			wantErr:   false,
		},
		{
			name:      "invalid format with spaces",
			taskDefID: "my task def",
			wantErr:   true,
		},
		{
			name:      "invalid format with special chars",
			taskDefID: "my@task!",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv := &TaskDefinitionValidator{TaskDefARN: tt.taskDefID}
			err := tv.ValidateARNFormat()

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateARNFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestManifestValidatorPath tests manifest path validation
func TestManifestValidatorPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid path",
			path:    "/tmp/manifest.yaml",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mv := &ManifestValidator{ManifestPath: tt.path}
			err := mv.ValidatePath()

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestManifestValidatorYAML tests manifest YAML validation
func TestManifestValidatorYAML(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid deployment YAML",
			content: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test`,
			wantErr: false,
		},
		{
			name: "valid service YAML",
			content: `apiVersion: v1
kind: Service
metadata:
  name: test`,
			wantErr: false,
		},
		{
			name:    "empty content",
			content: "",
			wantErr: true,
			errMsg:  "manifest content cannot be empty",
		},
		{
			name: "missing apiVersion",
			content: `kind: Deployment
metadata:
  name: test`,
			wantErr: true,
			errMsg:  "missing apiVersion field",
		},
		{
			name: "missing kind",
			content: `apiVersion: v1
metadata:
  name: test`,
			wantErr: true,
			errMsg:  "missing kind field",
		},
		{
			name: "missing metadata",
			content: `apiVersion: v1
kind: Deployment`,
			wantErr: true,
			errMsg:  "missing metadata field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mv := &ManifestValidator{Content: []byte(tt.content)}
			err := mv.ValidateYAML()

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateYAML() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateYAML() error = %v, want to contain %s", err, tt.errMsg)
				}
			}
		})
	}
}

// TestManifestValidatorKubernetesKind tests Kubernetes kind validation
func TestManifestValidatorKubernetesKind(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "valid Deployment kind",
			content: `apiVersion: v1
kind: Deployment
metadata:
  name: test`,
			wantErr: false,
		},
		{
			name: "valid Service kind",
			content: `apiVersion: v1
kind: Service
metadata:
  name: test`,
			wantErr: false,
		},
		{
			name: "valid ConfigMap kind",
			content: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test`,
			wantErr: false,
		},
		{
			name: "valid Secret kind",
			content: `apiVersion: v1
kind: Secret
metadata:
  name: test`,
			wantErr: false,
		},
		{
			name: "invalid kind",
			content: `apiVersion: v1
kind: InvalidKind
metadata:
  name: test`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mv := &ManifestValidator{Content: []byte(tt.content)}
			err := mv.ValidateKubernetesKind()

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKubernetesKind() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestManifestValidatorValidate tests complete manifest validation
func TestManifestValidatorValidate(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		content string
		wantErr bool
	}{
		{
			name: "valid manifest",
			path: "/tmp/deployment.yaml",
			content: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test`,
			wantErr: false,
		},
		{
			name: "empty path",
			path: "",
			content: `apiVersion: v1
kind: Service`,
			wantErr: true,
		},
		{
			name:    "empty content",
			path:    "/tmp/deployment.yaml",
			content: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mv := &ManifestValidator{
				ManifestPath: tt.path,
				Content:      []byte(tt.content),
			}
			err := mv.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidAWSRegionsMap tests that ValidAWSRegions map is populated
func TestValidAWSRegionsMap(t *testing.T) {
	if len(ValidAWSRegions) == 0 {
		t.Error("ValidAWSRegions map is empty")
	}

	expectedRegions := []string{
		"us-east-1",
		"us-east-2",
		"us-west-1",
		"us-west-2",
		"eu-west-1",
		"eu-central-1",
		"ap-northeast-1",
	}

	for _, region := range expectedRegions {
		if !ValidAWSRegions[region] {
			t.Errorf("Expected region %s not found in ValidAWSRegions map", region)
		}
	}
}

// TestHelperFunctions tests helper functions
func TestIsValidClusterName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid cluster name",
			input:   "my-cluster",
			wantErr: false,
		},
		{
			name:    "valid with underscores",
			input:   "my_cluster",
			wantErr: false,
		},
		{
			name:    "valid with numbers",
			input:   "cluster123",
			wantErr: false,
		},
		{
			name:    "empty",
			input:   "",
			wantErr: true,
		},
		{
			name:    "with spaces",
			input:   "my cluster",
			wantErr: true,
		},
		{
			name:    "with special chars",
			input:   "my@cluster!",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidClusterName(tt.input)
			if (result == false) != tt.wantErr {
				t.Errorf("isValidClusterName() = %v, want %v", result, !tt.wantErr)
			}
		})
	}
}

func TestIsValidTaskDefName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid task def name",
			input:   "my-task",
			wantErr: false,
		},
		{
			name:    "valid with underscores",
			input:   "my_task",
			wantErr: false,
		},
		{
			name:    "valid with numbers",
			input:   "task123",
			wantErr: false,
		},
		{
			name:    "empty",
			input:   "",
			wantErr: true,
		},
		{
			name:    "with spaces",
			input:   "my task",
			wantErr: true,
		},
		{
			name:    "with special chars",
			input:   "my@task!",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidTaskDefName(tt.input)
			if (result == false) != tt.wantErr {
				t.Errorf("isValidTaskDefName() = %v, want %v", result, !tt.wantErr)
			}
		})
	}
}

// Benchmark tests

// BenchmarkRegionValidatorFormat benchmarks region format validation
func BenchmarkRegionValidatorFormat(b *testing.B) {
	rv := &RegionValidator{Region: "us-east-1"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rv.ValidateFormat()
	}
}

// BenchmarkClusterValidatorFormat benchmarks cluster format validation
func BenchmarkClusterValidatorFormat(b *testing.B) {
	cv := &ClusterValidator{ClusterName: "my-cluster"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cv.ValidateFormat()
	}
}

// BenchmarkManifestValidatorValidate benchmarks manifest validation
func BenchmarkManifestValidatorValidate(b *testing.B) {
	mv := &ManifestValidator{
		ManifestPath: "/tmp/deployment.yaml",
		Content: []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test`),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mv.Validate()
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || (len(s) > len(substr) && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
