package controllers

import (
	"context"
	"fmt"
	"time"

	managementv3 "github.com/gorizond/fleet-workspace-controller/pkg/apis/management.cattle.io/v3"
	"github.com/gorizond/fleet-workspace-controller/pkg/generated/controllers/management.cattle.io"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var nowFn = time.Now

// globalRoleBindingIndexer covers indexed lookup by user.
type globalRoleBindingIndexer interface {
	GetByIndex(indexName, key string) ([]*managementv3.GlobalRoleBinding, error)
}

// fleetWorkspaceClient covers get/create operations used for the home workspace.
type fleetWorkspaceClient interface {
	Get(name string, opts metav1.GetOptions) (*managementv3.FleetWorkspace, error)
	Create(*managementv3.FleetWorkspace) (*managementv3.FleetWorkspace, error)
}

// InitUserWorkspaceGuard recreates a personal workspace if a user loses their last Fleet binding.
func InitUserWorkspaceGuard(ctx context.Context, mgmt *management.Factory) {
	globalRoleBindings := mgmt.Management().V3().GlobalRoleBinding()
	fleetWorkspaces := mgmt.Management().V3().FleetWorkspace()

	grbCache := globalRoleBindings.Cache()
	grbCache.AddIndexer("byUser", func(obj *managementv3.GlobalRoleBinding) ([]string, error) {
		if obj.UserName == "" {
			return nil, nil
		}
		return []string{obj.UserName}, nil
	})

	globalRoleBindings.OnRemove(ctx, "gorizond-user-workspace-guard", func(key string, obj *managementv3.GlobalRoleBinding) (*managementv3.GlobalRoleBinding, error) {
		if obj == nil {
			return nil, nil
		}

		// Only act on per-fleet bindings for real users.
		if obj.Labels == nil || obj.Labels["fleet"] == "" || obj.UserName == "" {
			return obj, nil
		}

		if err := ensureUserHasWorkspace(grbCache, fleetWorkspaces, obj.UserName); err != nil {
			return obj, err
		}

		return obj, nil
	})
}

func ensureUserHasWorkspace(grbIndexer globalRoleBindingIndexer, fleet fleetWorkspaceClient, userID string) error {
	bindings, err := grbIndexer.GetByIndex("byUser", userID)
	if err != nil {
		return err
	}

	for _, binding := range bindings {
		if binding.Labels["fleet"] == "" {
			continue
		}

		// The user still has access to a FleetWorkspace.
		return nil
	}

	baseName := workspacePrefix + userID
	targetName := baseName

	if existing, err := fleet.Get(baseName, metav1.GetOptions{}); err == nil {
		if existing.Annotations != nil && existing.Annotations["field.cattle.io/creatorId"] == userID {
			return nil
		}

		targetName = fmt.Sprintf("%s-%d", baseName, nowFn().UnixNano())
	} else if !errors.IsNotFound(err) {
		return err
	}

	_, err = fleet.Create(&managementv3.FleetWorkspace{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetName,
			Annotations: map[string]string{
				"field.cattle.io/creatorId": userID,
			},
		},
	})
	if errors.IsAlreadyExists(err) && targetName == baseName {
		targetName = fmt.Sprintf("%s-%d", baseName, nowFn().UnixNano())

		_, err = fleet.Create(&managementv3.FleetWorkspace{
			ObjectMeta: metav1.ObjectMeta{
				Name: targetName,
				Annotations: map[string]string{
					"field.cattle.io/creatorId": userID,
				},
			},
		})
	}

	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
