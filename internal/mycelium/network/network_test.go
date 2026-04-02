package network

import "testing"

func TestPeerInfo(t *testing.T) {
	p := PeerInfo{Endpoint: "http://peer:4200", InstanceID: "abc"}
	if p.Endpoint == "" { t.Error("empty endpoint") }
}
