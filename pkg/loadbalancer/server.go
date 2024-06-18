package loadbalancer

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cloud-provider-kind/pkg/config"
	"sigs.k8s.io/cloud-provider-kind/pkg/constants"
	"sigs.k8s.io/cloud-provider-kind/pkg/moby"
)

type Server struct {
}

var _ cloudprovider.LoadBalancer = &Server{}

func NewServer() cloudprovider.LoadBalancer {
	return &Server{}
}

func (s *Server) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	// report status
	name := loadBalancerName(clusterName, service)
	ipv4, ipv6, err := moby.IPs(ctx, name)
	if err != nil {
		if strings.Contains(err.Error(), "failed to get container details") {
			return nil, false, nil
		}
		return nil, false, err
	}
	status := &v1.LoadBalancerStatus{}

	// process Ports
	portStatus := []v1.PortStatus{}
	for _, port := range service.Spec.Ports {
		portStatus = append(portStatus, v1.PortStatus{
			Port:     port.Port,
			Protocol: port.Protocol,
		})
	}

	// process IPs
	svcIPv4 := false
	svcIPv6 := false
	for _, family := range service.Spec.IPFamilies {
		if family == v1.IPv4Protocol {
			svcIPv4 = true
		}
		if family == v1.IPv6Protocol {
			svcIPv6 = true
		}
	}
	if ipv4 != "" && svcIPv4 {
		status.Ingress = append(status.Ingress, v1.LoadBalancerIngress{
			IP:     ipv4,
			IPMode: ptr.To(v1.LoadBalancerIPModeProxy),
			Ports:  portStatus,
		})
	}
	if ipv6 != "" && svcIPv6 {
		status.Ingress = append(status.Ingress, v1.LoadBalancerIngress{
			IP:     ipv6,
			IPMode: ptr.To(v1.LoadBalancerIPModeProxy),
			Ports:  portStatus,
		})
	}

	return status, true, nil
}

func (s *Server) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	return loadBalancerName(clusterName, service)
}

func (s *Server) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	name := loadBalancerName(clusterName, service)
	running, err := moby.IsRunning(ctx, name)
	klog.V(2).Infof("checked loadbalancer is running %t %s", running, err)
	exists := true
	if errdefs.IsNotFound(err) {
		exists = false
	} else if err != nil {
		return nil, err
	} else if err == nil && !running {
		klog.Infof("container %s for loadbalancer is not running", name)
		err := moby.Delete(ctx, name)
		if err != nil {
			return nil, err
		}
		exists = false
	}

	if !exists {
		klog.V(2).Infof("creating container for loadbalancer")

		err := s.createLoadBalancer(ctx, clusterName, service, nodes, proxyImage)
		if err != nil {
			return nil, err
		}
	} else {
		// update loadbalancer
		klog.V(2).Infof("updating loadbalancer")
		err = s.UpdateLoadBalancer(ctx, clusterName, service, nodes)
		if err != nil {
			return nil, err
		}
	}

	// get loadbalancer Status
	klog.V(2).Infof("get loadbalancer status")
	status, ok, err := s.GetLoadBalancer(ctx, clusterName, service)
	if !ok {
		return nil, fmt.Errorf("loadbalancer %s not found", name)
	}
	return status, err
}

func (s *Server) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	return proxyUpdateLoadBalancer(ctx, clusterName, service, nodes)
}

func (s *Server) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	containerName := loadBalancerName(clusterName, service)
	var err1, err2 error
	// Before deleting the load balancer store the logs if required
	if config.DefaultConfig.EnableLogDump {
		fileName := path.Join(config.DefaultConfig.LogDir, service.Namespace+"_"+service.Name+".log")
		klog.V(2).Infof("storing logs for loadbalancer %s on %s", containerName, fileName)
		if err := moby.LogDump(ctx, containerName, fileName); err != nil {
			klog.Infof("error trying to store logs for load balancer %s : %v", containerName, err)
		}
	}
	err2 = moby.Delete(ctx, containerName)
	return errors.Join(err1, err2)
}

// loadbalancer name is a unique name for the loadbalancer container
func loadBalancerName(clusterName string, service *v1.Service) string {
	hash := sha256.Sum256([]byte(loadBalancerSimpleName(clusterName, service)))
	encoded := base32.StdEncoding.EncodeToString(hash[:])
	name := constants.ContainerPrefix + "-" + encoded[:40]

	return name
}

func loadBalancerSimpleName(clusterName string, service *v1.Service) string {
	return clusterName + "/" + service.Namespace + "/" + service.Name
}

func ServiceFromLoadBalancerSimpleName(s string) (clusterName string, service *v1.Service) {
	slices := strings.Split(s, "/")
	if len(slices) != 3 {
		return
	}
	clusterName = slices[0]
	service = &v1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: slices[1], Name: slices[2]}}
	return
}

// createLoadBalancer create a docker container with a loadbalancer
func (s *Server) createLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node, image string) error {
	name := loadBalancerName(clusterName, service)
	networkName := constants.FixedNetworkName

	portBinding := map[nat.Port][]nat.PortBinding{}
	exposed := map[nat.Port]struct{}{}
	// Forward the Service Ports to the host so they are accessible on Mac and Windows
	for _, port := range service.Spec.Ports {
		if port.Protocol != v1.ProtocolTCP && port.Protocol != v1.ProtocolUDP {
			continue
		}

		p := nat.Port(fmt.Sprintf("%d/%s", port.Port, port.Protocol))
		portBinding[p] = []nat.PortBinding{
			{
				HostPort: strconv.Itoa(int(port.Port)),
			},
		}
		exposed[p] = struct{}{}
	}

	config := &container.Config{
		Tty:   true,
		Image: image,
		Labels: map[string]string{
			constants.NodeCCMLabelKey:          clusterName,
			constants.LoadBalancerNameLabelKey: loadBalancerSimpleName(clusterName, service),
		},
		Hostname: name,
		Cmd: strslice.StrSlice{
			"bash", "-c",
			// we need to override the default envoy configuration
			// https://www.envoyproxy.io/docs/envoy/latest/start/quick-start/configuration-dynamic-filesystem
			// envoy crashes in some circumstances, causing the container to restart, the problem is that the container
			// may come with a different IP and we don't update the status, we may do it, but applications does not use
			// to handle that the assigned LoadBalancerIP changes.
			// https://github.com/envoyproxy/envoy/issues/34195

			fmt.Sprintf(`while true; do envoy -c %s && break; sleep 1; done`, proxyConfigPath),
		},
		ExposedPorts: exposed,
	}

	host := &container.HostConfig{
		NetworkMode:   container.NetworkMode(networkName),
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyOnFailure},
		PortBindings:  portBinding,
	}

	net := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {},
		},
	}

	c, _ := json.Marshal(config)
	n, _ := json.Marshal(net)
	h, _ := json.Marshal(host)

	klog.V(2).Infof("creating loadbalancer %s %s %s", string(c), string(n), string(h))
	var id, err = moby.Create(ctx, name, config, net, host)
	if err != nil {
		return fmt.Errorf("failed to create containers %s: %w", name, err)
	}

	ldsConfig, cdsConfig, err := generateConfig(service, nodes)
	if err != nil {
		return err
	}

	err = moby.Copy(ctx, id, map[string][]byte{
		proxyConfigPath:    []byte(dynamicFilesystemConfig),
		proxyConfigPathLDS: []byte(ldsConfig),
		proxyConfigPathCDS: []byte(cdsConfig),
	})
	if err != nil {
		return err
	}

	return moby.Start(ctx, id)
}
