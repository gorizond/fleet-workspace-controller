package controllers

import (
	"testing"

	managementv3 "github.com/gorizond/fleet-workspace-controller/pkg/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type fakeFleetWorkspaceDeleter struct {
	names []string
	err   error
}

func (f *fakeFleetWorkspaceDeleter) Delete(name string, options *metav1.DeleteOptions) error {
	f.names = append(f.names, name)
	return f.err
}

func TestEnsureWorkspacePrefix(t *testing.T) {
	tests := []struct {
		name            string
		workspaceName   string
		deleteErr       error
		wantDeleted     bool
		wantErr         bool
		wantDeleteCalls int
	}{
		{
			name:            "delete workspace without required prefix",
			workspaceName:   "demo",
			wantDeleted:     true,
			wantDeleteCalls: 1,
		},
		{
			name:            "skip delete when prefix matches",
			workspaceName:   workspacePrefix + "demo",
			wantDeleted:     false,
			wantDeleteCalls: 0,
		},
		{
			name:            "ignore not found errors during delete",
			workspaceName:   "no-prefix",
			deleteErr:       errors.NewNotFound(schema.GroupResource{Resource: "fleetworkspaces"}, "no-prefix"),
			wantDeleted:     true,
			wantDeleteCalls: 1,
		},
		{
			name:            "surface delete errors",
			workspaceName:   "broken",
			deleteErr:       errors.NewServiceUnavailable("boom"),
			wantDeleted:     true,
			wantErr:         true,
			wantDeleteCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleter := &fakeFleetWorkspaceDeleter{err: tt.deleteErr}
			ws := &managementv3.FleetWorkspace{ObjectMeta: metav1.ObjectMeta{Name: tt.workspaceName}}

			deleted, err := ensureWorkspacePrefix(deleter, ws, workspacePrefix)

			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}

			if deleted != tt.wantDeleted {
				t.Fatalf("expected deleted=%v, got %v", tt.wantDeleted, deleted)
			}

			if len(deleter.names) != tt.wantDeleteCalls {
				t.Fatalf("expected %d delete calls, got %d", tt.wantDeleteCalls, len(deleter.names))
			}

			if tt.wantDeleteCalls > 0 && deleter.names[0] != tt.workspaceName {
				t.Fatalf("delete called with %q, want %q", deleter.names[0], tt.workspaceName)
			}
		})
	}
}
