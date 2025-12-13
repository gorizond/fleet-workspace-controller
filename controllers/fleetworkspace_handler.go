package controllers

import (
	"context"
	"os"
	"strings"

	managementv3 "github.com/gorizond/fleet-workspace-controller/pkg/apis/management.cattle.io/v3"
	"github.com/gorizond/fleet-workspace-controller/pkg/generated/controllers/management.cattle.io"
	"github.com/rancher/lasso/pkg/log"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultWorkspacePrefix      = "workspace-"
	selfWorkspaceInitAnnotation = "self-workspace-init"
	userSelfFleetAnnotation     = "gorizond-self-fleet"
)

var workspacePrefix = getWorkspacePrefix()

type fleetWorkspaceDeleter interface {
	Delete(name string, options *metav1.DeleteOptions) error
}

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

		deleted, err := ensureWorkspacePrefix(fleetWorkspaces, obj, workspacePrefix)
		if err != nil {
			return obj, err
		}
		if deleted {
			return obj, nil
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
		searchedUser, err := findUserByUsername(os.Getenv("RANCHER_URL"), os.Getenv("RANCHER_TOKEN"), "/v3/user?id="+obj.Annotations["field.cattle.io/creatorId"])
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
		if obj == nil {
			return nil, nil
		}

		creator := ""
		if obj.Annotations != nil {
			creator = obj.Annotations["field.cattle.io/creatorId"]
		}
		if creator != "" {
			user, err := users.Get(creator, metav1.GetOptions{})
			if err != nil {
				if !errors.IsNotFound(err) {
					log.Infof("Failed to get user %s for deleted workspace %s: %v", creator, obj.Name, err)
				}
			} else {
				selfFleet := ""
				if user.Annotations != nil {
					selfFleet = user.Annotations[userSelfFleetAnnotation]
				}

				shouldReset := selfFleet == "" || selfFleet == obj.Name
				if !shouldReset && selfFleet != "" {
					ws, err := fleetWorkspaces.Get(selfFleet, metav1.GetOptions{})
					if err != nil {
						if errors.IsNotFound(err) {
							shouldReset = true
						} else {
							log.Infof("Failed to get fleetworkspace %s referenced by user %s: %v", selfFleet, creator, err)
						}
					} else if ws == nil || ws.DeletionTimestamp != nil {
						shouldReset = true
					}
				}

				if shouldReset {
					if err := patchUserAnnotations(users, creator, map[string]interface{}{
						selfWorkspaceInitAnnotation: nil,
						userSelfFleetAnnotation:     nil,
					}); err != nil && !errors.IsNotFound(err) {
						log.Infof("Failed to reset user annotations user=%s workspace=%s: %v", creator, obj.Name, err)
					}
				}
			}
		}

		// check rules init on workspace
		gorizondInit := obj.Annotations != nil && obj.Annotations["workspace-roles-init"] == "true"
		if !gorizondInit {
			return obj, nil
		}

		if err := mgmt.Management().V3().GlobalRole().Delete("gorizond-admin-"+obj.Name, nil); err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		if err := mgmt.Management().V3().GlobalRole().Delete("gorizond-editor-"+obj.Name, nil); err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		if err := mgmt.Management().V3().GlobalRole().Delete("gorizond-view-"+obj.Name, nil); err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		return obj, nil
	},
	)
}

func ensureWorkspacePrefix(fleetWorkspaces fleetWorkspaceDeleter, obj *managementv3.FleetWorkspace, expectedPrefix string) (bool, error) {
	if strings.HasPrefix(obj.Name, expectedPrefix) {
		return false, nil
	}

	log.Infof("Deleting fleet workspace %q without required prefix %q", obj.Name, expectedPrefix)

	if err := fleetWorkspaces.Delete(obj.Name, nil); err != nil && !errors.IsNotFound(err) {
		return true, err
	}

	return true, nil
}

func getWorkspacePrefix() string {
	if env := os.Getenv("WORKSPACE_PREFIX"); env != "" {
		return env
	}

	return defaultWorkspacePrefix
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
