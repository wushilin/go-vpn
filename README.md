# go-vpn
A secure, single executable (same for client/server) point to point VPN server that uses QUIC protocol. It also supports routing propagation by options


This program only supports **LINUX** at the moment. But it is easy to port this over to other platform. 

Let me know if you need other platform support!

# Before you start
* make sure you have `root` access!
* make sure your `/usr/sbin/ip` is there. It uses the command to manipulate IP address assignment and routes
* Make sure your golang is of `1.20` or newer.

The program may warn about UDP buffer size, it is OK if you don't want to adjust it. It has no significant effect because the IP Frame is typically max at 1500 bytes.

See https://github.com/quic-go/quic-go/wiki/UDP-Receive-Buffer-Size

In short, if you can and keen to adjust it once and for all, use this command:

```bash
# sysctl -w net.core.rmem_max=2500000
```

To make it permenant, edit /etc/sysctl.conf and add
```
net.core.rmem_max=2500000
```

then run
```bash
# sysctl -p
```

# How it works
This program has server mode and client mode.

It has **zero** allocation. It only need to do a one time allocation. No GC is ever required on this service most likely.

Both server and client must run as root, as we need to manipulate the system tunnel device.

By default tunnel device used is `TUN17`, you can specify the name by `-tunname tun12` to switch to `tun12` instead.

Server will start binding to QUIC (UDP) port and wait for client to connect.

Client connects via QUIC with mutual TLS as authentication. This is the security mechanism the VPN is offering. 
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

# Installing
You can install via

```bash
# go install github.com/wushilin/go-vpn@v1.0.0
```

The binary should be in your `$GOPATH/bin`

# Systemd
You can run this as systemd for both server and client.

An example of server:
```
[Unit]
Description=The VPN by Go
After=syslog.target network-online.target remote-fs.target nss-lookup.target
Wants=network-online.target
        
[Service]
Type=simple
#User=service
#Group=service
WorkingDirectory=/opt/vpn
PIDFile=/opt/vpn/vpn.pid
ExecStart=/opt/vpn/go-vpn -l -b 0.0.0.0:24192 -tunname tun99
ExecStop=/bin/kill -s QUIT $MAINPID
PrivateTmp=true
SyslogIdentifier=go-vpn
        
[Install]
WantedBy=multi-user.target
```

# Performance
The service can easily scale beyond 100MiB/s, depending on your network speed and processor speed.

The protocol is secure by default via open standard, TLS may slow down the speed a little bit but I hope you think
it is worth it.


# Built in certificate
The built in certificate is valid for server hostname that looks like `*.local`. If you want to use the default certificate (not recommended), you can add your server's IP address to client's `/etc/hosts` as `something.local`, and connect by that hostname with `-s something.local:4792`.

# Create your own cert
Please consider using https://github.com/wushilin/minica 

Alternatively, a script version is available at https://github.com/wushilin/minica-script
# Enjoy