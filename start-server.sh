#!/bin/sh


# key must match for server & client.
# mode can be tcp or quic
./daemon -l -b 0.0.0.0:4792 -laddr 172.47.88.4/24 -mode tcp -key mysupersecretkeythatworks
