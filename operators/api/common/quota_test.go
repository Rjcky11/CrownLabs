package common // DEVE essere lo stesso package di quota.go

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceSpec_UnmarshalLegacyCPUString(t *testing.T) {
	// Test 1: JSON legacy con la CPU rappresentata come stringa "4"
	rawJSON := []byte(`{
		"cpu": "4",
		"memory": "2Gi"
	}`)

	var spec ResourceSpec
	err := json.Unmarshal(rawJSON, &spec)

	assert.NoError(t, err)
	assert.Equal(t, int64(4), spec.CPU)
}

func TestResourceSpec_UnmarshalLegacyCPUMillicores(t *testing.T) {
	// Test 2: JSON legacy con millicores "8000m" -> deve diventare 8 int64
	rawJSON := []byte(`{
		"cpu": "8000m",
		"memory": "4Gi"
	}`)

	var spec ResourceSpec
	err := json.Unmarshal(rawJSON, &spec)

	assert.NoError(t, err)
	assert.Equal(t, int64(8), spec.CPU)
}

func TestWorkspaceResourceQuota_UnmarshalLegacyCPU(t *testing.T) {
	// Test 3: WorkspaceResourceQuota con CPU stringa e campo instances
	rawJSON := []byte(`{
		"cpu": "16",
		"memory": "8Gi",
		"instances": 5
	}`)

	var quota WorkspaceResourceQuota
	err := json.Unmarshal(rawJSON, &quota)

	assert.NoError(t, err)
	assert.Equal(t, int64(16), quota.CPU)
	assert.Equal(t, int64(5), quota.Instances)
}
