/*
	Copyright 2019 Stellar Project

	Permission is hereby granted, free of charge, to any person obtaining a copy of
	this software and associated documentation files (the "Software"), to deal in the
	Software without restriction, including without limitation the rights to use, copy,
	modify, merge, publish, distribute, sublicense, and/or sell copies of the Software,
	and to permit persons to whom the Software is furnished to do so, subject to the
	following conditions:

	The above copyright notice and this permission notice shall be included in all copies
	or substantial portions of the Software.

	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED,
	INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR
	PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE
	FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
	TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE
	USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

package heimdall

import "time"

// Config is the configuration used for the stellar server
type Config struct {
	// ID is the id of the node
	ID string
	// GRPCAddress is the address for the grpc server
	GRPCAddress string
	// AdvertiseGRPCAddress is the public address for the grpc server
	AdvertiseGRPCAddress string
	// GRPCPeerAddress is the peer address to join
	GRPCPeerAddress string
	// ClusterKey is a preshared key for cluster peers
	ClusterKey string
	// NodeNetwork is the network for the cluster nodes
	NodeNetwork string
	// PeerNetwork is the subnet that is used for cluster peers
	PeerNetwork string
	// EndpointIP is the IP used for peer communication
	EndpointIP string
	// GatewayPort is the port used for peer communication
	EndpointPort int
	// InterfaceName is the interface used for peer communication
	InterfaceName string
	// RedisURL is the uri to the redis backend
	RedisURL string
	// AdvertiseRedisURL is the uri to the public redis backend
	AdvertiseRedisURL string
	// TLSCertificate is the certificate used for grpc communication
	TLSServerCertificate string
	// TLSKey is the key used for grpc communication
	TLSServerKey string
	// TLSClientCertificate is the client certificate used for communication
	TLSClientCertificate string
	// TLSClientKey is the client key used for communication
	TLSClientKey string
	// TLSInsecureSkipVerify disables certificate verification
	TLSInsecureSkipVerify bool
}

// PeerConfig is the configuration of the peer
type PeerConfig struct {
	// ID is the id of the peer
	ID string
	// Address is the GRPC address of the peer to join
	Address string
	// UpdateInterval is the interval in which to update with the cluster
	UpdateInterval time.Duration
	// InterfaceName is the interface used for peer communication
	InterfaceName string
}
