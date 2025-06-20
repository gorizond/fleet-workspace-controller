package controllers

import (
	"os"
	"context"
	"github.com/rancher/lasso/pkg/log"
	managementv3 "github.com/gorizond/fleet-workspace-controller/pkg/apis/management.cattle.io/v3"
	"github.com/gorizond/fleet-workspace-controller/pkg/generated/controllers/management.cattle.io"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

func InitFleetWorkspaceController(ctx context.Context, mgmt *management.Factory) {
	fleetWorkspaces := mgmt.Management().V3().FleetWorkspace()
	users := mgmt.Management().V3().User()
	principal := mgmt.Management().V3().Principal()
	globalRoleBinding := mgmt.Management().V3().GlobalRoleBinding()
	fleetWorkspaces.OnChange(ctx, "gorizond-fleetworkspace-controller", func(key string, obj *managementv3.FleetWorkspace) (*managementv3.FleetWorkspace, error) {
		if obj == nil {
			return nil, nil
		}
		// ignore default workspaces
		if obj.Name == "fleet-default" || obj.Name == "fleet-local" {
			return nil, nil
		}

		//
		//
		//
		// Create or update global role bindings based on annotations
		for k, v := range obj.Annotations {
			if strings.HasPrefix(k, "gorizond-user.") {
				createGlobalRoleBinding(globalRoleBinding, "gorizond-user.", obj.Name, k)
			}
			if strings.HasPrefix(k, "gorizond-group.") {
				createGlobalRoleBindingForGroup(globalRoleBinding, "gorizond-group.", obj.Name, k, v)
			}
			if strings.HasPrefix(k, "gorizond-principal.") {
				return findByPrincipal(users, principal, globalRoleBinding, obj, fleetWorkspaces, k, v)
			}
		}

		// List all global role bindings with the label `fleet: <fleetWorkspace>`
		globalRoleBindings, err := mgmt.Management().V3().GlobalRoleBinding().List(metav1.ListOptions{
			LabelSelector: "fleet=" + obj.Name,
		})
		if err != nil {
			log.Infof("Failed to list global role bindings: %v", err)
			return obj, nil
		}

		// Delete global role bindings that do not have corresponding annotations
		for _, binding := range globalRoleBindings.Items {
			found := false
			for k := range obj.Annotations {
				if strings.HasPrefix(k, "gorizond-user.") && k == binding.Annotations["gorizond-binding"] {
					found = true
					break
				}
				if strings.HasPrefix(k, "gorizond-group.") && k == binding.Annotations["gorizond-binding"] {
					found = true
					break
				}
			}
			if !found {
				err := mgmt.Management().V3().GlobalRoleBinding().Delete(binding.Name, nil)
				if err != nil && !errors.IsNotFound(err) {
					log.Infof("Failed to delete global role binding %s: %v", binding.Name, err)
				}
			}
		}
		//
		// create ROLES
		//
		// check rules init on workspace create
		firstInit := obj.Annotations != nil && obj.Annotations["workspace-roles-init"] == "true"

		if firstInit {
			return obj, nil
		}

		// Create roles
		createRole(mgmt, obj, "admin", []string{"*"}, obj.Annotations["field.cattle.io/creatorId"])
		createRole(mgmt, obj, "editor", []string{"get", "list", "watch", "update", "patch"}, obj.Annotations["field.cattle.io/creatorId"])
		createRole(mgmt, obj, "view", []string{"get", "list", "watch"}, obj.Annotations["field.cattle.io/creatorId"])

		obj = obj.DeepCopy()
		// Add annotation
		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}
		obj.Annotations["workspace-roles-init"] = "true"
		// find principal for user if exist
		searchedUser, err := findUserByUsername(os.Getenv("RANCHER_URL"), os.Getenv("RANCHER_TOKEN"), "/v3/user?id=" + obj.Annotations["field.cattle.io/creatorId"])
		if err != nil {
			return nil, err
		}
		principalId := "local://" + obj.Annotations["field.cattle.io/creatorId"]
		for _, iterPrincipal := range searchedUser.Data[0].PrincipalIDs {
			if !strings.HasPrefix(iterPrincipal, "local://") {
				principalId = iterPrincipal
			}
		}
		obj.Annotations["gorizond-user."+obj.Annotations["field.cattle.io/creatorId"]+".admin"] = principalId

		return fleetWorkspaces.Update(obj)
	},
	)
	fleetWorkspaces.OnRemove(ctx, "gorizond-workspace-delete", func(key string, obj *managementv3.FleetWorkspace) (*managementv3.FleetWorkspace, error) {
		// check rules init on workspace
		gorizondInit := obj.Annotations != nil && obj.Annotations["workspace-roles-init"] == "true"

		if !gorizondInit {
			return obj, nil
		}

		err := mgmt.Management().V3().GlobalRole().Delete("gorizond-admin-"+obj.Name, nil)
		if err != nil {
			return nil, err
		}
		err = mgmt.Management().V3().GlobalRole().Delete("gorizond-editor-"+obj.Name, nil)
		if err != nil {
			return nil, err
		}
		err = mgmt.Management().V3().GlobalRole().Delete("gorizond-view-"+obj.Name, nil)
		if err != nil {
			return nil, err
		}
		return obj, nil
	},
	)
}

func createRole(mgmt *management.Factory, fleetworkspace *managementv3.FleetWorkspace, role string, verbs []string, userID string) {
	roleName := "gorizond-" + role + "-" + fleetworkspace.Name
	billingVerbs := []string{"get", "list", "watch"}
	if role == "admin" {
		billingVerbs = []string{"create", "delete", "get", "list", "watch"}
	}
	globalRole := &managementv3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Annotations: map[string]string{
				"field.cattle.io/creatorId": userID,
			},
			Labels: map[string]string{
				"role":  role,
				"fleet": fleetworkspace.Name,
			},
		},
		DisplayName: "GitOps for " + role + " " + fleetworkspace.Name,
		NamespacedRules: map[string][]rbacv1.PolicyRule{
			fleetworkspace.Name: {
				{
					APIGroups: []string{"fleet.cattle.io"},
					Resources: []string{"gitrepos", "bundles", "clusterregistrationtokens", "gitreporestrictions", "clusters", "clustergroups"},
					Verbs:     verbs,
				},
				{
					APIGroups: []string{"provisioning.gorizond.io"},
					Resources: []string{"clusters"},
					Verbs:     verbs,
				},
				{
					APIGroups: []string{"provisioning.gorizond.io"},
					Resources: []string{"billings", "billingevents"},
					Verbs:     billingVerbs,
				},
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"management.cattle.io"},
				ResourceNames: []string{fleetworkspace.Name},
				Resources:     []string{"fleetworkspaces"},
				Verbs:         verbs,
			},
			{
				APIGroups:     []string{"provisioning.gorizond.io"},
				ResourceNames: []string{fleetworkspace.Name},
				Resources:     []string{"clusters"},
				Verbs:         verbs,
			},
			{
				APIGroups:     []string{"provisioning.gorizond.io"},
				ResourceNames: []string{fleetworkspace.Name},
				Resources:     []string{"billings", "billingevents"},
				Verbs:         billingVerbs,
			},
		},
	}

	_, err := mgmt.Management().V3().GlobalRole().Create(globalRole)
	if err != nil && !errors.IsAlreadyExists(err) {
		log.Infof("Failed to create global role: %v", err)
	}
}
