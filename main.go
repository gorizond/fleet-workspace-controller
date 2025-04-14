package main

import (
	"flag"
	"github.com/gorizond/fleet-workspace-controller/controllers"
	
	"github.com/rancher/wrangler/v3/pkg/start"
	
	"github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io"
	"github.com/rancher/wrangler/v3/pkg/kubeconfig"
	"github.com/rancher/wrangler/v3/pkg/signals"
	"k8s.io/client-go/rest"
	"github.com/rancher/lasso/pkg/log"
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
	// users := factory.Management().V3().User()

	ctx := signals.SetupSignalContext()
	// Initialize controllers
	controllers.InitUserController(ctx, factory)
	controllers.InitFleetWorkspaceController(ctx, factory)
	controllers.InitGlobalRoleBindingController(ctx, factory)
	// Start controllers
	if err := start.All(ctx, 1, factory); err != nil {
		panic(err)
	}

	<-ctx.Done()
}