package heimdall

import "time"

// Config is the configuration used for the server
type Config struct {
	// ID is the id of the node
	ID string
	// Name is the name of the node
	Name string
	// RedisPort is the port to use for the managed Redis
	RedisPort int
	// DataDir is the directory for local node configuration
	DataDir string
	// GRPCAddress is the address for the grpc server
	GRPCAddress string
	// AdvertiseGRPCAddress is the public address for the grpc server
	AdvertiseGRPCAddress string
	// GRPCPeerAddress is the peer address to join
	GRPCPeerAddress string
	// DNSServerAddress is the address for the DNS server
	DNSServerAddress string
	// DNSUpstreamAddress is the upstream server for DNS
	DNSUpstreamAddress string
	// AllowPeerToPeer enables peer to peer communication
	AllowPeerToPeer bool
	// ClusterKey is a preshared key for cluster peers
	ClusterKey string
	// NodeNetwork is the network for the cluster nodes
	NodeNetwork string
	// NodeInterface is the ethernet interface for node network
	NodeInterface string
	// PeerNetwork is the subnet that is used for cluster peers
	PeerNetwork string
	// EndpointIP is the IP used for peer communication
	EndpointIP string
	// GatewayPort is the port used for peer communication
	EndpointPort int
	// InterfaceName is the interface used for peer communication
	InterfaceName string
	// AuthorizedPeers are peers to authorize at startup
	AuthorizedPeers []string
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
	// Name is the name of the peer
	Name string
	// Address is the GRPC address of the peer to join
	Address string
	// UpdateInterval is the interval in which to update with the cluster
	UpdateInterval time.Duration
	// InterfaceName is the interface used for peer communication
	InterfaceName string
	// TLSClientCertificate is the client certificate used for communication
	TLSClientCertificate string
	// TLSClientKey is the client key used for communication
	TLSClientKey string
	// TLSInsecureSkipVerify disables certificate verification
	TLSInsecureSkipVerify bool
}
