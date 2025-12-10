package controllers

import (
    "context"
    "fmt"
    "encoding/json"
    "strings"
    "time"

    managementv3 "github.com/gorizond/fleet-workspace-controller/pkg/apis/management.cattle.io/v3"
    "github.com/gorizond/fleet-workspace-controller/pkg/generated/controllers/management.cattle.io"
    "github.com/rancher/lasso/pkg/log"
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/types"
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

        fwName := fmt.Sprintf("%s%s-%d", workspacePrefix, obj.Name, time.Now().UnixNano())
        fleetworkspace := &managementv3.FleetWorkspace{
            ObjectMeta: metav1.ObjectMeta{
                Name: fwName,
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

        // Patch only metadata.annotations to avoid sending spec field to Rancher API.
        patch := map[string]interface{}{
            "metadata": map[string]interface{}{
                "annotations": obj.Annotations,
            },
        }
        body, err := json.Marshal(patch)
        if err != nil {
            return obj, err
        }

        if _, err := users.Patch(obj.Name, types.MergePatchType, body); err != nil {
            return obj, err
        }

        return obj, nil
    },
    )
}
