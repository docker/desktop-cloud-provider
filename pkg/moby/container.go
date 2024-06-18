package moby

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"sigs.k8s.io/cloud-provider-kind/pkg/constants"
)

var api client.APIClient

func init() {
	cli, _ := command.NewDockerCli()
	_ = cli.Initialize(flags.NewClientOptions())
	api = cli.Client()
}

func Logs(ctx context.Context, name string, w io.Writer) error {
	logs, err := api.ContainerLogs(ctx, name, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return err
	}
	_, err = io.Copy(w, logs)
	return err
}

func LogDump(ctx context.Context, containerName string, fileName string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	err = Logs(ctx, containerName, f)
	if err != nil {
		return err
	}
	return nil
}

func Create(ctx context.Context, name string, config *container.Config, net *network.NetworkingConfig, host *container.HostConfig) (string, error) {
	created, err := api.ContainerCreate(ctx, config, host, net, nil, name)
	if err != nil {
		return "", err
	}
	return created.ID, err
}

func Start(ctx context.Context, name string) error {
	return api.ContainerStart(ctx, name, container.StartOptions{})
}

func Delete(ctx context.Context, name string) error {
	return api.ContainerRemove(ctx, name, container.RemoveOptions{Force: true})
}

func IsRunning(ctx context.Context, name string) (bool, error) {
	inspect, err := api.ContainerInspect(ctx, name)
	if err != nil {
		return false, err
	}
	return inspect.State.Running, nil
}

func Copy(ctx context.Context, name string, files map[string][]byte) error {
	b := bytes.Buffer{}
	tarWriter := tar.NewWriter(&b)

	for path, content := range files {
		mode := int64(0o777)
		err := tarWriter.WriteHeader(&tar.Header{
			Name:    path,
			Size:    int64(len(content)),
			ModTime: time.Now(),
			Mode:    mode,
		})
		if err != nil {
			return err
		}
		_, err = tarWriter.Write(content)
		if err != nil {
			return err
		}
	}
	err := tarWriter.Close()
	if err != nil {
		return err
	}

	return api.CopyToContainer(ctx, name, "/", &b, types.CopyToContainerOptions{})
}

func IPs(ctx context.Context, name string) (ipv4 string, ipv6 string, err error) {
	inspect, err := api.ContainerInspect(ctx, name)
	if err != nil {
		return "", "", err
	}
	settings, ok := inspect.NetworkSettings.Networks[constants.FixedNetworkName]
	if !ok {
		return "", "", fmt.Errorf("contianer is not connected on network %s", constants.FixedNetworkName)
	}
	return settings.IPAddress, settings.GlobalIPv6Address, nil
}

func ListByLabel(ctx context.Context, label string) ([]types.Container, error) {
	return api.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("label", label)),
	})
}
