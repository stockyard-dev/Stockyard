// Package network provides peer discovery and secure exchange.
package network

// PeerInfo represents a discovered peer.
type PeerInfo struct {
	Endpoint   string `json:"endpoint"`
	InstanceID string `json:"instance_id"`
	Version    string `json:"version"`
}

// ExchangePayload is the data sent between peers.
type ExchangePayload struct {
	Insights []map[string]any `json:"insights"`
	PeerID   string           `json:"peer_id"`
}
