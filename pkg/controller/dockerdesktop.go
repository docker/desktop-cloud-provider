package controller

import (
	"fmt"

	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
)

// getKubeAPIEndpoint returns the kube-apiserver endpoint of the cluster using the IP of the node
// serving it on the kind network (Docker Desktop specific, see getKubeConfig).
func (c *Controller) getKubeAPIEndpoint(cluster string) (string, error) {
	n, err := c.kind.ListNodes(cluster)
	if err != nil {
		return "", fmt.Errorf("failed to list cluster nodes %s: %w", cluster, err)
	}
	node, err := nodeutils.APIServerEndpointNode(n)
	if err != nil {
		return "", fmt.Errorf("failed to select the API server node of cluster %s: %w", cluster, err)
	}
	ipv4, _, err := node.IP()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve the API server node IP of cluster %s: %w", cluster, err)
	}

	// kind's APIServerInternalPort is in an internal package
	return fmt.Sprintf("https://%s:6443", ipv4), nil
}
