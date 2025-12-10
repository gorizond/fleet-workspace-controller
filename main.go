package main

import (
	"flag"
	"github.com/gorizond/fleet-workspace-controller/controllers"
	"github.com/gorizond/fleet-workspace-controller/pkg/generated/controllers/management.cattle.io"
	"github.com/rancher/lasso/pkg/log"
	"github.com/rancher/wrangler/v3/pkg/kubeconfig"
	"github.com/rancher/wrangler/v3/pkg/signals"
	"github.com/rancher/wrangler/v3/pkg/start"
	"k8s.io/client-go/rest"
)

func main() {
	var kubeconfig_file string
	flag.StringVar(&kubeconfig_file, "kubeconfig", "", "Path to kubeconfig")
	flag.Parse()

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Infof("Error getting kubernetes config: %v", err)
		config, err = kubeconfig.GetNonInteractiveClientConfig(kubeconfig_file).ClientConfig()
		if err != nil {
			panic(err)
		}
		log.Infof("Using kubeconfig file")
	}

	factory, err := management.NewFactoryFromConfig(config)
	if err != nil {
		log.Errorf("Failed to create management factory: %v", err)
	}

	ctx := signals.SetupSignalContext()
	// Initialize controllers
	controllers.InitUserController(ctx, factory)
	controllers.InitFleetWorkspaceController(ctx, factory)
	controllers.InitGlobalRoleBindingController(ctx, factory)
	controllers.InitGlobalRoleBindingTTLController(ctx, factory)
	controllers.InitUserWorkspaceGuard(ctx, factory)
	// Start controllers
	if err := start.All(ctx, 10, factory); err != nil {
		panic(err)
	}

	<-ctx.Done()
}
