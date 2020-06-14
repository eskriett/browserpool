package browserpool

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	debugPort      = 9222
	dockerImage    = "zenika/alpine-chrome"
	imagePullLimit = time.Minute
)

type Browser struct {
	CreationTime time.Time
	DebugURL     string
	ID           string
	Port         int

	dockerCLI *client.Client
}

// NewBrowser creates a new browser.
func NewBrowser() (*Browser, error) {
	b := &Browser{}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	b.dockerCLI = cli

	return b, nil
}

// Spins up a new browser.
func (b *Browser) Launch(ctx context.Context) error {
	if err := b.pullImage(ctx); err != nil {
		return err
	}

	if err := b.createContainer(ctx); err != nil {
		return err
	}

	if err := b.startContainer(ctx); err != nil {
		return err
	}

	debugURL, err := getDebugURL(b.Port)
	if err != nil {
		return err
	}

	b.DebugURL = debugURL

	if err := b.dockerCLI.Close(); err != nil {
		return err
	}

	b.CreationTime = time.Now()

	return nil
}

// Close closes the browser.
func (b *Browser) Close(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	return cli.ContainerRemove(ctx, b.ID, types.ContainerRemoveOptions{Force: true})
}

func (b *Browser) createContainer(ctx context.Context) error {
	tcpPort := nat.Port(fmt.Sprintf("%d/tcp", debugPort))

	// Describe the operations of the container
	containerConfig := &container.Config{
		Image: dockerImage,
		ExposedPorts: nat.PortSet{
			tcpPort: struct{}{},
		},
		Cmd: []string{
			"--remote-debugging-address=0.0.0.0",
			fmt.Sprintf("--remote-debugging-port=%d", debugPort),
		},
	}

	// Load chrome seccomp profile for running headless chrome securely
	chromeSec, err := ioutil.ReadFile("chrome.json")
	if err != nil {
		return err
	}

	port, err := getFreePort()
	if err != nil {
		return err
	}

	b.Port = port

	hostConfig := &container.HostConfig{
		AutoRemove: true,
		PortBindings: nat.PortMap{
			tcpPort: []nat.PortBinding{
				{
					HostIP:   "",
					HostPort: strconv.Itoa(port),
				},
			},
		},
		SecurityOpt: []string{"seccomp=" + string(chromeSec)},
	}

	resp, err := b.dockerCLI.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return err
	}

	b.ID = resp.ID

	return nil
}

func getDebugURL(port int) (string, error) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/json/version", port))
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	var result map[string]interface{}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result["webSocketDebuggerUrl"].(string), nil
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}

	port := l.Addr().(*net.TCPAddr).Port

	return port, l.Close()
}

func (b *Browser) pullImage(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, imagePullLimit)
	defer cancel()

	reader, err := b.dockerCLI.ImagePull(ctx, dockerImage, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	r := bufio.NewReader(reader)

	for {
		_, err := r.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}
	}

	return reader.Close()
}

func (b *Browser) startContainer(ctx context.Context) error {
	if err := b.dockerCLI.ContainerStart(ctx, b.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	reader, err := b.dockerCLI.ContainerLogs(ctx, b.ID, types.ContainerLogsOptions{
		Follow:     true,
		ShowStderr: true,
	})
	if err != nil {
		return err
	}

	r := bufio.NewReader(reader)

	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}

		if bytes.Contains(line, []byte("ws://")) {
			break
		}
	}

	return reader.Close()
}
