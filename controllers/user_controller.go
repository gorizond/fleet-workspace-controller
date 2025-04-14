package controllers

import (
	"context"
	"github.com/rancher/lasso/pkg/log"
	managementv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func InitUserController(ctx context.Context, mgmt *management.Factory) {
	users := mgmt.Management().V3().User()
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

			fleetworkspace := &managementv3.FleetWorkspace{
				ObjectMeta: metav1.ObjectMeta{
					Name: obj.Username + "-workspace",
					Annotations: map[string]string{
						"field.cattle.io/creatorId": obj.Name,
					},
				},
			}

			_, err := mgmt.Management().V3().FleetWorkspace().Create(fleetworkspace)
			if err != nil && !errors.IsAlreadyExists(err) {
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
