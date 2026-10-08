// Package tenanttest provides a fake clientset for code using tenant.Provisioner.
package tenanttest

import (
	"time"

	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// Namespace returns a namespace, labelled as a tenant if tenant is true.
func Namespace(name string, tenant bool) *corev1.Namespace {
	n := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{}}}
	if tenant {
		n.Labels["kubevirt-ui.io/tenant"] = "true"
	}
	return n
}

// FakeClient returns a fake clientset that answers TokenRequests, which the
// default object tracker does not implement.
func FakeClient(objs ...runtime.Object) *fake.Clientset {
	c := fake.NewClientset(objs...)
	c.PrependReactor("create", "serviceaccounts", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if a.GetSubresource() != "token" {
			return false, nil, nil
		}
		name := a.(k8stesting.CreateActionImpl).Name
		if _, err := c.Tracker().Get(corev1.SchemeGroupVersion.WithResource("serviceaccounts"), a.GetNamespace(), name); err != nil {
			return true, nil, err
		}
		return true, &authv1.TokenRequest{Status: authv1.TokenRequestStatus{
			Token: "tok-" + a.GetNamespace() + "-" + name, ExpirationTimestamp: metav1.NewTime(time.Now().Add(10 * time.Minute)),
		}}, nil
	})
	return c
}
