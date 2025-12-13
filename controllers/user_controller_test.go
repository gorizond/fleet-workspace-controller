package controllers

import (
	"encoding/json"
	"testing"
	"time"

	managementv3 "github.com/gorizond/fleet-workspace-controller/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type fakeUserPatcher struct {
	lastName      string
	lastPatchType types.PatchType
	lastData      []byte
}

func (f *fakeUserPatcher) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*managementv3.User, error) {
	f.lastName = name
	f.lastPatchType = pt
	f.lastData = append([]byte(nil), data...)
	return &managementv3.User{}, nil
}

func TestPatchUserAnnotations(t *testing.T) {
	fake := &fakeUserPatcher{}
	err := patchUserAnnotations(fake, "u1", map[string]interface{}{
		selfWorkspaceInitAnnotation: nil,
		userSelfFleetAnnotation:     "ws1",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if fake.lastName != "u1" {
		t.Fatalf("expected patch name u1, got %q", fake.lastName)
	}
	if fake.lastPatchType != types.MergePatchType {
		t.Fatalf("expected merge patch type, got %v", fake.lastPatchType)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(fake.lastData, &payload); err != nil {
		t.Fatalf("failed to unmarshal patch: %v", err)
	}
	metadata, ok := payload["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected metadata object")
	}
	annotations, ok := metadata["annotations"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected annotations object")
	}
	if _, ok := annotations[selfWorkspaceInitAnnotation]; !ok {
		t.Fatalf("expected %q key present", selfWorkspaceInitAnnotation)
	}
	if annotations[selfWorkspaceInitAnnotation] != nil {
		t.Fatalf("expected %q to be null", selfWorkspaceInitAnnotation)
	}
	if annotations[userSelfFleetAnnotation] != "ws1" {
		t.Fatalf("expected %q to be ws1, got %v", userSelfFleetAnnotation, annotations[userSelfFleetAnnotation])
	}
}

type fakeFleetWorkspaceLister struct {
	list *managementv3.FleetWorkspaceList
	err  error
}

func (f fakeFleetWorkspaceLister) List(opts metav1.ListOptions) (*managementv3.FleetWorkspaceList, error) {
	return f.list, f.err
}

func TestFindActiveWorkspaceForUser(t *testing.T) {
	userID := "user123"
	now := time.Now()
	deleteTS := metav1.NewTime(now.Add(-time.Minute))

	list := &managementv3.FleetWorkspaceList{Items: []managementv3.FleetWorkspace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "ws-old",
				CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Minute)),
				Annotations: map[string]string{
					"field.cattle.io/creatorId": userID,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "ws-new",
				CreationTimestamp: metav1.NewTime(now.Add(-1 * time.Minute)),
				Annotations: map[string]string{
					"field.cattle.io/creatorId": userID,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "ws-other-user",
				CreationTimestamp: metav1.NewTime(now.Add(-30 * time.Second)),
				Annotations: map[string]string{
					"field.cattle.io/creatorId": "other",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "ws-deleting",
				CreationTimestamp: metav1.NewTime(now),
				DeletionTimestamp: &deleteTS,
				Annotations: map[string]string{
					"field.cattle.io/creatorId": userID,
				},
			},
		},
	}}

	ws, err := findActiveWorkspaceForUser(fakeFleetWorkspaceLister{list: list}, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws != "ws-new" {
		t.Fatalf("expected ws-new, got %q", ws)
	}
}
