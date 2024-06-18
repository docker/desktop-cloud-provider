package moby

import (
	"archive/tar"
	"context"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

func ListClusters(ctx context.Context) ([]string, error) {
	containers, err := ListByLabel(ctx, "io.x-k8s.kind.cluster")
	if err != nil {
		return nil, err
	}
	clusters := sets.Set[string]{}
	for _, c := range containers {
		clusters.Insert(c.Labels["io.x-k8s.kind.cluster"])
	}
	return sets.List(clusters), nil
}

const (
	kindClusterLabel     = "io.x-k8s.kind.cluster"
	kindRoleLabel        = "io.x-k8s.kind.role"
	kindRoleControlPlane = "control-plane"
)

func Nodes(ctx context.Context, cluster string) ([]nodes.Node, error) {
	containers, err := ListByLabel(ctx,
		fmt.Sprintf("%s=%s", kindClusterLabel, cluster),
	)
	if err != nil {
		return nil, err
	}
	var nodes []nodes.Node
	for _, container := range containers {
		nodes = append(nodes, &node{container})
	}
	return nodes, err
}

func ControlPlane(ctx context.Context, cluster string) (string, error) {
	containers, err := ListByLabel(ctx,
		fmt.Sprintf("%s=%s", kindClusterLabel, cluster),
		fmt.Sprintf("%s=%s", kindRoleLabel, kindRoleControlPlane),
	)
	if err != nil {
		return "", err
	}
	if len(containers) < 1 {
		return "", fmt.Errorf("failed to find control-plane node for cluster %s", cluster)
	}
	return containers[0].ID, nil
}

func KubeConfig(ctx context.Context, cluster string) ([]byte, error) {
	controlPlane, err := ControlPlane(ctx, cluster)
	if err != nil {
		return nil, err
	}
	stream, _, err := api.CopyFromContainer(ctx, controlPlane, "/etc/kubernetes/admin.conf")
	r := tar.NewReader(stream)
	_, err = r.Next()
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}
