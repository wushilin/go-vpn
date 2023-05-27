package transport

import (
	"log"
	"net"
)

type UDPTransport struct {
	Conn          *net.UDPConn
	TargetAddress *net.UDPAddr
	IsServer      bool
	Remaining     []byte
	Length        int
	Offset        int
}

func (v *UDPTransport) Read(buffer []byte) (int, error) {
	if v.Offset < v.Length {
		//log.Printf("Reading remaining bytes.\n")
		copied := copy(buffer, v.Remaining[v.Offset:v.Length])
		v.Offset += copied
		//log.Printf("Read %d bytes from buffer\n", copied)
		return copied, nil
	} else {
		v.Length = 0
		v.Offset = 0
	}
	//log.Printf("Attempting to read from %v\n", v.Conn.LocalAddr())
	var nread int
	var addr *net.UDPAddr
	var err error
	for {
		nread, addr, err = v.Conn.ReadFromUDP(v.Remaining)
		if err != nil {
			log.Fatal("Failed to read from UDP", err)
		}
		//log.Printf("Received %d bytes from %v\n", nread, addr)
		if !v.IsServer {
			v.Length = nread
			return v.Read(buffer)
		}
		// server
		if v.TargetAddress == nil {
			//log.Printf("Marking target address as %s\n", addr)
			v.TargetAddress = addr
			v.Length = nread
			return v.Read(buffer)
		} else {
			if v.TargetAddress.IP.Equal(addr.IP) && v.TargetAddress.Port == addr.Port {
				v.Length = nread
				return v.Read(buffer)
			} else {
				//log.Printf("%v != %v, ignored\n", v.TargetAddress, addr)
			}
		}
	}
}

func (v *UDPTransport) Write(buffer []byte) (int, error) {
	//log.Printf("Writing %v to %v\n", buffer, v.TargetAddress)
	var written int
	var err error
	if !v.IsServer {
		written, err = v.Conn.Write(buffer)
	} else {
		written, err = v.Conn.WriteToUDP(buffer, v.TargetAddress)
	}

	//log.Printf("Write result %d %v\n", written, err)
	return written, err
}

func (v *UDPTransport) Close() error {
	v.TargetAddress = nil
	return v.Conn.Close()
}

func NewUDPServerTransport(bind_addr string) Transport {
	udpServer, err := net.ResolveUDPAddr("udp4", bind_addr)
	if err != nil {
		log.Fatal(err)
	}
	conn, err := net.ListenUDP("udp4", udpServer)
	if err != nil {
		log.Fatal(err)
	}
	return &UDPTransport{Conn: conn, TargetAddress: nil, IsServer: true, Remaining: make([]byte, 4096), Length: 0, Offset: 0}
}

func NewUDPClientTransport(server_addr string) Transport {
	udpServer, err := net.ResolveUDPAddr("udp4", server_addr)
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.DialUDP("udp4", nil, udpServer)
	if err != nil {
		log.Fatal(err)
	}

	return &UDPTransport{Conn: conn, TargetAddress: udpServer, IsServer: false, Remaining: make([]byte, 4096), Length: 0, Offset: 0}
}
