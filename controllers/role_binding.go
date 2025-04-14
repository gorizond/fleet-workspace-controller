package controllers

import (
    "github.com/rancher/lasso/pkg/log"
    "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io"
    managementv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createGlobalRoleBinding(mgmt *management.Factory, fleetworkspace *managementv3.FleetWorkspace, annotationKey, annotationValue string) {
    userID := annotationKey[len("gorizond-user."):len(annotationKey)-len("."+annotationValue)]
    role := annotationValue
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
        UserName:      userID,
        GlobalRoleName: "gorizond-" + role + "-" + fleetworkspace.Name,
    }

    _, err := mgmt.Management().V3().GlobalRoleBinding().Create(globalRoleBinding)
    if err != nil && !errors.IsAlreadyExists(err) {
        log.Infof("Failed to create global role binding: %v", err)
    }
}