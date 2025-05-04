package controllers

import (
	"github.com/rancher/lasso/pkg/log"
	managementv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

func createGlobalRoleBinding(mgmt *management.Factory, fleetworkspace *managementv3.FleetWorkspace, annotationKey string) {
	parts := strings.SplitN(annotationKey[len("gorizond-user."):], ".", 2)
	userID := parts[0]
	role := parts[1]
	globalRoleBinding := &managementv3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gorizond-" + role + "-" + userID + "-" + fleetworkspace.Name,
			Annotations: map[string]string{
				"gorizond-binding": annotationKey,
			},
			Labels: map[string]string{
				"fleet": fleetworkspace.Name,
			},
		},
		UserName:       userID,
		GlobalRoleName: "gorizond-" + role + "-" + fleetworkspace.Name,
	}

	_, err := mgmt.Management().V3().GlobalRoleBinding().Create(globalRoleBinding)
	if err != nil && !errors.IsAlreadyExists(err) {
		log.Infof("Failed to create global role binding: %v", err)
	}
}
