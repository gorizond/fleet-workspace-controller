// Code generated by controller-gen. DO NOT EDIT.

// +k8s:deepcopy-gen=package
// +groupName=management.cattle.io
package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GlobalRoleBindingList is a list of GlobalRoleBinding resources
type GlobalRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GlobalRoleBinding `json:"items"`
}

func NewGlobalRoleBinding(namespace, name string, obj GlobalRoleBinding) *GlobalRoleBinding {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("GlobalRoleBinding").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FleetWorkspaceList is a list of FleetWorkspace resources
type FleetWorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []FleetWorkspace `json:"items"`
}

func NewFleetWorkspace(namespace, name string, obj FleetWorkspace) *FleetWorkspace {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("FleetWorkspace").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UserList is a list of User resources
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []User `json:"items"`
}

func NewUser(namespace, name string, obj User) *User {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("User").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GlobalRoleList is a list of GlobalRole resources
type GlobalRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GlobalRole `json:"items"`
}

func NewGlobalRole(namespace, name string, obj GlobalRole) *GlobalRole {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("GlobalRole").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PrincipalList is a list of Principal resources
type PrincipalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Principal `json:"items"`
}

func NewPrincipal(namespace, name string, obj Principal) *Principal {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("Principal").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}
