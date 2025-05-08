package controllers

import (
	"context"
	"strings"

	managementv3 "github.com/gorizond/fleet-workspace-controller/pkg/apis/management.cattle.io/v3"
	"github.com/gorizond/fleet-workspace-controller/pkg/generated/controllers/management.cattle.io"
)

func InitGlobalRoleBindingController(ctx context.Context, mgmt *management.Factory) {
	globalRoles := mgmt.Management().V3().GlobalRole()
	globalRoleBinding := mgmt.Management().V3().GlobalRoleBinding()
	globalRoles.OnChange(ctx, "gorizond-admin-bindings-controller", func(key string, obj *managementv3.GlobalRole) (*managementv3.GlobalRole, error) {
		if obj == nil {
			return nil, nil
		}

		if _, ok := obj.Labels["fleet"]; !ok {
			return nil, nil
		}

		if !strings.HasPrefix(obj.Name, "gorizond-admin-") {
			return nil, nil
		}

		firstInit := obj.Annotations != nil && obj.Annotations["global-role-init"] == "true"

		if firstInit {
			return obj, nil
		}

		// set user as admin for workspace
		userID := obj.Annotations["field.cattle.io/creatorId"]
		FleetName := obj.Labels["fleet"]
		createGlobalRoleBinding(globalRoleBinding, "gorizond-user.", FleetName, "gorizond-user."+userID+".admin")

		obj = obj.DeepCopy()
		// Add annotation
		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}
		obj.Annotations["global-role-init"] = "true"

		return globalRoles.Update(obj)
	})
}
