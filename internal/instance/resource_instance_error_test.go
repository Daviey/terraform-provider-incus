package instance

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatInstanceCreationError(t *testing.T) {
	tests := []struct {
		name         string
		instanceName string
		err          error
		wantContains []string
	}{
		{
			name:         "storage unique constraint error",
			instanceName: "test-instance",
			err:          errors.New("UNIQUE constraint failed: storage_volumes_unique_storage_pool_id_node_id_project_id_name_type"),
			wantContains: []string{
				"UNIQUE constraint failed",
				"storage volume with the name \"test-instance\" already exists",
				"Check for orphaned storage volumes: incus storage volume list default",
				"DELETE FROM storage_volumes WHERE name=\"test-instance\"",
			},
		},
		{
			name:         "generic database error",
			instanceName: "test-instance",
			err:          errors.New("database is locked"),
			wantContains: []string{"database is locked"},
		},
		{
			name:         "permission denied error",
			instanceName: "myinstance",
			err:          errors.New("permission denied"),
			wantContains: []string{"permission denied"},
		},
		{
			name:         "empty error",
			instanceName: "test",
			err:          errors.New(""),
			wantContains: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatInstanceCreationError(tt.instanceName, tt.err)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatInstanceCreationError() = %v, should contain %q", got, want)
				}
			}

			// For non-storage constraint errors, the result should match the original error
			if !strings.Contains(tt.err.Error(), "UNIQUE constraint failed") {
				if got != tt.err.Error() {
					t.Errorf("formatInstanceCreationError() = %v, want %v", got, tt.err.Error())
				}
			}
		})
	}
}