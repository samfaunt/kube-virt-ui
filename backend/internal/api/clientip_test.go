package api

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIP(t *testing.T) {
	ingress := []netip.Prefix{netip.MustParsePrefix("10.244.0.0/16")}
	cases := []struct {
		name    string
		trusted []netip.Prefix
		peer    string
		xff     []string
		want    string
	}{
		{"no proxies configured ignores header", nil, "203.0.113.9:1234", []string{"198.51.100.1"}, "203.0.113.9"},
		{"untrusted peer ignores header", ingress, "203.0.113.9:1234", []string{"198.51.100.1"}, "203.0.113.9"},
		{"trusted peer reports client", ingress, "10.244.1.5:1234", []string{"198.51.100.1"}, "198.51.100.1"},
		{"client-sent prefix is not believed", ingress, "10.244.1.5:1234", []string{"1.1.1.1, 198.51.100.1"}, "198.51.100.1"},
		{"multiple headers are merged", ingress, "10.244.1.5:1234", []string{"1.1.1.1", "198.51.100.1"}, "198.51.100.1"},
		{"chained trusted hops are skipped", ingress, "10.244.1.5:1234", []string{"198.51.100.1, 10.244.2.7"}, "198.51.100.1"},
		{"garbage stops at the last trusted hop", ingress, "10.244.1.5:1234", []string{"nonsense"}, "10.244.1.5"},
		{"trusted peer without header", ingress, "10.244.1.5:1234", nil, "10.244.1.5"},
		{"v4-mapped peer", ingress, "[::ffff:10.244.1.5]:1234", []string{"198.51.100.1"}, "198.51.100.1"},
		{"other client headers are ignored", ingress, "203.0.113.9:1234", nil, "203.0.113.9"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &Server{Options: Options{TrustedProxies: c.trusted}}
			r := httptest.NewRequest("POST", "/api/login", nil)
			r.RemoteAddr = c.peer
			r.Header.Set("True-Client-IP", "192.0.2.66")
			r.Header.Set("X-Real-IP", "192.0.2.66")
			for _, v := range c.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if got := s.clientIP(r).String(); got != c.want {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestRateKeyGroupsIPv6By64(t *testing.T) {
	s := &Server{}
	key := func(peer string) string {
		r := httptest.NewRequest("POST", "/api/login", nil)
		r.RemoteAddr = peer
		return s.rateKey(r)
	}
	if a, b := key("[2001:db8:1:2::1]:1"), key("[2001:db8:1:2:ffff::9]:1"); a != b {
		t.Errorf("same /64 keyed differently: %s vs %s", a, b)
	}
	if a, b := key("[2001:db8:1:2::1]:1"), key("[2001:db8:1:3::1]:1"); a == b {
		t.Errorf("different /64s share key %s", a)
	}
	if got := key("203.0.113.9:1"); got != "ip:203.0.113.9" {
		t.Errorf("IPv4 key = %s", got)
	}
}
