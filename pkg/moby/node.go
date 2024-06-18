package moby

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
)

type node struct {
	container types.Container
}

func (n *node) Command(s string, s2 ...string) exec.Cmd {
	//TODO implement me
	panic("implement me")
}

func (n *node) CommandContext(ctx context.Context, s string, s2 ...string) exec.Cmd {
	//TODO implement me
	panic("implement me")
}

func (n *node) String() string {
	return n.container.ID
}

func (n *node) Role() (string, error) {
	return n.container.Labels[kindRoleLabel], nil
}

func (n *node) IP() (ipv4 string, ipv6 string, err error) {
	return IPs(context.Background(), n.container.ID)
}

func (n *node) SerialLogs(writer io.Writer) error {
	//TODO implement me
	panic("implement me")
}

var _ nodes.Node = &node{}
