package controllers

import (
	"context"
	"strconv"
	"time"

	v3 "github.com/gorizond/fleet-workspace-controller/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/lasso/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	managementGlobalRoleBinding "github.com/gorizond/fleet-workspace-controller/pkg/generated/controllers/management.cattle.io"
)

func InitGlobalRoleBindingTTLController(ctx context.Context, mgmt *managementGlobalRoleBinding.Factory) {
	globalRoleBindings := mgmt.Management().V3().GlobalRoleBinding()
	globalRoleBindings.OnChange(ctx, "gorizond-grb-ttl-controller", func(key string, obj *v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error) {
		if obj == nil {
			return nil, nil
		}

		// Check for the gorizond-ttl label
		ttlLabel := obj.Labels["gorizond-ttl"]
		if ttlLabel == "" {
			return obj, nil
		}

		// Parse the TTL value
		ttlValue, err := strconv.Atoi(ttlLabel)
		if err != nil {
			log.Infof("Failed to parse gorizond-ttl label: %v", err)
			return obj, nil
		}

		// Calculate the expiration time
		expirationTime := obj.CreationTimestamp.Add(time.Duration(ttlValue) * time.Second)
		if time.Now().After(expirationTime) {
			// Delete the GlobalRoleBinding
			err := globalRoleBindings.Delete(obj.Name, &metav1.DeleteOptions{})
			if err != nil {
				log.Infof("Failed to delete GlobalRoleBinding %s: %v", obj.Name, err)
				return obj, nil
			}
			log.Infof("Deleted GlobalRoleBinding %s due to TTL expiration", obj.Name)
			return nil, nil
		}

		return obj, nil
	})
}
