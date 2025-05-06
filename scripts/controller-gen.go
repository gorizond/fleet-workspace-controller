package main

import (
	v3 "github.com/gorizond/fleet-workspace-controller/pkg/apis/management.cattle.io/v3"
	controllergen "github.com/rancher/wrangler/v3/pkg/controller-gen"
	"github.com/rancher/wrangler/v3/pkg/controller-gen/args"
)

func main() {
	controllergen.Run(args.Options{
		OutputPackage: "github.com/gorizond/fleet-workspace-controller/pkg/generated",
		Boilerplate:   "scripts/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"management.cattle.io": {
				PackageName: "management.cattle.io",
				Types: []interface{}{
					v3.GlobalRoleBinding{},
					v3.FleetWorkspace{},
					v3.User{},
					v3.GlobalRole{},
					v3.Principal{},
				},
				GenerateTypes: true,
			},
		},
	})
}