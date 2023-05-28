# go-vpn
A secure, single executable (same for client/server) point to point VPN server that uses QUIC protocol. It also supports routing propagation by options


# How it works
This program has server mode and client mode.

Both server and client must run as root, as we need to manipulate the system tunnel device.

By default tunnel device used is TUN17, you can specify the name by `-tunname tun12` to switch to `tun12` instead.

Server will start binding to QUIC (UDP) port and wait for client to connect.

Client connects via QUIC with mutual. 
* Server always load `server.pem`, `server.key`, `ca.pem` for TLS configuration.
* Client always load `client.pem`, `client.key`, `ca.pem` for TLS configuration.

Server and client can specify the cert's common name by `-commonName` to validate. 
If peer certificate's commonName is not matching, connection will be disconnected.

Client and server both can propogate the additional route rules to request remote host to route the IP ranges to local.

If route setup fails, the connection will be reset and all routing rule that had been requested will be deleted.

If more than 1 subnet is required, separate by `;`. For example `-route "192.168.44.0/24;10.251.116.0/24"`

## Server

```bash
# ./go-vpn -l -b 0.0.0.0:4792 -laddr 172.47.88.1/24 -route "192.168.44.0/24;10.251.116.0/24" -commonName client003
```

Explantion:
* Listen on UDP: `0.0.0.0:4792`
* Assign Local Tun Device IP of `172.47.88.1/24`
* Request remote to route `192.168.44.0/24;10.251.116.0/24` in addition to `172.47.88.1/32` (the local IP)
* Require client certificate is signed by `ca.pem` and has commonName of `client003`

## Client

```bash
# ./go-vpn -s 192.168.44.105:4792 -laddr 192.168.115.211/24 -route "172.19.0.0/16;172.31.0.0/16" -commonName vpnServer
```
Explanation
* Connect to server on UDP: `0.0.0.0:4792`
* Assign Local Tun Device IP of `192.168.115.211/24`
* Request remote to route `172.19.0.0/16;172.31.0.0/16` in addition to `192.168.115.211/32` (the local IP)
* Require client certificate is signed by `ca.pem` and has commonName of `vpnServer`

## Fault tolerance
Connection will be forever retried. It would eventually re-establish connection whenever network disconnect is encountered.

# Built in certificate
The built in certificate is valid for server hostname that looks like `*.local`. If you want to use the default certificate (not recommended), you can add your server's IP address to client's `/etc/hosts` as `something.local`, and connect by that hostname with `-s something.local:4792`.

# Create your own cert
Please consider using https://github.com/wushilin/minica 

Alternatively, a script version is available at https://github.com/wushilin/minica-script
# Enjoy