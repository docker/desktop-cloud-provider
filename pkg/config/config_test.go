package config

import "testing"

func TestOverrideProxyImage(t *testing.T) {
	t.Run("PROXY_IMAGE unset keeps the proxy image", func(t *testing.T) {
		t.Setenv("PROXY_IMAGE", "")
		c := &Config{ProxyImage: "default/envoy:test"}
		overrideProxyImage(c)
		if c.ProxyImage != "default/envoy:test" {
			t.Errorf("expected default/envoy:test, got %q", c.ProxyImage)
		}
	})

	t.Run("PROXY_IMAGE overrides the proxy image", func(t *testing.T) {
		t.Setenv("PROXY_IMAGE", "custom/envoy:test")
		c := &Config{ProxyImage: "default/envoy:test"}
		overrideProxyImage(c)
		if c.ProxyImage != "custom/envoy:test" {
			t.Errorf("expected custom/envoy:test, got %q", c.ProxyImage)
		}
	})
}
