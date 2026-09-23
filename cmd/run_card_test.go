package main

import (
	"testing"

	"github.com/orbitproxy/orbitproxy-go/service"
)

func TestEndpointForwards(t *testing.T) {
	got := endpointForwards([]service.EndpointStatus{
		{
			Category: "mcp", EndpointID: "iep_1", Protocol: "http", LocalAddr: "127.0.0.1:3000",
			Proxies: []service.EndpointProxy{
				{ProxyID: "proxy-a", PubHost: "mcp.example.com", PubPort: 443},
				{ProxyID: "proxy-b", PubHost: "composer.example.com", PubPort: 8443},
			},
		},
		{Category: "basic", EndpointID: "mep_1", Protocol: "tcp", LocalAddr: "127.0.0.1:22",
			Proxies: []service.EndpointProxy{{ProxyID: "proxy-c", PubHost: "tcp.example.com", PubPort: 20022}}},
		{Protocol: "webhook", Delivery: service.DeliveryInProcess},
	})
	if len(got) != 3 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Kind != "mcp" || got[0].EndpointID != "iep_1" || got[0].Local != "127.0.0.1:3000" {
		t.Fatalf("row0=%+v", got[0])
	}
	if len(got[0].Hosts) != 2 || got[0].Hosts[0] != "mcp.example.com" || got[0].Hosts[1] != "composer.example.com:8443" {
		t.Fatalf("hosts=%v", got[0].Hosts)
	}
	if len(got[1].Hosts) != 1 || got[1].Hosts[0] != "tcp.example.com:20022" {
		t.Fatalf("tcp hosts=%v", got[1].Hosts)
	}
	if got[2].Kind != "webhook" || got[2].Local != "in-process" {
		t.Fatalf("row2=%+v", got[2])
	}
}
