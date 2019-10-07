# Heimdall
Heimdall is a peer-to-peer private network system.  It uses [Wireguard](https://www.wireguard.com) for peer
communication and [Redis](https://redis.io) for backend storage.  All nodes are equal and the system
auto-elects in the event of a current master failure.  Non-node peers are also supported in which they
can only access the network but do not provide routing capabilities.

![Heimdall](docs/overview.png)

#
