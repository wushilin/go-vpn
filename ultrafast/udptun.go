package main

// Demo program of a Simple VPN using UDP, no encryption, no security
// It performs almost at the exactly same speed as the QUIC one.
// Just for your info
import (
	"flag"
	"log"
	"net"
	"os"
	"sync"

	"github.com/songgao/water"
	"github.com/wushilin/go-vpn/common"
)

var connect string = ""
var listen string = ""
var server_mode bool = false
var device_name string = ""
var laddr string = ""

func validate_params() {
	if connect == "" && listen != "" {
		server_mode = true
	} else if connect != "" && listen == "" {
		server_mode = false
	} else {
		log.Fatalf("You have to specify a mode!")
	}
}
func main() {
	flag.StringVar(&connect, "connect", "", "Peer UDP host and port in xx.xx.xx.xx:4432 format")
	flag.StringVar(&listen, "listen", "", "Listen UDP address in xx.xx.xx.xx:4432 format")
	flag.StringVar(&device_name, "tunname", "TUN17", "Device name")
	flag.StringVar(&laddr, "laddr", "", "Local interface address")
	flag.Parse()
	validate_params()

	config := water.Config{
		DeviceType: water.TUN,
	}
	config.Name = device_name
	var err error
	iface, err := water.New(config)
	if err != nil {
		log.Fatal(err)
	}
	if !common.BringUpLink(device_name) {
		log.Fatalf("Failed to bring link UP\n")
	}
	if !common.SetIPAddress(device_name, laddr) {
		log.Fatalf("Failed to set IP Address to %s\n", laddr)
	}

	if server_mode {
		run_server(iface)
	} else {
		run_client(iface)
	}
}

func run_server(iface *water.Interface) {
	udpServer, err := net.ListenPacket("udp", listen)
	if err != nil {
		log.Fatal(err)
	}
	defer udpServer.Close()

	wg := new(sync.WaitGroup)
	wg.Add(1)
	var addr *net.UDPAddr
	go func() {
		defer wg.Done()
		var err error
		buf := make([]byte, 4096)
		nread := 0
		for {
			var addr1 net.Addr
			nread, addr1, err = udpServer.ReadFrom(buf)
			if addr == nil {
				addr, _ = addr1.(*net.UDPAddr)
			}
			if err != nil {
				continue
			}
			iface.Write(buf[:nread])
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		var err error
		nread := 0
		for {
			nread, err = iface.Read(buf)
			if err != nil {
				continue
			}
			udpServer.WriteTo(buf[:nread], addr)
		}
	}()

	wg.Wait()
}

func run_client(iface *water.Interface) {
	udpServer, err := net.ResolveUDPAddr("udp", connect)

	if err != nil {
		println("ResolveUDPAddr failed:", err.Error())
	}

	conn, err := net.DialUDP("udp", nil, udpServer)
	if err != nil {
		println("Listen failed:", err.Error())
		os.Exit(1)
	}

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		buf := make([]byte, 4096)
		nread := 0
		for {
			nread, _, err = conn.ReadFrom(buf)
			if err != nil {
				continue
			}
			iface.Write(buf[:nread])
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		var err error
		nread := 0
		for {
			nread, err = iface.Read(buf)
			if err != nil {
				continue
			}
			conn.Write(buf[:nread])
		}
	}()

	wg.Wait()
	// close the connection
	defer conn.Close()
}
