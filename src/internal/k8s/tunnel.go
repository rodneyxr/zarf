package k8s

// Forked from https://github.com/gruntwork-io/terratest/blob/v0.38.8/modules/k8s/tunnel.go

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Global lock to synchronize port selections
var globalMutex sync.Mutex

const (
	PodResource  = "pod"
	SvcResource  = "svc"
	ZarfRegistry = "REGISTRY"
	ZarfLogging  = "LOGGING"
	ZarfGit      = "GIT"
)

// makeLabels is a helper to format a map of label key and value pairs into a single string for use as a selector.
func makeLabels(labels map[string]string) string {
	var out []string
	for key, value := range labels {
		out = append(out, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(out, ",")
}

// Tunnel is the main struct that configures and manages port forwading tunnels to Kubernetes resources.
type Tunnel struct {
	out          io.Writer
	autoOpen     bool
	localPort    int
	remotePort   int
	namespace    string
	resourceType string
	resourceName string
	urlSuffix    string
	stopChan     chan struct{}
	readyChan    chan struct{}
	spinner      *message.Spinner
}

// NewTunnel will create a new Tunnel struct
// Note that if you use 0 for the local port, an open port on the host system
// will be selected automatically, and the Tunnel struct will be updated with the selected port.
func NewTunnel(namespace, resourceType, resourceName string, local, remote int) *Tunnel {
	message.Debugf("tunnel.NewTunnel(%s, %s, %s, %d, %d)", namespace, resourceType, resourceName, local, remote)
	return &Tunnel{
		out:          ioutil.Discard,
		localPort:    local,
		remotePort:   remote,
		namespace:    namespace,
		resourceType: resourceType,
		resourceName: resourceName,
		stopChan:     make(chan struct{}, 1),
		readyChan:    make(chan struct{}, 1),
	}
}

func NewZarfTunnel() *Tunnel {
	return NewTunnel(ZarfNamespace, SvcResource, "", 0, 0)
}

func (tunnel *Tunnel) EnableAutoOpen() {
	tunnel.autoOpen = true
}

func (tunnel *Tunnel) AddSpinner(spinner *message.Spinner) {
	tunnel.spinner = spinner
}

func (tunnel *Tunnel) Connect(target string, blocking bool) {
	message.Debugf("tunnel.Connect(%s, %#v)", target, blocking)
	switch strings.ToUpper(target) {
	case ZarfRegistry:
		tunnel.resourceName = "zarf-docker-registry"
		tunnel.remotePort = 5000
	case ZarfLogging:
		tunnel.resourceName = "zarf-loki-stack-grafana"
		tunnel.remotePort = 3000
	case ZarfGit:
		tunnel.resourceName = "zarf-gitea-http"
		tunnel.remotePort = 3000
	default:
		if target != "" {
			if err := tunnel.checkForZarfConnectLabel(target); err != nil {
				message.Errorf(err, "Problem looking for a zarf connect label in the cluster")
			}
		}

		if tunnel.resourceName == "" {
			message.Fatalf(nil, "Ensure a resource name is provided")
		}
		if tunnel.remotePort < 1 {
			message.Fatal(nil, "A remote port must be specified to connect to.")
		}
	}

	url, err := tunnel.Establish()

	// On error abort
	if err != nil {
		message.Fatal(err, "Unable to establish the tunnel")
	}

	if blocking {
		// Otherwise, if this is blocking it is coming from a user request so try to open the URL, but ignore errors
		if tunnel.autoOpen {
			switch runtime.GOOS {
			case "linux":
				_ = exec.Command("xdg-open", url).Start()
			case "windows":
				_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
			case "darwin":
				_ = exec.Command("open", url).Start()
			}
		} else {
			// Dump the tunnel URL to the console for other tools if not auto-opening
			fmt.Print(url)
		}

		// Dump the tunnel URL to the console for other tools to use
		fmt.Print(url)

		// Since this blocking, set the defer now so it closes properly on sigterm
		defer tunnel.Close()

		// Keep this open until an interrupt signal is received
		c := make(chan os.Signal)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			os.Exit(0)
		}()

		for {
			runtime.Gosched()
		}
	}
}

// Endpoint returns the tunnel endpoint
func (tunnel *Tunnel) Endpoint() string {
	message.Debug("tunnel.Endpoint()")
	return fmt.Sprintf("127.0.0.1:%d", tunnel.localPort)
}

// Close disconnects a tunnel connection by closing the StopChan, thereby stopping the goroutine.
func (tunnel *Tunnel) Close() {
	message.Debug("tunnel.Close()")
	close(tunnel.stopChan)
}

func (tunnel *Tunnel) checkForZarfConnectLabel(name string) error {
	message.Debugf("tunnel.checkForZarfConnectLabel(%s)", name)
	matches, err := GetServicesByLabel("", config.ZarfConnectLabelName, name)
	if err != nil {
		return fmt.Errorf("unable to lookup the service: %w", err)
	}

	if len(matches.Items) > 0 {
		// If there is a match, use the first one as these are supposed to be unique
		svc := matches.Items[0]

		// Reset based on the matched params
		tunnel.resourceType = SvcResource
		tunnel.resourceName = svc.Name
		tunnel.namespace = svc.Namespace
		// Only support a service with a single port
		tunnel.remotePort = svc.Spec.Ports[0].TargetPort.IntValue()

		// Add the url suffix too
		tunnel.urlSuffix = svc.Annotations[config.ZarfConnectAnnotationUrl]

		message.Debugf("tunnel connection match: %s/%s on port %d", svc.Namespace, svc.Name, tunnel.remotePort)
	}

	return nil
}

// Establish opens a tunnel to a kubernetes resource, as specified by the provided tunnel struct.
func (tunnel *Tunnel) Establish() (string, error) {
	message.Debug("tunnel.Establish()")

	var spinner *message.Spinner

	spinnerMessage := fmt.Sprintf("Creating a port forwarding tunnel for resource %s/%s in namespace %s routing local port %d to remote port %d",
		tunnel.resourceType,
		tunnel.resourceName,
		tunnel.namespace,
		tunnel.localPort,
		tunnel.remotePort,
	)

	if tunnel.spinner != nil {
		spinner = tunnel.spinner
		spinner.Updatef(spinnerMessage)
	} else {
		spinner = message.NewProgressSpinner(spinnerMessage)
		defer spinner.Stop()
	}

	// Find the pod to port forward to
	podName, err := tunnel.getAttachablePodForResource()
	if err != nil {
		return "", fmt.Errorf("unable to find pod attached to given resource: %w", err)
	}
	spinner.Debugf("Selected pod %s to open port forward to", podName)

	clientset := getClientset()

	// Build url to the port forward endpoint
	// example: http://localhost:8080/api/v1/namespaces/helm/pods/tiller-deploy-9itlq/portforward
	postEndpoint := clientset.CoreV1().RESTClient().Post()
	namespace := tunnel.namespace
	portForwardCreateURL := postEndpoint.
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward").
		URL()

	spinner.Debugf("Using URL %s to create portforward", portForwardCreateURL)

	restConfig := getRestConfig()

	// Construct the spdy client required by the client-go portforward library
	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return "", fmt.Errorf("unable to create the spdy client %w", err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", portForwardCreateURL)

	// If the local-port is 0, get an available port before continuing. We do this here instead of relying on the
	// underlying port-forwarder library, because the port-forwarder library does not expose the selected local port in a
	// machine-readable manner.
	// Synchronize on the global lock to avoid race conditions with concurrently selecting the same available port,
	// since there is a brief moment between `GetAvailablePort` and `forwarder.ForwardPorts` where the selected port
	// is available for selection again.
	if tunnel.localPort == 0 {
		spinner.Debugf("Requested local port is 0. Selecting an open port on host system")
		tunnel.localPort, err = GetAvailablePort()
		if err != nil {
			return "", fmt.Errorf("unable to find an available port: %w", err)
		}
		spinner.Debugf("Selected port %d", tunnel.localPort)
		globalMutex.Lock()
		defer globalMutex.Unlock()
	}

	// Construct a new PortForwarder struct that manages the instructed port forward tunnel
	ports := []string{fmt.Sprintf("%d:%d", tunnel.localPort, tunnel.remotePort)}
	portforwarder, err := portforward.New(dialer, ports, tunnel.stopChan, tunnel.readyChan, tunnel.out, tunnel.out)
	if err != nil {
		return "", fmt.Errorf("unable to create the port forward: %w", err)
	}

	// Open the tunnel in a goroutine so that it is available in the background. Report errors to the main goroutine via
	// a new channel.
	errChan := make(chan error)
	go func() {
		errChan <- portforwarder.ForwardPorts()
	}()

	// Wait for an error or the tunnel to be ready
	select {
	case err = <-errChan:
		if tunnel.spinner == nil {
			spinner.Stop()
		}
		return "", fmt.Errorf("unable to start the tunnel: %w", err)
	case <-portforwarder.Ready:
		url := fmt.Sprintf("http://%s:%d%s", config.IPV4Localhost, tunnel.localPort, tunnel.urlSuffix)
		msg := fmt.Sprintf("Creating port forwarding tunnel at %s", url)
		if tunnel.spinner == nil {
			spinner.Successf(msg)
		} else {
			spinner.Updatef(msg)
		}
		return url, nil
	}
}

// GetAvailablePort retrieves an available port on the host machine. This delegates the port selection to the golang net
// library by starting a server and then checking the port that the server is using.
func GetAvailablePort() (int, error) {
	message.Debug("tunnel.GetAvailablePort()")
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer func(l net.Listener) {
		// ignore this error because it won't help us to tell the user
		_ = l.Close()
	}(l)

	_, p, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return 0, err
	}
	return port, err
}

// getAttachablePodForResource will find a pod that can be port forwarded to the provided resource type and return
// the name.
func (tunnel *Tunnel) getAttachablePodForResource() (string, error) {
	message.Debug("tunnel.GettAttachablePodForResource()")
	switch tunnel.resourceType {
	case PodResource:
		return tunnel.resourceName, nil
	case SvcResource:
		return tunnel.getAttachablePodForService()
	default:
		return "", fmt.Errorf("unknown resource type: %s", tunnel.resourceType)
	}
}

// getAttachablePodForServiceE will find an active pod associated with the Service and return the pod name.
func (tunnel *Tunnel) getAttachablePodForService() (string, error) {
	message.Debug("tunnel.getAttachablePodForService()")
	service, err := GetService(tunnel.namespace, tunnel.resourceName)
	if err != nil {
		return "", fmt.Errorf("unable to find the service: %w", err)
	}
	selectorLabelsOfPods := makeLabels(service.Spec.Selector)

	servicePods := WaitForPodsAndContainers(types.ZarfContainerTarget{
		Namespace: tunnel.namespace,
		Selector:  selectorLabelsOfPods,
	}, false)

	return servicePods[0], nil
}
