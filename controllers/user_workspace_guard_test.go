package controllers

import (
	"fmt"
	"testing"
	"time"

	managementv3 "github.com/gorizond/fleet-workspace-controller/pkg/apis/management.cattle.io/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type fakeGlobalRoleBindingIndexer struct {
	index map[string][]*managementv3.GlobalRoleBinding
	err   error
}

func (f *fakeGlobalRoleBindingIndexer) GetByIndex(indexName, key string) ([]*managementv3.GlobalRoleBinding, error) {
	if f.err != nil {
		return nil, f.err
	}

	return f.index[key], nil
}

type fakeFleetWorkspaceClient struct {
	existingCreators map[string]string
	getErr           error
	createErr        error
	createErrs       []error

	created []string
}

func (f *fakeFleetWorkspaceClient) Get(name string, opts metav1.GetOptions) (*managementv3.FleetWorkspace, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	if f.existingCreators != nil {
		if creator, ok := f.existingCreators[name]; ok {
			annotations := map[string]string{"field.cattle.io/creatorId": creator}
			return &managementv3.FleetWorkspace{ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: annotations}}, nil
		}
	}

	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "fleetworkspaces"}, name)
}

func (f *fakeFleetWorkspaceClient) Create(obj *managementv3.FleetWorkspace) (*managementv3.FleetWorkspace, error) {
	f.created = append(f.created, obj.Name)

	if len(f.createErrs) > 0 {
		err := f.createErrs[0]
		f.createErrs = f.createErrs[1:]
		if err != nil {
			return nil, err
		}
		return obj, nil
	}

	if f.createErr != nil {
		return nil, f.createErr
	}

	return obj, nil
}

func TestEnsureUserHasWorkspace(t *testing.T) {
	oldPrefix := workspacePrefix
	workspacePrefix = defaultWorkspacePrefix
	t.Cleanup(func() { workspacePrefix = oldPrefix })

	fixedNow := time.Unix(0, 123)
	oldNow := nowFn
	nowFn = func() time.Time { return fixedNow }
	t.Cleanup(func() { nowFn = oldNow })

	tests := []struct {
		name             string
		index            map[string][]*managementv3.GlobalRoleBinding
		indexErr         error
		existing         map[string]string
		getErr           error
		createErr        error
		createErrs       []error
		wantCreateCalls  int
		wantErr          bool
		wantSuffix       bool
		wantDoubleCreate bool
	}{
		{
			name: "user still has another workspace",
			index: map[string][]*managementv3.GlobalRoleBinding{
				"u1": {{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "gorizond-admin-u1-ws1",
						Labels: map[string]string{"fleet": "workspace-ws1"},
					},
					UserName: "u1",
				}},
			},
			wantCreateCalls: 0,
		},
		{
			name:            "workspace already exists for user",
			existing:        map[string]string{defaultWorkspacePrefix + "u1": "u1"},
			wantCreateCalls: 0,
		},
		{
			name:            "creates workspace when none remain",
			wantCreateCalls: 1,
		},
		{
			name: "ignores entries without fleet label",
			index: map[string][]*managementv3.GlobalRoleBinding{
				"u1": {{
					ObjectMeta: metav1.ObjectMeta{Name: "no-fleet-label"},
					UserName:   "u1",
				}},
			},
			wantCreateCalls: 1,
		},
		{
			name:            "creates unique name if existing owned by another user",
			existing:        map[string]string{defaultWorkspacePrefix + "u1": "other"},
			wantCreateCalls: 1,
			wantSuffix:      true,
		},
		{
			name: "retries with suffix on AlreadyExists",
			createErrs: []error{
				apierrors.NewAlreadyExists(schema.GroupResource{Resource: "fleetworkspaces"}, defaultWorkspacePrefix+"u1"),
				nil,
			},
			wantCreateCalls:  2,
			wantSuffix:       true,
			wantDoubleCreate: true,
		},
		{
			name: "retries with suffix when namespace terminating",
			createErrs: []error{
				apierrors.NewForbidden(schema.GroupResource{Resource: "fleetworkspaces"}, defaultWorkspacePrefix+"u1", fmt.Errorf("namespace is being terminated")),
				nil,
			},
			wantCreateCalls: 2,
			wantSuffix:      true,
		},
		{
			name:     "propagates index error",
			indexErr: apierrors.NewServiceUnavailable("boom"),
			wantErr:  true,
		},
		{
			name:    "propagates get error",
			getErr:  apierrors.NewServiceUnavailable("boom"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lister := &fakeGlobalRoleBindingIndexer{index: tt.index, err: tt.indexErr}
			client := &fakeFleetWorkspaceClient{existingCreators: tt.existing, getErr: tt.getErr, createErr: tt.createErr, createErrs: tt.createErrs}

			err := ensureUserHasWorkspace(lister, client, "u1")

			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}

			if len(client.created) != tt.wantCreateCalls {
				t.Fatalf("expected %d create calls, got %d", tt.wantCreateCalls, len(client.created))
			}

			if tt.wantCreateCalls > 0 {
				base := workspacePrefix + "u1"
				created := client.created[len(client.created)-1]

				if tt.wantSuffix {
					want := base + "-123"
					if created != want {
						t.Fatalf("created workspace %q, want %q", created, want)
					}
				} else if created != base {
					t.Fatalf("created workspace %q, want %q", created, base)
				}

				if tt.wantDoubleCreate && len(client.created) != 2 {
					t.Fatalf("expected second create attempt, got %d", len(client.created))
				}
			}
		})
	}
}
