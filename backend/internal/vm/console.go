package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// ConsoleKinds are the KubeVirt streaming subresources the UI exposes:
// the graphical VNC display and the serial console.
var ConsoleKinds = map[string]bool{"vnc": true, "console": true}

// DialConsole opens KubeVirt's websocket for a VM instance's VNC display or
// serial console, authenticating with cfg (the user's credentials).
// Handshake refusals are returned as Kubernetes status errors.
func DialConsole(ctx context.Context, cfg *rest.Config, namespace, name, kind string) (*websocket.Conn, error) {
	if !ConsoleKinds[kind] {
		return nil, fmt.Errorf("unknown console kind %q", kind)
	}
	u, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	}
	u = u.JoinPath("/apis/subresources.kubevirt.io/v1/namespaces", namespace, "virtualmachineinstances", name, kind)

	tlsConfig, err := rest.TLSConfigFor(cfg)
	if err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
		Subprotocols:    []string{"plain.kubevirt.io"},
	}
	header := http.Header{"Authorization": {"Bearer " + cfg.BearerToken}}
	conn, resp, err := dialer.DialContext(ctx, u.String(), header)
	if err != nil && resp != nil {
		defer resp.Body.Close()
		return nil, statusError(resp)
	}
	return conn, err
}

func statusError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var status metav1.Status
	if json.Unmarshal(body, &status) != nil || status.Message == "" {
		status.Message = fmt.Sprintf("console handshake failed: %s", resp.Status)
	}
	status.Status = metav1.StatusFailure
	status.Code = int32(resp.StatusCode)
	return &apierrors.StatusError{ErrStatus: status}
}
