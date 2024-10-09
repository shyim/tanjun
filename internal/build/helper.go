package build

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/moby/buildkit/client/connhelper"
	"io"
	"net"
	"net/url"
	"time"
)

var activeDockerClient *client.Client

func init() {
	connhelper.Register("tanjun", Helper)
}

func Helper(u *url.URL) (*connhelper.ConnectionHelper, error) {
	attach, err := activeDockerClient.ContainerExecCreate(context.Background(), u.Hostname(), container.ExecOptions{
		Cmd:          []string{"buildctl", "dial-stdio"},
		AttachStdin:  true,
		AttachStdout: true,
	})

	if err != nil {
		return nil, err
	}

	conn, err := activeDockerClient.ContainerExecAttach(context.Background(), attach.ID, container.ExecAttachOptions{})

	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()

	go func() {
		stdcopy.StdCopy(pw, io.Discard, conn.Reader)
	}()

	converter := dockerNetConn{org: conn.Conn, pr: pr}

	return &connhelper.ConnectionHelper{
		ContextDialer: func(ctx context.Context, addr string) (net.Conn, error) {
			return converter, nil
		},
	}, nil
}

type dockerNetConn struct {
	org net.Conn
	pr  *io.PipeReader
}

func (d dockerNetConn) Read(b []byte) (n int, err error) {
	return d.pr.Read(b)
}

func (d dockerNetConn) Write(b []byte) (n int, err error) {
	return d.org.Write(b)
}

func (d dockerNetConn) Close() error {
	return d.org.Close()
}

func (d dockerNetConn) LocalAddr() net.Addr {
	return d.org.LocalAddr()
}

func (d dockerNetConn) RemoteAddr() net.Addr {
	return d.org.RemoteAddr()
}

func (d dockerNetConn) SetDeadline(t time.Time) error {
	return d.org.SetDeadline(t)
}

func (d dockerNetConn) SetReadDeadline(t time.Time) error {
	return d.org.SetReadDeadline(t)
}

func (d dockerNetConn) SetWriteDeadline(t time.Time) error {
	return d.org.SetWriteDeadline(t)
}
