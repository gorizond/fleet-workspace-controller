package controllers

import (
	"context"
	"fmt"
	"strings"

	managementv3 "github.com/gorizond/fleet-workspace-controller/pkg/apis/management.cattle.io/v3"
	"github.com/gorizond/fleet-workspace-controller/pkg/generated/controllers/management.cattle.io"
	"github.com/rancher/lasso/pkg/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
		// check workspace init on user create
		firstInit := obj.Annotations != nil && obj.Annotations["self-workspace-init"] == "true"

		if firstInit {
			return obj, nil
		}
		// ignore system users
		for _, id := range obj.PrincipalIDs {
			if strings.HasPrefix(id, "system://") {
				if obj.Annotations == nil {
					obj.Annotations = make(map[string]string)
				}
				obj.Annotations["self-workspace-init"] = "true"
				return users.Update(obj)
			}
		}

		if err := createWorkspaceForUser(fleetWorkspaces, obj.Name); err != nil {
			log.Infof("Failed to create fleetworkspace %s: %v", obj.Username, err)
		}
		obj = obj.DeepCopy()

		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}

		obj.Annotations["self-workspace-init"] = "true"

		// Here we are using the k8s client embedded onto the users controller to perform an update. This will go to the K8s API.
		return users.Update(obj)
	},
	)
}

func createWorkspaceForUser(fleet fleetWorkspaceClient, userID string) error {
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

	_, err := fleet.Create(&managementv3.FleetWorkspace{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetName,
			Annotations: map[string]string{
				"field.cattle.io/creatorId": userID,
			},
		},
	})

	if shouldRetryWithSuffix(err, targetName == baseName) {
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
