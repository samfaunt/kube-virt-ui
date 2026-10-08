package api

import (
	"context"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kvui/internal/image"
	"kvui/internal/store"
)

func TestImageUpload(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	var received []string
	refusals := 2 // the upload server is not listening yet for the first attempts
	proxy := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if refusals > 0 {
			refusals--
			http.Error(w, "dial tcp: connect: connection refused", http.StatusBadGateway)
			return
		}
		body, _ := io.ReadAll(r.Body)
		received = append(received, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization")+" "+string(body))
	}))
	defer proxy.Close()
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: proxy.Certificate().Raw})
	e.k8s.CoreV1().ConfigMaps("cdi").Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cdi-uploadproxy-signer-bundle", Namespace: "cdi"},
		Data:       map[string]string{"ca-bundle.crt": string(caPEM)},
	}, metav1.CreateOptions{})
	e.api.UploadProxy = &image.UploadProxy{URL: proxy.URL, CAConfigMapNamespace: "cdi",
		CAConfigMapName: "cdi-uploadproxy-signer-bundle", CAConfigMapKey: "ca-bundle.crt", Kube: e.k8s}

	_, link, _ := CreateInvite(ctx, e.store, "https://ui.test", store.Invite{
		Namespace: "team-a", Role: "owner", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
	alice := e.client()
	e.redeem(alice, link, "alice")

	upload := func(name, body string) int {
		req, _ := http.NewRequest("POST", e.srv.URL+"/api/namespaces/team-a/images/"+name+"/upload", strings.NewReader(body))
		req.Header.Set("X-Requested-With", "kvui")
		resp, err := alice.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if st := upload("iso", "ISO-BYTES"); st != 204 {
		t.Fatalf("upload: %d", st)
	}
	// Retried after the refusals with the whole body intact.
	if refusals != 0 || len(received) != 1 || received[0] != "POST /v1beta1/upload-async Bearer upload-token ISO-BYTES" {
		t.Fatalf("proxy received %q", received)
	}
	// Only labelled images can be uploaded into, not arbitrary DataVolumes.
	if st := upload("web-root", "x"); st != 400 {
		t.Fatalf("upload into a VM disk: %d", st)
	}
	// The token was requested as alice.
	var tokenReq string
	for _, r := range e.requests {
		if strings.Contains(r, "uploadtokenrequests") {
			tokenReq = r
		}
	}
	if !strings.HasSuffix(tokenReq, "Bearer tok-team-a-kvui-u1") {
		t.Fatalf("token request: %q", tokenReq)
	}
}
