// Package cloudwego contains small helpers for assembling CloudWeGo options.
package cloudwego

type RegistryConfig struct {
	Type        string            `mapstructure:"type"`
	Endpoints   []string          `mapstructure:"endpoints"`
	Prefix      string            `mapstructure:"prefix"`
	ServiceName string            `mapstructure:"service_name"`
	Addr        string            `mapstructure:"addr"`
	Weight      int               `mapstructure:"weight"`
	Tags        map[string]string `mapstructure:"tags"`
}

type DiscoveryConfig struct {
	Type      string   `mapstructure:"type"`
	Endpoints []string `mapstructure:"endpoints"`
	Prefix    string   `mapstructure:"prefix"`
}
