package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	kvstore "github.com/shyim/tanjun/kv-store"
	"io"
)

type KvClient struct {
	resp types.HijackedResponse
	pr   *io.PipeReader
}

func (c KvClient) Get(key string) (string, error) {
	payload := kvstore.KVInput{Operation: "get", Key: key}

	encoded, err := json.Marshal(payload)

	if err != nil {
		return "", err
	}

	encoded = append(encoded, []byte("\n")...)

	if _, err := c.resp.Conn.Write(encoded); err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(c.pr)

	scanner.Scan()

	var res kvstore.KVResponse

	if err := json.Unmarshal(scanner.Bytes(), &res); err != nil {
		return "", err
	}

	if res.ErrorMessage != "" {
		return "", fmt.Errorf("%s", res.ErrorMessage)
	}

	return res.Value, nil
}

func (c KvClient) Delete(key string) error {
	payload := kvstore.KVInput{Operation: "del", Key: key}

	encoded, err := json.Marshal(payload)

	if err != nil {
		return err
	}

	encoded = append(encoded, []byte("\n")...)

	if _, err := c.resp.Conn.Write(encoded); err != nil {
		return err
	}

	scanner := bufio.NewScanner(c.pr)
	scanner.Scan()

	var res kvstore.KVResponse

	if err := json.Unmarshal(scanner.Bytes(), &res); err != nil {
		return err
	}

	if res.ErrorMessage != "" {
		return fmt.Errorf("%s", res.ErrorMessage)
	}

	return nil
}

func (c KvClient) Set(key string, value string) error {
	payload := kvstore.KVInput{Operation: "set", Key: key, Value: value}

	encoded, err := json.Marshal(payload)

	if err != nil {
		return err
	}

	encoded = append(encoded, []byte("\n")...)

	if _, err := c.resp.Conn.Write(encoded); err != nil {
		return err
	}

	scanner := bufio.NewScanner(c.pr)
	scanner.Scan()

	var res kvstore.KVResponse

	if err := json.Unmarshal(scanner.Bytes(), &res); err != nil {
		return err
	}

	if res.ErrorMessage != "" {
		return fmt.Errorf("%s", res.ErrorMessage)
	}

	return nil
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
		Cmd:          []string{"/kv-store"},
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
		_, _ = stdcopy.StdCopy(pw, io.Discard, resp.Reader)
	}()

	return &KvClient{
		resp: resp,
		pr:   pr,
	}, nil
}
