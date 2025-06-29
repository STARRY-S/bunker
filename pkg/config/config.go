package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type RouteType string

const (
	TypeDefault    RouteType = "default"
	TypePlainText  RouteType = "plainText"
	TypeStaticFile RouteType = "staticFile"
)

type Config struct {
	// BindAddr is the address to bind, default 127.0.0.1
	BindAddr string `json:"bindAddr" yaml:"bindAddr"`
	// Port is the port of the proxy server
	Port int `json:"listen" yaml:"listen"`

	// TLS Certificate keypair
	CertFile string `json:"certFile" yaml:"certFile"`
	KeyFile  string `json:"keyFile" yaml:"keyFile"`

	// InsecureSkipTLSVerify, if true, will skip TLS verification for the proxied requests
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify" yaml:"insecureSkipTLSVerify"`

	// Routes is the list of other custom routes to be proxied
	Routes []Route `json:"routes,omitempty" yaml:"routes,omitempty"`
}

type Route struct {
	// RouteType is the type of this custom route configuration
	// Available: default/plainText/StaticFile
	RouteType RouteType `json:"type" yaml:"type"`

	// Name is the name of the route
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	RemoteURL string `json:"remoteURL" yaml:"remoteURL"`
	LocalURL  string `json:"localURL" yaml:"localURL"`

	// Path matches the exact URL path to be proxied
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	// Prefix matches the URL prefix to be proxied
	Prefix string `json:"prefix,omitempty" yaml:"prefix,omitempty"`

	// PlainText responses the txt data if route type is plainText
	PlainText *PlainText `json:"plainText,omitempty" yaml:"plainText,omitempty"`

	// Static file responses the file content if route type is staticFile
	StaticFile string `json:"staticFile,omitempty" yaml:"staticFile,omitempty"`
}

type PlainText struct {
	// Content is the plaintext content to be response
	Content string `json:"content,omitempty" yaml:"content,omitempty"`

	// Status is the status code of the plaintext response
	Status int `json:"status,omitempty" yaml:"status,omitempty"`
}

func NewConfigFromFile(name string) (*Config, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", name, err)
	}
	c := &Config{}
	err = yaml.Unmarshal(b, c)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config YAML %q: %w", name, err)
	}
	return c, nil
}
