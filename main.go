package main

import (
    "flag"
    "net/http"
    "os"

    "github.com/gorizond/fleet-workspace-controller/controllers"
    "github.com/gorizond/fleet-workspace-controller/pkg/generated/controllers/management.cattle.io"
    "github.com/rancher/lasso/pkg/log"
    "github.com/rancher/wrangler/v3/pkg/kubeconfig"
    "github.com/rancher/wrangler/v3/pkg/signals"
    "github.com/rancher/wrangler/v3/pkg/start"
    "k8s.io/client-go/rest"
    "k8s.io/klog/v2"
)

// warnLoggingRT logs Warning headers with request context for tracing unknown field issues.
type warnLoggingRT struct{ rt http.RoundTripper }

func (w *warnLoggingRT) RoundTrip(req *http.Request) (*http.Response, error) {
    resp, err := w.rt.RoundTrip(req)
    if err != nil {
        return resp, err
    }
    if ws := resp.Header.Values("Warning"); len(ws) > 0 {
        for _, wmsg := range ws {
            log.Infof("API warning method=%s url=%s warning=%q", req.Method, req.URL.String(), wmsg)
        }
    }
    return resp, nil
}

func init() {
    // Emit server warnings with context to pinpoint `unknown field "spec"` origin.
    klog.InitFlags(nil)
    rest.SetDefaultWarningHandler(rest.NewWarningWriter(os.Stderr, rest.WarningWriterOptions{
        Deduplicate: false,
        Color:       true,
    }))
}

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

    // Wrap transport to log Warning headers with request details.
    config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
        if rt == nil {
            rt = http.DefaultTransport
        }
        return &warnLoggingRT{rt: rt}
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
    // controllers.InitUserWorkspaceGuard(ctx, factory)
    // Start controllers
    if err := start.All(ctx, 10, factory); err != nil {
        panic(err)
    }

    <-ctx.Done()
}
