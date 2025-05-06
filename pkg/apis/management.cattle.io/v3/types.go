package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rancherv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GlobalRoleBinding struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object metadata; More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// UserName is the name of the user subject to be bound. Immutable.
	// +optional
	UserName string `json:"userName,omitempty" norman:"noupdate,type=reference[user]"`

	// GroupPrincipalName is the name of the group principal subject to be bound. Immutable.
	// +optional
	GroupPrincipalName string `json:"groupPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`

	// UserPrincipalName is the name of the user principal subject to be bound. Immutable.
	// +optional
	UserPrincipalName string `json:"userPrincipalName,omitempty" norman:"noupdate,type=reference[principal]"`

	// GlobalRoleName is the name of the Global Role that the subject will be bound to. Immutable.
	// +kubebuilder:validation:Required
	GlobalRoleName string `json:"globalRoleName" norman:"required,noupdate,type=reference[globalRole]"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FleetWorkspace is a wrapper around rancher type
type FleetWorkspace rancherv3.FleetWorkspace

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// User is a wrapper around rancher type
type User rancherv3.User

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GlobalRole is a wrapper around rancher type
type GlobalRole rancherv3.GlobalRole

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GlobalRole is a wrapper around rancher type
type Principal rancherv3.Principal