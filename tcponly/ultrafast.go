package main

// Demo program of a Simple VPN using UDP, no encryption, no security
// It performs almost at the exactly same speed as the QUIC one.
// Just for your info
import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/songgao/water"
	"github.com/wushilin/go-vpn/common"
	"github.com/wushilin/go-vpn/encryption"
)

type AddressHolder struct {
	Address *net.UDPAddr
}
type DATA_TYPE byte

const DATA DATA_TYPE = 0
const PING DATA_TYPE = 1

const SERVER_IP_DEFAULT = "10.99.99.1/30"
const CLIENT_IP_DEFAULT = "10.99.99.2/30"
const LISTEN_DEFAULT = "0.0.0.0:20192"

var connect string = ""
var listen string = ""
var server_mode bool = false
var device_name string = ""
var key string = ""
var laddr string = ""
var aes encryption.Coder = nil

func is_data(data []byte) bool {
	return data[0] == byte(DATA)
}

func is_ping(data []byte) bool {
	if data[0] != byte(PING) {
		return false
	}
	buffer := make([]byte, 100)
	return string(aes.Decrypt(data[1:], buffer)) == "PING"
}

func validate_params() {
	if connect == "" && listen != "" {
		server_mode = true
	} else if connect != "" && listen == LISTEN_DEFAULT {
		server_mode = false
		listen = ""
	} else if connect == "" && listen == "" {
		log.Fatalf("You have to use -listen or -connect")
	} else {
		log.Fatalf("You can't specify -listen and -connect at the same time!")
	}
}
func main() {
	flag.StringVar(&connect, "connect", "", "Peer UDP host and port in [ip|hostname]:port format. Default is \"\"")
	flag.StringVar(&listen, "listen", LISTEN_DEFAULT, "UDP Listen address in ip:port format. Default is 0.0.0.0:20192")
	flag.StringVar(&device_name, "tunname", "TUN17", "Device name")
	flag.StringVar(&key, "aeskey", "", "AES 256 encryption key. Will be padded with ' ' or trimmed if not 32 chars")
	flag.StringVar(&laddr, "laddr", "", "Local interface address. Default 10.99.99.1/30 for server, 10.99.99.2/30 for client")
	flag.Parse()

	validate_params()

	for len(key) < 32 {
		key = key + " "
	}

	key = key[:32]
	var err error
	aes, err = encryption.NewAESCoder([]byte(key))
	if err != nil {
		log.Fatal(err)
	}
	if server_mode {
		log.Printf("SERVER MODE. Listens on %s", listen)
	} else {
		log.Printf("CLIENT MODE. Connects to %s", connect)
	}
	if laddr == "" {
		if server_mode {
			laddr = SERVER_IP_DEFAULT
		} else {
			laddr = CLIENT_IP_DEFAULT
		}
	}
	config := water.Config{
		DeviceType: water.TUN,
	}
	config.Name = device_name
	iface, err := water.New(config)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Tunnel device %s had been created.", device_name)
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

func gen_ping(buf []byte) int {
	buf[0] = byte(PING)
	payload := []byte("PING")
	encrypted := aes.Encrypt(payload, buf[1:])
	return 1 + len(encrypted)
}

func run_ping(ping_run *bool, addr_holder *AddressHolder, conn *net.UDPConn) {
	go func() {
		buffer := make([]byte, 4096)
		var nwritten = 0
		var err error
		log.Printf("PING started")
		sent := false
		for *ping_run {
			to_write := buffer[:gen_ping(buffer)]
			if addr_holder == nil {
				nwritten, err = conn.Write(to_write)
				sent = true
			} else if addr_holder.Address != nil {
				nwritten, err = conn.WriteTo(to_write, addr_holder.Address)
				sent = true
			} else {
				sent = false
			}
			if sent {
				if err != nil {
					log.Printf("Ping send failed: %s(%d)", err, nwritten)
				}
			}
			time.Sleep(3 * time.Second)
		}
		log.Printf("PING stopped")
	}()
}

func run_server(iface *water.Interface) {
	// Create a UDP address to listen on
	udpAddr, err := net.ResolveUDPAddr("udp", listen)
	if err != nil {
		fmt.Println("Error resolving UDP address:", err)
		return
	}

	// Create a UDP listener
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Println("Error creating UDP listener:", err)
		return
	}
	defer udpConn.Close()

	fmt.Println("UDP listener started on", udpConn.LocalAddr())

	wg := new(sync.WaitGroup)
	wg.Add(1)
	var addr_holder = &AddressHolder{}
	ping_run := true
	run_ping(&ping_run, addr_holder, udpConn)

	go func() {
		defer wg.Done()
		var err error
		buf := make([]byte, 4096)
		nread := 0
		nwritten := 0
		for {
			var addr1 *net.UDPAddr
			nread, addr1, err = udpConn.ReadFromUDP(buf)
			if err != nil {
				log.Printf("UDP read failed: %s(%d)", err, nread)
				continue
			}
			if is_ping(buf[:nread]) {
				old_addr := addr_holder.Address
				addr_holder.Address = addr1
				if old_addr == nil {
					log.Printf("Link up by first successful PING. Peer: nil -> %s", addr1)
				} else if !old_addr.IP.Equal(addr1.IP) || old_addr.Port != addr1.Port {
					log.Printf("Link reset by successful PING. Peer: %s -> %s", old_addr, addr1)
				}
				continue
			}
			if addr_holder.Address != nil && (!addr_holder.Address.IP.Equal(addr1.IP) || addr_holder.Address.Port != addr1.Port) {
				//log.Printf("Ignored packet not from original sender!")
				continue
			}
			if is_data(buf) {
				nwritten, err = iface.Write(buf[1:nread])
				if err != nil {
					log.Printf("Iface write error: %s(%d)", err, nwritten)
				}
				if nwritten != nread-1 {
					log.Printf("Incomplete write written %d != read %d", nwritten, nread-1)
				}
			} else {
				log.Printf("Ignored unknown data of type %d", buf[0])
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		var err error
		nread := 0
		nwritten := 0
		for {
			nread, err = iface.Read(buf[1:])
			if err != nil {
				log.Printf("Iface read error: %s (%d)", err, nread)
				continue
			}
			if addr_holder.Address == nil {
				log.Printf("Discarding packet because link is not up yet (no target)")
				// first byte not in yet. ignoring
				continue
			}
			buf[0] = byte(DATA)
			nwritten, err = udpConn.WriteTo(buf[:nread+1], addr_holder.Address)
			if err != nil {
				log.Printf("UDP write error: %s", err)
			}
			if nwritten != nread+1 {
				log.Printf("UDP incomplete write written %d != read %d", nwritten, nread+1)
			}
		}
	}()

	wg.Wait()
	ping_run = false
}

func run_client(iface *water.Interface) {
	udpServer, err := net.ResolveUDPAddr("udp", connect)

	if err != nil {
		println("ResolveUDPAddr failed:", err.Error())
	}

	conn, err := net.DialUDP("udp", nil, udpServer)
	if err != nil {
		println("Connect failed:", err.Error())
		os.Exit(1)
	}

	ping_run := true
	run_ping(&ping_run, nil, conn)

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		buf := make([]byte, 4096)
		nread := 0
		nwritten := 0
		var addr net.Addr
		var link_up bool = false
		for {
			nread, addr, err = conn.ReadFromUDP(buf)
			if err != nil {
				log.Printf("UDP read error: %s(%d)", err, nread)
				continue
			} else {
				if !link_up {
					log.Printf("Link up by first packet with %s", addr)
					link_up = true
				}
			}
			if is_ping(buf[:nread]) {
				continue
			}
			if is_data(buf) {
				nwritten, err = iface.Write(buf[1:nread])
				if err != nil {
					log.Printf("Iface write error: %s(%d)", err, nwritten)
				}
				if nwritten != nread-1 {
					log.Printf("Ifae incomplete write written %d != read %d", nwritten, nread-1)
				}
			} else {
				//log.Printf("Ignored unknown data of type %d", buf[0])
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		var err error
		nread := 0
		nwritten := 0
		for {
			nread, err = iface.Read(buf[1:])
			if err != nil {
				log.Printf("Iface read error %s(%d)", err, nread)
				continue
			}
			buf[0] = byte(DATA)
			nwritten, err = conn.Write(buf[:nread+1])
			if err != nil {
				log.Printf("UDP write error %s(%d)", err, nwritten)
			}
			if nwritten != nread+1 {
				log.Printf("UDP incomplete write written %d != read %d", nwritten, nread+1)
			}
		}
	}()

	wg.Wait()
	// close the connection
	defer conn.Close()
}
