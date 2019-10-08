# Heimdall
Heimdall is a peer-to-peer private network system.  It uses [Wireguard](https://www.wireguard.com) for peer
communication and [Redis](https://redis.io) for backend storage.  All nodes are equal and the system
auto-elects in the event of a current master failure.  Non-node peers are also supported in which they
can only access the network but do not provide routing capabilities.

![Heimdall](docs/overview.png)

# Concepts
The following is the core design of Heimdall.

## Backend
[Redis](https://redis.io) is used to store network state.  It operates in a master/replica design for
fault tolerance and high availability.

## Security
Security and privacy are a core component of the network.  UUIDs are generated for nodes and peers
and both can use optional TLS to connect to the network.  All communication happens over Wireguard or minimal
management over the GRPC API.

## Node
A Node is a machine in the network that operates as a gateway.  Nodes get a /16 by default to provide
network access to services.  Since all nodes are created equal, a pre-shared cluster key is used for access
when joining.  Upon joining, the node's Redis store is configured as a replica of the current master.
If the master goes away, an "oldest sibling" master election is performed.  The new master node's Redis
store is re-configured as the master and all other peer nodes are re-configured as replicas.

## Peer
There is also the ability for non-node peers to join.  These peers can access all services provided by the
gateway nodes but cannot provide routing or access themselves.  They are access only peers.  In order for
a peer to join, their peer ID must be authorized by an existing node.

## Routes
In the event that the node's /16 network space is not enough or wants to provide access to another subnet,
custom routes can be published.  This is done by publishing the route via the desired node ID.  All nodes
and peers will sync and re-configure their route tables accordingly.

# Setup
The following is a quick start to get Heimdall running.  This has only been tested on Alpine linux but
should be similar with other distros.

## Dependencies
The following need to be installed for Heimdall to operate properly:

Note: the `edge` repositories must be enabled to get the Wireguard packages.  Uncomment them from `/etc/apk/repositories`.

- `wireguard-vanilla` or `wireguard-virt` if using a VM
- `wireguard-tools`
- `iptables`
- `ip6tables`
- `redis` (optional)

This can be peformed with the following:

```bash
$> apk add -U wireguard-vanilla wireguard-tools iptables ip6tables
```

Note: make sure to reboot after installation so the kernel modules can be properly loaded.

Enable IP forwarding:

```bash
$> cat << EOF > /etc/sysctl.conf
net.ipv4.ip_forward = 1
EOF
```

# Redis
It is recommended to run a local Redis datastore on the node as all data is replicated.
It must be configured to bind (`bind 0.0.0.0`) on all interfaces so it can be accessed over the Wireguard
tunnel once Heimdall creates it.  Due to [Protected Mode](https://redis.io/topics/security)
in Redis it is recommended to use a password via the `requirepass` directive.

If using Alpine, you will also want to ensure Redis starts on boot:

```bash
$> rc-update add redis default
$> service redis start
```

# Heimdall
Use the following to get the latest version of Heimdall.

## Build
Assuming you have a [Go](https://golang.org) toolchain configured you should be able to run `make` to get binaries.

## Releases
See the releases page for the latest pre-built binaries.

# Running
To start a network use the following.  In this example, we will be using a three node network:

On the first node, run Heimdall:

```bash
$> heimdall -D -r redis://root:<password>@127.0.0.1:6379 --cluster-key 05e73f5c-2848-434c-a0d8-ce4bb78bd4d3
DEBU[0000] starting grpc server                          addr="tcp://192.168.122.198:9000"
INFO[0000] starting heimdall                             commit=f845269a.m version=0.1.0
INFO[0000] starting as master id=09ed6b23-c608-5863-9b4b-f6d8b75c85ef
DEBU[0000] starting master heartbeat
DEBU[0000] disabling replica status
DEBU[0000] starting master heartbeat: ttl=5s
INFO[0000] cluster master key=05e73f5c-2848-434c-a0d8-ce4bb78bd4d3
WARN[0000] node does not have an IP assigned yet
DEBU[0000] generating new keypair for 09ed6b23-c608-5863-9b4b-f6d8b75c85ef
DEBU[0000] allocated network 10.11.0.0/16 for 09ed6b23-c608-5863-9b4b-f6d8b75c85ef
DEBU[0000] updating node network: id=09ed6b23-c608-5863-9b4b-f6d8b75c85ef subnet=10.11.0.0/16
DEBU[0000] peer info updated: id=09ed6b23-c608-5863-9b4b-f6d8b75c85ef
DEBU[0000] updating peer config to version 6cf910f3d2aeb5424f6568092014d2a826a269230dad729db93834b03f2df4ca
DEBU[0000] starting node heartbeat: ttl=15s
INFO[0000] restarting tunnel darknet
DEBU[0000] starting peer config updater: ttl=10s
```

Note: if no cluster-key is specified one will be generated.  This will need to be specified on future invocations as it is random.

On the first node, it allocated `10.11.0.0/16` as its network.  It will have the IP `10.11.0.1/16` assigned.

On the second node, run Heimdall specifying the first node as the peer:

```bash
$> heimdall -D --cluster-key 05e73f5c-2848-434c-a0d8-ce4bb78bd4d3 -r redis://root:<password>@127.0.0.1:6379 --peer tcp://192.168.122.198:9000
DEBU[0000] starting grpc server                          addr="tcp://192.168.122.143:9000"
INFO[0000] starting heimdall                             commit=f845269a.m version=0.1.0
DEBU[0000] joining tcp://192.168.122.198:9000
DEBU[0000] master info received: id=09ed6b23-c608-5863-9b4b-f6d8b75c85ef grpc=tcp://192.168.122.198:9000
DEBU[0000] updating peer config to version 1577951667592b0851e6fbfa53a5c8076aeda1266f9855df06d1da081c34a8b7
INFO[0000] restarting tunnel darknet
INFO[0000] waiting for master 09ed6b23-c608-5863-9b4b-f6d8b75c85ef
DEBU[0005] rtt master ping: 5.232848429s
DEBU[0005] joining master: 09ed6b23-c608-5863-9b4b-f6d8b75c85ef
INFO[0005] configuring node as replica of 09ed6b23-c608-5863-9b4b-f6d8b75c85ef
DEBU[0005] configuring redis as slave of 09ed6b23-c608-5863-9b4b-f6d8b75c85ef
INFO[0005] waiting for redis sync with 09ed6b23-c608-5863-9b4b-f6d8b75c85ef
DEBU[0006] starting replica monitor: ttl=5s
DEBU[0006] node network for f20c02a1-6fd1-5a8d-9df1-caeccdd987c5: 10.12.0.0/16
DEBU[0006] starting node heartbeat: ttl=15s
DEBU[0006] starting peer config updater: ttl=10s
```

On the second node, it allocated `10.12.0.0/16` as its network.  It will have the IP `10.12.0.1/16` assigned.

On the third node, run Heimdall specifying either one of the other nodes as the peer (any node can be joined as it forwards the request to the master):

```bash
$> heimdall -D --cluster-key 05e73f5c-2848-434c-a0d8-ce4bb78bd4d3 --peer tcp://192.168.122.143:9000 -r redis://root:darknet@127.0.0.1:6379
DEBU[0000] starting grpc server                          addr="tcp://192.168.122.109:9000"
INFO[0000] starting heimdall                             commit=f845269a.m version=0.1.0
DEBU[0000] joining tcp://192.168.122.143:9000
DEBU[0000] master info received: id=09ed6b23-c608-5863-9b4b-f6d8b75c85ef grpc=tcp://192.168.122.198:9000
DEBU[0000] updating peer config to version 477833e21934c0f4dfb3cb8a2786d61444038a057e18e9c041c44fbe47f2d7a0
INFO[0000] restarting tunnel darknet
INFO[0000] waiting for master 09ed6b23-c608-5863-9b4b-f6d8b75c85ef
DEBU[0010] rtt master ping: 10.369636565s
DEBU[0010] joining master: 09ed6b23-c608-5863-9b4b-f6d8b75c85ef
INFO[0010] configuring node as replica of 09ed6b23-c608-5863-9b4b-f6d8b75c85ef
DEBU[0010] configuring redis as slave of 09ed6b23-c608-5863-9b4b-f6d8b75c85ef
INFO[0010] waiting for redis sync with 09ed6b23-c608-5863-9b4b-f6d8b75c85ef
DEBU[0011] starting replica monitor: ttl=5s
DEBU[0011] node network for 9e915536-830e-555b-b74c-d4495c0c3467: 10.13.0.0/16
DEBU[0011] starting node heartbeat: ttl=15s
DEBU[0011] starting peer config updater: ttl=10s
```

On the third node, it allocated `10.13.0.0/16` as its network.  It will have the IP `10.13.0.1/16` assigned.

You can view the status of the Wireguard tunnel (the peers will vary depending on which node you check the status) as well as verify connectivity:

```bash
$> wg
interface: darknet
  public key: yOA4f4iyxFg6spDUGy6uS3M12d5/vkQMfdLgCK27mwI=
  private key: (hidden)
  listening port: 10100

peer: Lbcue7KFOqDE2rN1Or2f5hSxE7+ruJJjzOOSpk2m23s=
  endpoint: 192.168.122.143:10100
  allowed ips: 10.12.0.0/16
  latest handshake: 1 minute, 56 seconds ago
  transfer: 36.70 KiB received, 43.65 KiB sent

peer: /2YDhxBQQQbP5D+Yu0v07QvBwEnOBNZ6aFtwikhn4C4=
  endpoint: 192.168.122.109:10100
  allowed ips: 10.13.0.0/16
  latest handshake: 1 minute, 56 seconds ago
  transfer: 39.08 KiB received, 47.66 KiB sent
```

From the first node you should be able to ping the other nodes:

```bash
$> ip a s darknet
55: darknet: <POINTOPOINT,NOARP,UP,LOWER_UP> mtu 1420 qdisc noqueue state UNKNOWN group default qlen 1000
    link/none
    inet 10.11.0.1/16 scope global darknet
       valid_lft forever preferred_lft forever

$> ping 10.12.0.1
PING 10.12.0.1 (10.12.0.1): 56 data bytes
64 bytes from 10.12.0.1: seq=0 ttl=64 time=1.643 ms
64 bytes from 10.12.0.1: seq=1 ttl=64 time=1.757 ms
64 bytes from 10.12.0.1: seq=2 ttl=64 time=0.429 ms

$> ping 10.13.0.1
PING 10.13.0.1 (10.13.0.1): 56 data bytes
64 bytes from 10.13.0.1: seq=0 ttl=64 time=1.743 ms
64 bytes from 10.13.0.1: seq=1 ttl=64 time=1.705 ms
64 bytes from 10.13.0.1: seq=2 ttl=64 time=0.992 ms
```

# Peers
To start a non-node peer using the following:

First, get the ID of the peer:

```bash
$> hpeer -v
peer version=0.1.0-dev (f845269a.m) linux/amd64 id=9684c4f6-5bf1-502f-953b-2abc943911c4
```

Next, authorize the ID in the cluster:

```bash
$> hctl -a tcp://192.168.122.198:9000 peers authorize 9e915536-830e-555b-b74c-d4495c0c3467
```

Verify the ID was authorized:

```bash
$> hctl -a tcp://192.168.122.198:9000 peers authorized
ID
9e915536-830e-555b-b74c-d4495c0c3467
```

Now connect the peer:

```bash
$> hpeer -D --addr tcp://192.168.122.198:9000
INFO[0000] starting heimdall                             commit=f845269a.m version=0.1.0
INFO[0000] connecting to peer tcp://192.168.122.198:9000
DEBU[0000] updating peer config to version 5055b697213dd024f5f001169a1c1f5558739603299ceaa0ff1e17ced15ee1e9
INFO[0000] restarting tunnel darknet
```

The peer will be allocated an IP on the peer network (default: `10.51.0.0/16`).
You can now verify connectivity:

```bash
$> ip a s darknet
5: darknet: <POINTOPOINT,NOARP,UP,LOWER_UP> mtu 1420 qdisc noqueue state UNKNOWN group default qlen 1000
    link/none
    inet 10.51.0.2/32 scope global darknet
       valid_lft forever preferred_lft forever

$> ping 10.11.0.1
PING 10.11.0.1 (10.11.0.1): 56 data bytes
64 bytes from 10.11.0.1: seq=0 ttl=64 time=4.157 ms
64 bytes from 10.11.0.1: seq=1 ttl=64 time=1.582 ms
64 bytes from 10.11.0.1: seq=2 ttl=64 time=1.177 ms
```
