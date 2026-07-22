package common // DEVE essere lo stesso package di quota.go

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
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

func TestWorkspaceResourceQuotaUnmarshal(t *testing.T) {
	rawJSON := []byte(`{"cpu": "6", "memory": "22Gi", "instances": 5}`)
	var quota WorkspaceResourceQuota

	err := json.Unmarshal(rawJSON, &quota)
	assert.Nil(t, err)
	assert.Equal(t, int64(6), quota.CPU)
}

// ValidateInstanceAgainstQuota verifica se le risorse richieste superano la quota del workspace.
func ValidateInstanceAgainstQuota(requested *ResourceSpec, quota *WorkspaceResourceQuota) bool {
	if requested == nil || quota == nil {
		return true
	}

	// 1. Controllo CPU
	if requested.CPU > quota.CPU {
		return false
	}

	// 2. Controllo Memory
	if requested.Memory.Cmp(quota.Memory) > 0 {
		return false
	}

	// 3. Controllo Disk (se definito nella quota)
	if !quota.Disk.IsZero() && requested.Disk.Cmp(quota.Disk) > 0 {
		return false
	}

	// 4. Controllo Extended Resources (GPU, ecc.)
	for resName, reqQty := range requested.OtherResources {
		quotaQty, exists := quota.OtherResources[resName]
		if !exists || reqQty.Cmp(quotaQty) > 0 {
			return false
		}
	}

	return true
}

func TestInstanceQuotaValidation_BlockedWhenOverQuota(t *testing.T) {
	workspaceQuota := WorkspaceResourceQuota{
		ResourceSpec: ResourceSpec{
			CPU:    6,
			Memory: resource.MustParse("22Gi"),
			OtherResources: map[string]resource.Quantity{
				"nvidia.com/gpu": resource.MustParse("1"),
			},
		},
		Instances: 6,
	}

	testCases := []struct {
		name        string
		requested   ResourceSpec
		shouldAllow bool
	}{
		{
			name: "Allowed - Normal instance within limits",
			requested: ResourceSpec{
				CPU:    2,
				Memory: resource.MustParse("4Gi"),
			},
			shouldAllow: true,
		},
		{
			name: "Blocked - CPU exceeds workspace quota (7 CPUs requested, max 6)",
			requested: ResourceSpec{
				CPU:    7,
				Memory: resource.MustParse("4Gi"),
			},
			shouldAllow: false,
		},
		{
			name: "Blocked - RAM exceeds workspace quota (32Gi requested, max 22Gi)",
			requested: ResourceSpec{
				CPU:    2,
				Memory: resource.MustParse("32Gi"),
			},
			shouldAllow: false,
		},
		{
			name: "Blocked - Requesting unallocated GPU vendor (AMD GPU requested, 0 allocated)",
			requested: ResourceSpec{
				CPU:    2,
				Memory: resource.MustParse("4Gi"),
				OtherResources: map[string]resource.Quantity{
					"amd.com/gpu": resource.MustParse("1"),
				},
			},
			shouldAllow: false,
		},
		{
			name: "Blocked - GPU quantity exceeds quota (2 Nvidia GPUs requested, max 1)",
			requested: ResourceSpec{
				CPU:    2,
				Memory: resource.MustParse("4Gi"),
				OtherResources: map[string]resource.Quantity{
					"nvidia.com/gpu": resource.MustParse("2"),
				},
			},
			shouldAllow: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed := ValidateInstanceAgainstQuota(&tc.requested, &workspaceQuota)
			assert.Equal(t, tc.shouldAllow, allowed, "Quota validation failed for test case: %s", tc.name)
		})
	}
}

func TestWorkspaceQuota_StringCPU_UnmarshalAndValidation(t *testing.T) {
	// Simula lo YAML/JSON del Workspace presente sul cluster, dove la CPU è registrata come stringa "6"
	rawWorkspaceJSON := []byte(`{
		"cpu": "6",
		"memory": "22Gi",
		"instances": 6
	}`)

	var workspaceQuota WorkspaceResourceQuota
	err := json.Unmarshal(rawWorkspaceJSON, &workspaceQuota)
	require.NoError(t, err, "L'unmarshaling del Workspace con CPU stringa '6' non deve fallire")

	// Verifica che il parser custom int64Flexible abbia convertito "6" (stringa) nel valore numerico int64 6
	assert.Equal(t, int64(6), workspaceQuota.CPU, "La CPU del workspace deve essere valutata come int64(6)")

	testCases := []struct {
		name        string
		rawInstance string
		shouldAllow bool
	}{
		{
			name: "Consentita - Istanza nei limiti (2 CPU sotto forma di stringa)",
			rawInstance: `{
				"cpu": 2,
				"memory": "4Gi"
			}`,
			shouldAllow: true,
		},
		{
			name: "Consentita - Istanza con notazione millicores (5000m = 5 CPU)",
			rawInstance: `{
				"cpu": "5000m",
				"memory": "4Gi"
			}`,
			shouldAllow: true,
		},
		{
			name: "BLOCCATA - Istanza oltre quota CPU (7 CPU sotto forma di stringa '7')",
			rawInstance: `{
				"cpu": 7,
				"memory": "4Gi"
			}`,
			shouldAllow: false,
		},
		{
			name: "BLOCCATA - Istanza oltre quota CPU in millicores (7000m = 7 CPU > 6)",
			rawInstance: `{
				"cpu": "7000m",
				"memory": "4Gi"
			}`,
			shouldAllow: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var requestedSpec ResourceSpec
			err := json.Unmarshal([]byte(tc.rawInstance), &requestedSpec)
			require.NoError(t, err, "L'unmarshaling dell'istanza non deve fallire")

			allowed := ValidateInstanceAgainstQuota(&requestedSpec, &workspaceQuota)
			assert.Equal(t, tc.shouldAllow, allowed, "Esito validazione non coerente per la prova: %s", tc.name)
		})
	}
}
