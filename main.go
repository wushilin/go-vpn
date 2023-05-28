package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wushilin/pool"
	"golang.org/x/net/context"

	"github.com/songgao/water"
	"github.com/wushilin/go-vpn/common"
	"github.com/wushilin/go-vpn/piper"
	"github.com/wushilin/go-vpn/transport"
)

var server_mode bool
var server_address string
var bind_string string
var laddr = ""
var routes = ""
var commonName = ""

func validate_params() {
	if server_mode {
		if bind_string == "" {
			fmt.Printf("ERROR: Server mode requires a bind_string via -b flag")
			os.Exit(1)
		}
		if server_address != "" {
			fmt.Printf("ERROR: Server mode can't accept server_address via -s flag")
			os.Exit(1)
		}
	} else {
		if bind_string != "" {
			fmt.Printf("ERROR: Client mode can't accept bind_string via -b flag")
			os.Exit(1)
		}
		if server_address == "" {
			fmt.Printf("ERROR: Client mode requires a server_address via -s flag")
			os.Exit(1)
		}
	}
}

func main() {
	stop_context, cancel_function := context.WithCancel(context.TODO())
	flag.BoolVar(&server_mode, "l", false, "Run as server mode")
	flag.StringVar(&server_address, "s", "", "Server to Connect To")
	flag.StringVar(&bind_string, "b", "", "Bind address")
	flag.StringVar(&laddr, "laddr", "", "Local address in CIDR notation(10.1.0.10/24)")
	flag.StringVar(&routes, "route", "", "Network to ask remote to route to local in cidr;cidr; format (10.0.0.0/8;192.168.44.7/32;...)")
	flag.StringVar(&commonName, "commonName", "", "Allowed remote certificate common name, default is No Check")
	flag.Parse()
	log.Println("This is the new version")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT, syscall.SIGHUP, syscall.SIGQUIT)
	go func() {
		<-sigs
		fmt.Println("")
		cancel_function()
	}()
	if laddr == "" {
		if server_mode {
			laddr = "10.54.0.10/24"
			log.Printf("Default local addres set to %s\n", laddr)
		} else {
			laddr = "10.54.0.11/24"
			log.Printf("Default local addres set to %s\n", laddr)
		}
	}
	if routes == "" {
		log.Printf("Not requesting additional routing from other party. you can specify -r parameter to request")
	}
	validate_params()
	if server_mode {
		log.Println("Mode: Server, Bind:", bind_string)
	} else {
		log.Println("Mode: Client, Target:", server_address)
	}

	run := true
	for run {
		select {
		case <-stop_context.Done():
			log.Println("Context stopped. Breaking")
			run = false
		default:
		}
		if !run {
			// no need to cleanup, routes and TUN devices will be deleted automatically
			break
		}
		func() {
			var iface *water.Interface
			var trans transport.Transport
			var pipe *piper.Pipe
			defer func() {
				if iface != nil {
					log.Println("Deleting interface TUN17")
					iface.Close()
				}
				if pipe != nil {
					log.Println("Closing pipe")
					pipe.Close()
				}
			}()
			config := water.Config{
				DeviceType: water.TUN,
			}
			config.Name = "TUN17"
			var err error
			iface, err = water.New(config)
			if err != nil {
				log.Fatal(err)
			}
			if !common.BringUpLink() {
				log.Fatalf("Failed to bring link UP\n")
			}
			if !common.SetIPAddress(laddr) {
				log.Fatalf("Failed to set IP Address to %s\n", laddr)
			}
			if server_mode {
				trans, err = setup_server_transport(stop_context, commonName)
			} else {
				trans, err = setup_client_transport(stop_context, commonName)
			}
			if err != nil {
				log.Printf("Setup Transport Error: %s\n", err)
				if !server_mode {
					time.Sleep(3 * time.Second)
				}
				return
			}

			pipe, err = piper.NewPipe(iface, trans, generate_routes(routes, laddr))

			if err != nil {
				log.Fatal(err)
			}
			done := make(chan bool)
			go func() {
				errlocal := pipe.Run(stop_context, server_mode)
				log.Printf("Link Down!")
				if errlocal != nil {
					log.Printf("The service didn't work well... %s", errlocal)
					time.Sleep(3 * time.Second)
				}
				done <- true
			}()
			<-done
			pipe.Close()
			log.Println("Service Loop Ended. Restarting...")
		}()
	}
}

func maker() ([]byte, error) {
	return make([]byte, 4096), nil
}

func generate_routes(routes string, laddr string) []string {
	result := make([]string, 0)
	result = append(result, simplify(laddr))
	for _, next := range common.ToArray(routes) {
		result = append(result, next)
	}
	return result
}

func simplify(addr string) string {
	tokens := strings.Split(addr, "/")
	return tokens[0]
}

var POOL = pool.NewFixedPool(200, maker)

func test_channel() {
	c := make(chan []byte, 1000)
	start := time.Now()
	wg := new(sync.WaitGroup)
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 10000000; i++ {
			buffer, _ := POOL.Borrow()
			for j := 0; j < len(buffer); j++ {
				buffer[j] = 1
			}
			c <- buffer
		}
		close(c)
	}()

	go func() {
		defer wg.Done()
		var count int64 = 0
		for buffer := range c {
			// received
			for _, b := range buffer {
				count += int64(b)
			}
			POOL.Return(buffer)
		}
		fmt.Println("Total bytes", count)
	}()
	wg.Wait()
	fmt.Println(POOL.CreatedCount())
	fmt.Println("Total time", time.Since(start))
}
func setup_server_transport(ctx context.Context, certName string) (transport.Transport, error) {
	config := transport.QuicConfig{
		CertFile: "server.pem",
		KeyFile:  "server.key",
		CAFile:   "ca.pem",
	}
	ss, err := transport.NewQuicServerTransport(config, bind_string, ctx, certName)
	return ss, err
}

func setup_client_transport(ctx context.Context, certName string) (transport.Transport, error) {
	config := transport.QuicConfig{
		CertFile: "client.pem",
		KeyFile:  "client.key",
		CAFile:   "ca.pem",
	}
	return transport.NewQuicClientTransport(config, server_address, ctx, certName)
}
