package controllers

import (
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCreateWorkspaceForUser(t *testing.T) {
	oldPrefix := workspacePrefix
	workspacePrefix = defaultWorkspacePrefix
	t.Cleanup(func() { workspacePrefix = oldPrefix })

	fixedNow := time.Unix(0, 456)
	oldNow := nowFn
	nowFn = func() time.Time { return fixedNow }
	t.Cleanup(func() { nowFn = oldNow })

	tests := []struct {
		name            string
		existing        map[string]string
		getErr          error
		createErr       error
		wantCreateCalls int
		wantErr         bool
		wantName        string
	}{
		{
			name:            "creates when missing",
			wantCreateCalls: 1,
			wantName:        defaultWorkspacePrefix + "u1",
		},
		{
			name:            "skips when already owned",
			existing:        map[string]string{defaultWorkspacePrefix + "u1": "u1"},
			wantCreateCalls: 0,
		},
		{
			name:            "creates unique when name owned by another",
			existing:        map[string]string{defaultWorkspacePrefix + "u1": "other"},
			wantCreateCalls: 1,
			wantName:        defaultWorkspacePrefix + "u1-456",
		},
		{
			name:            "retries on AlreadyExists",
			createErr:       apierrors.NewAlreadyExists(schema.GroupResource{Resource: "fleetworkspaces"}, defaultWorkspacePrefix+"u1"),
			wantCreateCalls: 2,
			wantName:        defaultWorkspacePrefix + "u1-456",
		},
		{
			name:    "propagates get error",
			getErr:  apierrors.NewServiceUnavailable("boom"),
			wantErr: true,
		},
		{
			name:            "propagates create error",
			createErr:       apierrors.NewServiceUnavailable("boom"),
			wantCreateCalls: 1,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeFleetWorkspaceClient{existingCreators: tt.existing, getErr: tt.getErr, createErr: tt.createErr}

			err := createWorkspaceForUser(client, "u1")

			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}

			if len(client.created) != tt.wantCreateCalls {
				t.Fatalf("expected %d create calls, got %d", tt.wantCreateCalls, len(client.created))
			}

			if tt.wantName != "" && len(client.created) > 0 {
				got := client.created[len(client.created)-1]
				if got != tt.wantName {
					t.Fatalf("created name %q, want %q", got, tt.wantName)
				}
			}
		})
	}
}
