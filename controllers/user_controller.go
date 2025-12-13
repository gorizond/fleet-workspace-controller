package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	managementv3 "github.com/gorizond/fleet-workspace-controller/pkg/apis/management.cattle.io/v3"
	"github.com/gorizond/fleet-workspace-controller/pkg/generated/controllers/management.cattle.io"
	"github.com/rancher/lasso/pkg/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type userPatcher interface {
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*managementv3.User, error)
}

func patchUserAnnotations(users userPatcher, userID string, updates map[string]interface{}) error {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": updates,
		},
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	_, err = users.Patch(userID, types.MergePatchType, body)
	return err
}

func findActiveWorkspaceForUser(fleetWorkspaces interface {
	List(opts metav1.ListOptions) (*managementv3.FleetWorkspaceList, error)
}, userID string) (string, error) {
	wsList, err := fleetWorkspaces.List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	var (
		bestName string
		bestTS   time.Time
	)
	for _, ws := range wsList.Items {
		if ws.DeletionTimestamp != nil {
			continue
		}
		if ws.Annotations["field.cattle.io/creatorId"] != userID {
			continue
		}
		ts := ws.CreationTimestamp.Time
		if bestName == "" || ts.After(bestTS) {
			bestName = ws.Name
			bestTS = ts
		}
	}

	return bestName, nil
}

func InitUserController(ctx context.Context, mgmt *management.Factory) {
	users := mgmt.Management().V3().User()
	fleetWorkspaces := mgmt.Management().V3().FleetWorkspace()

	users.OnChange(ctx, "gorizond-user-controller", func(key string, obj *managementv3.User) (*managementv3.User, error) {
		if obj == nil {
			return nil, nil
		}

		// check non system users
		if obj.Status.Conditions == nil {
			return nil, nil
		}

		// ignore system users
		for _, id := range obj.PrincipalIDs {
			if strings.HasPrefix(id, "system://") {
				if obj.Annotations == nil || obj.Annotations[selfWorkspaceInitAnnotation] != "true" {
					if err := patchUserAnnotations(users, obj.Name, map[string]interface{}{
						selfWorkspaceInitAnnotation: "true",
					}); err != nil {
						return obj, err
					}
				}
				return obj, nil
			}
		}

		selfFleet := ""
		selfInit := false
		if obj.Annotations != nil {
			selfFleet = obj.Annotations[userSelfFleetAnnotation]
			selfInit = obj.Annotations[selfWorkspaceInitAnnotation] == "true"
		}

		// If the user doesn't have a default workspace set, try to adopt an existing one.
		if selfFleet == "" {
			existing, err := findActiveWorkspaceForUser(fleetWorkspaces, obj.Name)
			if err != nil {
				return obj, err
			}
			if existing != "" {
				if err := patchUserAnnotations(users, obj.Name, map[string]interface{}{
					selfWorkspaceInitAnnotation: "true",
					userSelfFleetAnnotation:     existing,
				}); err != nil {
					return obj, err
				}
				return obj, nil
			}
		}

		// If the user has a default workspace, ensure it still exists.
		if selfFleet != "" {
			ws, err := fleetWorkspaces.Get(selfFleet, metav1.GetOptions{})
			if err == nil && ws != nil && ws.DeletionTimestamp == nil {
				if !selfInit {
					if err := patchUserAnnotations(users, obj.Name, map[string]interface{}{
						selfWorkspaceInitAnnotation: "true",
					}); err != nil {
						return obj, err
					}
				}
				return obj, nil
			}
			if err != nil && !errors.IsNotFound(err) {
				return obj, err
			}
		}

		// Create a new workspace and mark it as user's default.
		fwName := fmt.Sprintf("%s%s-%d", workspacePrefix, obj.Name, time.Now().UnixNano())
		fleetworkspace := &managementv3.FleetWorkspace{
			ObjectMeta: metav1.ObjectMeta{
				Name: fwName,
				Annotations: map[string]string{
					"field.cattle.io/creatorId": obj.Name,
				},
			},
		}

		if _, err := fleetWorkspaces.Create(fleetworkspace); err != nil && !errors.IsAlreadyExists(err) {
			log.Infof("Failed to create fleetworkspace %s: %v", obj.Username, err)
			return obj, err
		}

		if err := patchUserAnnotations(users, obj.Name, map[string]interface{}{
			selfWorkspaceInitAnnotation: "true",
			userSelfFleetAnnotation:     fwName,
		}); err != nil {
			return obj, err
		}

		return obj, nil
	})
}
