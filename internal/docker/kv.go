package docker

import (
	"bufio"
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"io"
)

type KvClient struct {
	resp types.HijackedResponse
	pr   *io.PipeReader
}

func (c KvClient) Get(key string) string {
	if _, err := c.resp.Conn.Write([]byte(fmt.Sprintf("GET %s\n", key))); err != nil {
		return ""
	}

	scanner := bufio.NewScanner(c.pr)

	scanner.Scan()

	return scanner.Text()
}

func (c KvClient) Set(key string, value string) bool {
	if _, err := c.resp.Conn.Write([]byte(fmt.Sprintf("SET '%s' '%s'\n", key, value))); err != nil {
		return false
	}

	scanner := bufio.NewScanner(c.pr)
	scanner.Scan()

	return scanner.Text() == "OK"
}

func (c KvClient) Delete(key string) bool {
	_, _ = c.resp.Conn.Write([]byte(fmt.Sprintf("DELETE '%s'\n", key)))

	scanner := bufio.NewScanner(c.pr)
	scanner.Scan()

	return scanner.Text() == "OK"
}

func (c KvClient) Close() {
	c.resp.Close()
	c.pr.Close()
}

func CreateKVConnection(ctx context.Context, client *client.Client) (*KvClient, error) {
	opts := container.ListOptions{
		Filters: filters.NewArgs(),
	}

	opts.Filters.Add("name", "tanjun-kv")

	containers, err := client.ContainerList(ctx, opts)

	if err != nil {
		return nil, err
	}

	if len(containers) != 1 {
		return nil, fmt.Errorf("expected 1 kv container, got %d", len(containers))
	}

	execId, err := client.ContainerExecCreate(ctx, containers[0].ID, container.ExecOptions{
		Tty:          false,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"valkey-cli"},
		Env:          []string{"TERM=dumb", "NO_COLOR=1"},
	})

	if err != nil {
		return nil, err
	}

	resp, err := client.ContainerExecAttach(ctx, execId.ID, container.ExecAttachOptions{})

	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()

	go func() {
		stdcopy.StdCopy(pw, io.Discard, resp.Reader)
	}()

	return &KvClient{
		resp: resp,
		pr:   pr,
	}, nil
}
