package main

import (
	"testing"

	"github.com/orbitproxy/orbitproxy-go/service"
)

func TestEndpointForwards(t *testing.T) {
	got := endpointForwards([]service.EndpointStatus{
		{Type: "http", PubHost: "app.example.com", EndpointID: "mep_1", LocalAddr: "127.0.0.1:3000"},
		{Protocol: "webhook", Delivery: service.DeliveryInProcess},
	})
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Kind != "http" || got[0].Domain != "app.example.com" || got[0].EndpointID != "mep_1" || got[0].Local != "127.0.0.1:3000" {
		t.Fatalf("row0=%+v", got[0])
	}
	if got[1].Kind != "webhook" || got[1].Local != "in-process" {
		t.Fatalf("row1=%+v", got[1])
	}
}
