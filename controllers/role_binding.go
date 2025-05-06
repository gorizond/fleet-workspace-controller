package controllers

import (
	"fmt"
	"strings"
	"time"

	"github.com/rancher/lasso/pkg/log"
	managementv3 "github.com/gorizond/fleet-workspace-controller/pkg/apis/management.cattle.io/v3"
	v3 "github.com/gorizond/fleet-workspace-controller/pkg/generated/controllers/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createGlobalRoleBinding(mgmt v3.GlobalRoleBindingController, fleetworkspaceName string, annotationKey string) {
	parts := strings.SplitN(annotationKey[len("gorizond-user."):], ".", 2)
	userID := parts[0]
	role := parts[1]
	globalRoleBinding := &managementv3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gorizond-" + role + "-" + userID + "-" + fleetworkspaceName,
			Annotations: map[string]string{
				"gorizond-binding": annotationKey,
			},
			Labels: map[string]string{
				"fleet": fleetworkspaceName,
			},
		},
		UserName:       userID,
		GlobalRoleName: "gorizond-" + role + "-" + fleetworkspaceName,
	}

	_, err := mgmt.Create(globalRoleBinding)
	if err != nil && !errors.IsAlreadyExists(err) {
		log.Infof("Failed to create global role binding: %v", err)
	}
}


func findByPrincipal(users v3.UserController, principal v3.PrincipalController, mgmt v3.GlobalRoleBindingController, fleetworkspace *managementv3.FleetWorkspace, fleetWorkspaces v3.FleetWorkspaceController, annotationKey string, annotationValue string) (*managementv3.FleetWorkspace, error) {
	parts := strings.SplitN(annotationKey[len("gorizond-principal."):], ".", 2)
	principalID := parts[0]
	role := parts[1]
	// check if group
	isGroupPrincipal := false
	if strings.HasPrefix(principalID, "github_org://") {
		isGroupPrincipal = true
	}
	if strings.HasPrefix(principalID, "genericoidc_group://") {
		isGroupPrincipal = true
	}
	// build tmp GlobalRoleBinding with ttl for rancher create if not exist user/group from principal to rancher user/group
	globalRoleBindingTMP := &managementv3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "gorizond-tmp-",
			Annotations: map[string]string{
				"gorizond-binding": annotationKey,
			},
			Labels: map[string]string{
				"fleet": fleetworkspace.Name,
				"gorizond-ttl": "30",
			},
		},
		GlobalRoleName: "gorizond-" + role + "-" + fleetworkspace.Name,
	}
	if isGroupPrincipal {
		globalRoleBindingTMP.GroupPrincipalName = principalID
		globalRoleBindingTMP.Annotations["type"] = "group"
	} else {
		globalRoleBindingTMP.UserPrincipalName = principalID
		globalRoleBindingTMP.Annotations["type"] = "user"
	}
	_, err := mgmt.Create(globalRoleBindingTMP)
	if err != nil && !errors.IsAlreadyExists(err) {
		log.Infof("Failed to create global role binding: %v", err)
	}

	principalObject, err := principal.Get(principalID, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to create global role binding: %v", err)
	}
	
	if principalObject.PrincipalType == "group" {
		// TODO create group code
		log.Errorf("Rancher groups not comming soon")
		return nil, nil
	}
	log.Infof("Try find rancher user for %s (%s)", principalObject.LoginName, principalObject.DisplayName)
	// find new NOT INIT users
	searchedUser1, err := users.List(metav1.ListOptions{FieldSelector: "username="})
	if err != nil {
		return nil, fmt.Errorf("Failed to find metav1.ListOptions{FieldSelector: username=}: %v", err)
	}
	log.Infof("Found username=empty %d",len(searchedUser1.Items))
	// find exist users with username=principal LoginName
	searchedUser2, err := users.List(metav1.ListOptions{FieldSelector: "username=" +  strings.ToLower(principalObject.LoginName)})
	if err != nil {
		return nil, fmt.Errorf("Failed to find metav1.ListOptions{FieldSelector: username=principalObject.LoginName}: %v", err)
	}
	log.Infof("Found username=%s %d",strings.ToLower(principalObject.LoginName), len(searchedUser2.Items))
	// find exist users with name=principal LoginName
	searchedUser3, err := users.List(metav1.ListOptions{FieldSelector: "name=" +  strings.ToLower(principalObject.LoginName)})
	if err != nil {
		return nil, fmt.Errorf("Failed to find metav1.ListOptions{FieldSelector: name=principalObject.LoginName}: %v", err)
	}
	log.Infof("Found name=%s %d",strings.ToLower(principalObject.LoginName), len(searchedUser3.Items))
	items := append(searchedUser1.Items, searchedUser2.Items...)
	items = append(items, searchedUser3.Items...)
	userFind := false
	userlocalID := ""
	for _, user := range items {
		// iter all PrincipalIDs
		if user.PrincipalIDs != nil {
			for _, iterPrincipal := range user.PrincipalIDs {
				if iterPrincipal == principalID {
					userFind = true
				}
				if strings.HasPrefix(iterPrincipal, "local://") && userFind {
					userlocalID = strings.Split(iterPrincipal, "://")[1]
					break
				}
			}
		}
		if userFind && userlocalID != "" {
			break
		}
	}
	
	if userlocalID == "" {
		return nil, fmt.Errorf("Rancher user for %s not found in %d searched users", principalID, len(items))
	}
	
	fleetworkspace.Annotations["gorizond-user." + userlocalID + "." + role] = time.Now().UTC().GoString()
	delete(fleetworkspace.Annotations, annotationKey)
	return fleetWorkspaces.Update(fleetworkspace)
}