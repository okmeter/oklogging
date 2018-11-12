package agent

import (
	"path"
	"strings"
	"fmt"
	"github.com/docker/docker/client"
	"context"
	"errors"
	"time"
)

const (
	dockerTimeout = 10 * time.Second
)

var (
	ErrSkip = errors.New("Skipped")
)

type LogLabels map[string]string

func getContainerId(logPath string) (string, error) {
	_, f := path.Split(logPath)
	if !strings.HasSuffix(f, "-json.log") {
		return "", fmt.Errorf("invalid log path format: %s (should be *-json.log)", logPath)
	}
	parts := strings.Split(f, "-")
	if len(parts) != 2 {
		return "", fmt.Errorf("can't get container id from log path: %s", logPath)
	}
	return parts[0], nil
}


func GetLabelsByLog(logPath string) (LogLabels, error) {
	containerId, err :=  getContainerId(logPath)
	if err != nil {
		return nil, err
	}
	docker, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	defer docker.Close()
	ctx, cancel := context.WithTimeout(context.Background(), dockerTimeout)
	defer cancel()

	client.WithVersion("1.12")(docker)
	ver, err := docker.ServerVersion(ctx)
	if err != nil {
		return nil, err
	}
	client.WithVersion(ver.APIVersion)(docker)
	container, err := docker.ContainerInspect(ctx, containerId)
	if err != nil {
		return nil, err
	}
	if container.Config.Labels["io.kubernetes.docker.type"] != "container" {
		return nil, ErrSkip
	}
	labels := LogLabels{
		"docker.name": strings.TrimLeft(container.Name, "/"),
	}
	return labels, nil
}

