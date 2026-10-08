package tenant_test

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"kvui/internal/tenant"
	"kvui/internal/tenant/tenanttest"
)

func TestEnsureRoleChangeAndRemove(t *testing.T) {
	ctx := context.Background()
	c := tenanttest.FakeClient(tenanttest.Namespace("team-a", true))
	p := tenant.NewProvisioner(c, &rest.Config{Host: "https://k8s"})

	if err := p.Ensure(ctx, "team-a", 7, "alice", "viewer"); err != nil {
		t.Fatal(err)
	}
	if err := p.Ensure(ctx, "team-a", 7, "alice", "owner"); err != nil {
		t.Fatal(err)
	}
	rb, err := c.RbacV1().RoleBindings("team-a").Get(ctx, "kvui-u7", metav1.GetOptions{})
	if err != nil || rb.RoleRef.Name != "kubevirt-ui-owner" || rb.Subjects[0].Name != "kvui-u7" {
		t.Fatalf("rolebinding: %+v %v", rb, err)
	}

	cfg, err := p.UserConfig(ctx, "team-a", 7, "alice", "owner")
	if err != nil || cfg.BearerToken != "tok-team-a-kvui-u7" || cfg.Host != "https://k8s" {
		t.Fatalf("user config: %+v %v", cfg, err)
	}

	if err := p.Remove(ctx, "team-a", 7); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CoreV1().ServiceAccounts("team-a").Get(ctx, "kvui-u7", metav1.GetOptions{}); err == nil {
		t.Fatal("serviceaccount not removed")
	}
}

func TestRefusesNonTenantNamespace(t *testing.T) {
	ctx := context.Background()
	p := tenant.NewProvisioner(tenanttest.FakeClient(tenanttest.Namespace("kube-system", false)), &rest.Config{})
	if err := p.Ensure(ctx, "kube-system", 1, "x", "owner"); !errors.Is(err, tenant.ErrNotTenant) {
		t.Fatalf("kube-system: %v", err)
	}
	if err := p.Ensure(ctx, "missing", 1, "x", "owner"); !errors.Is(err, tenant.ErrNotTenant) {
		t.Fatalf("missing: %v", err)
	}
}

func TestUserConfigReprovisions(t *testing.T) {
	ctx := context.Background()
	c := tenanttest.FakeClient(tenanttest.Namespace("team-a", true))
	p := tenant.NewProvisioner(c, &rest.Config{})
	// No Ensure beforehand: UserConfig must create the ServiceAccount itself.
	if _, err := p.UserConfig(ctx, "team-a", 3, "bob", "viewer"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RbacV1().RoleBindings("team-a").Get(ctx, "kvui-u3", metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
}
