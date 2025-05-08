package controllers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	managementv3 "github.com/gorizond/fleet-workspace-controller/pkg/apis/management.cattle.io/v3"
	v3 "github.com/gorizond/fleet-workspace-controller/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/lasso/pkg/log"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createGlobalRoleBinding(mgmt v3.GlobalRoleBindingController, preffix, fleetworkspaceName string, annotationKey string) {
	parts := strings.SplitN(annotationKey[len(preffix):], ".", 2)
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

func createGlobalRoleBindingForGroup(mgmt v3.GlobalRoleBindingController, preffix, fleetworkspaceName string, annotationKey string, groupPrincipalName string) {
	parts := strings.SplitN(annotationKey[len(preffix):], ".", 2)
	GroupID := parts[0]
	role := parts[1]
	globalRoleBinding := &managementv3.GlobalRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gorizond-" + role + "-" + GroupID + "-" + fleetworkspaceName,
			Annotations: map[string]string{
				"gorizond-binding": annotationKey,
			},
			Labels: map[string]string{
				"fleet": fleetworkspaceName,
			},
		},
		GroupPrincipalName: groupPrincipalName,
		GlobalRoleName: "gorizond-" + role + "-" + fleetworkspaceName,
	}

	_, err := mgmt.Create(globalRoleBinding)
	if err != nil && !errors.IsAlreadyExists(err) {
		log.Infof("Failed to create global role binding: %v", err)
	}
}


func findByPrincipal(users v3.UserController, principal v3.PrincipalController, mgmt v3.GlobalRoleBindingController, fleetworkspace *managementv3.FleetWorkspace, fleetWorkspaces v3.FleetWorkspaceController, annotationKey string, annotationValue string) (*managementv3.FleetWorkspace, error) {
	parts := strings.SplitN(annotationKey[len("gorizond-principal."):], ".", 2)
	principalID := annotationValue
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

	principalObject, err := getLoginName(os.Getenv("RANCHER_URL"), os.Getenv("RANCHER_TOKEN"), principalID)
	if err != nil {
		return nil, fmt.Errorf("Failed to get principalID: %v", err)
	}

	if principalObject.PrincipalType == "group" {
		groupID := strings.Split(principalID, "://")[1]
		fleetworkspace.Annotations["gorizond-group." + groupID + "." + role] = annotationValue
	} else {
		userlocalID, lenItems, err := findUserByPrincipal(principalObject, principalID, role)
		if err != nil {
			return nil, err
		}
		if userlocalID == "" {
			fmt.Errorf("Rancher user for %s not found in %d searched users", principalID, lenItems)
		} else {
			fleetworkspace.Annotations["gorizond-user." + userlocalID + "." + role] = annotationValue
		}
	}
	delete(fleetworkspace.Annotations, annotationKey)
	return fleetWorkspaces.Update(fleetworkspace)
}

func findUserByPrincipal(principalObject Principal, principalID string, role string) (string, int, error) {
	log.Infof("Try find rancher user for %s", principalObject.LoginName)
	// find new NOT INIT users
	searchedUser1, err := findUserByUsername(os.Getenv("RANCHER_URL"), os.Getenv("RANCHER_TOKEN"), "/v3/users?username=")
	if err != nil {
		return "", 0, fmt.Errorf("Failed to find /v3/users?username=: %v", err)
	}
	log.Infof("Found username=empty %d",len(searchedUser1.Data))
	// find exist users with username=principal LoginName
	searchedUser2, err := findUserByUsername(os.Getenv("RANCHER_URL"), os.Getenv("RANCHER_TOKEN"), "/v3/users?username="+ strings.ToLower(principalObject.LoginName))
	if err != nil {
		return "", 0, fmt.Errorf("Failed to find /v3/users?username=principalObject.LoginName: %v", err)
	}
	log.Infof("Found username=%s %d",strings.ToLower(principalObject.LoginName), len(searchedUser2.Data))
	// find exist users with name=principal LoginName
	searchedUser3, err := findUserByUsername(os.Getenv("RANCHER_URL"), os.Getenv("RANCHER_TOKEN"), "/v3/users?name=" + strings.ToLower(principalObject.LoginName))
	if err != nil {
		return "", 0, fmt.Errorf("Failed to find /v3/users?name=principalObject.LoginName: %v", err)
	}
	log.Infof("Found name=%s %d",strings.ToLower(principalObject.LoginName), len(searchedUser3.Data))
	items := append(searchedUser3.Data, searchedUser2.Data...)
	// if by name not found may by its 'admin'?
	if len(items) == 0 {
		// try get admin
		admin, err := findUserByUsername(os.Getenv("RANCHER_URL"), os.Getenv("RANCHER_TOKEN"), "/v3/users?username=admin")
		if err != nil {
			return "", 0, fmt.Errorf("Failed to find /v3/users?username=admin: %v", err)
		}
		items = append(items, admin.Data...)
	}
	items = append(items, searchedUser1.Data...)
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
	return userlocalID, len(items), nil
}

type Principal struct {
    LoginName string `json:"loginName"`
    PrincipalType string `json:"principalType"`
}
type SearchedUser struct {
    LoginName string `json:"loginName"`
    PrincipalType string `json:"principalType"`
}

func getLoginName(rancherURL, token, principalID string) (Principal, error) {
    // Escape the principal identifier for use in the URL
    encodedID := url.PathEscape(principalID)
    endpoint := fmt.Sprintf("%s/v3/principals/%s", rancherURL, encodedID)

    // Create an HTTP client with certificate verification disabled (for example purposes)
    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
        },
    }

    var principal Principal
    // Formulate the HTTP request
    req, err := http.NewRequest("GET", endpoint, nil)
    if err != nil {
        return principal, fmt.Errorf("error creating request: %v", err)
    }

    // Add the authorization header
    req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

    // Execute the request
    resp, err := client.Do(req)
    if err != nil {
        return principal, fmt.Errorf("error executing request: %v", err)
    }
    defer resp.Body.Close()

    // Check the response status code
    if resp.StatusCode != http.StatusOK {
        return principal, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    // Read the response body
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return principal, fmt.Errorf("error reading response body: %v", err)
    }

    // Parse the JSON response
    if err := json.Unmarshal(body, &principal); err != nil {
        return principal, fmt.Errorf("error parsing JSON: %v", err)
    }

    return principal, nil
}

type User struct {
    ID         string `json:"id"`
    Username   string `json:"username"`
    Name       string `json:"name"`
    PrincipalIDs []string `json:"principalIds"`
}

type UserCollection struct {
    Data []User `json:"data"`
}

func findUserByUsername(rancherURL, token, query string) (*UserCollection, error) {
    // Escape the username for use in the URL
    endpoint := rancherURL + query

    // Create an HTTP client with certificate verification disabled (for example purposes)
    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
        },
    }

    // Formulate the HTTP request
    req, err := http.NewRequest("GET", endpoint, nil)
    if err != nil {
        return nil, fmt.Errorf("error creating request: %v", err)
    }

    // Add the authorization header
    req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

    // Execute the request
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("error executing request: %v", err)
    }
    defer resp.Body.Close()

    // Check the response status code
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    // Read the response body
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("error reading response body: %v", err)
    }

    // Parse the JSON response
    var users UserCollection
    if err := json.Unmarshal(body, &users); err != nil {
        return nil, fmt.Errorf("error parsing JSON: %v", err)
    }

    return &users, nil
}
