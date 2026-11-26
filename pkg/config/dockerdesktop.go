package config

import "os"

func init() {
	overrideProxyImage(DefaultConfig)
}

// overrideProxyImage lets Docker Desktop pass the full proxy image reference with PROXY_IMAGE.
func overrideProxyImage(c *Config) {
	if image := os.Getenv("PROXY_IMAGE"); image != "" {
		c.ProxyImage = image
	}
}
