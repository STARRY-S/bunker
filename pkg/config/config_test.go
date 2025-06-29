package config

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func Test_Config(t *testing.T) {
	c := &Config{
		BindAddr: "127.0.0.1",
		Port:     8080,
		CertFile: "certs/cert.pem",
		KeyFile:  "certs/cert.key",

		InsecureSkipTLSVerify: false,

		Routes: []Route{
			{
				// Check server alive
				RouteType: TypePlainText,
				Name:      "keep alive checker",
				Path:      "/ping",
				PlainText: &PlainText{
					Content: "/pong\n",
					Status:  200,
				},
			},
			{
				// Default response 403 to all requests
				RouteType: TypePlainText,
				Prefix:    "/",
				Name:      "default block all access",
				PlainText: &PlainText{
					Content: "forbidden\n",
					Status:  403,
				},
			},
			{
				RouteType: TypeDefault,
				Name:      "route web1",
				RemoteURL: "https://web1.example.com",
				LocalURL:  "https://127.0.0.1:8081",
			},
			{
				RouteType: TypeDefault,
				Name:      "route web2",
				RemoteURL: "https://web2.example.com",
				LocalURL:  "https://127.0.0.1:8082",
			},
			{
				RouteType: TypeDefault,
				Name:      "route web3",
				RemoteURL: "https://web3.example.com",
				LocalURL:  "https://127.0.0.1:8083",
			},
		},
	}

	b, err := yaml.Marshal(c)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	f, err := os.Create("config.yaml")
	if err != nil {
		t.Fatalf("failed to create tmp.yaml: %v", err)
	}
	defer f.Close()
	f.Write(b)
}
