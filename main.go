package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/context"

	"github.com/dustin/go-humanize"
	"github.com/songgao/water"
	"github.com/wushilin/go-vpn/common"
	"github.com/wushilin/go-vpn/piper"
	"github.com/wushilin/go-vpn/stats"
	"github.com/wushilin/go-vpn/transport"
)

var server_mode bool
var server_address string
var bind_string string
var laddr = ""
var routes = ""
var commonName = ""
var device_name = ""

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

func print_stats(v *stats.GlobalStats, ctx context.Context) {
	log.Printf("Print Stats Started")
	var run = true
	for run {
		select {
		case <-ctx.Done():
			run = false
		case <-time.After(30 * time.Second):
			downloaded := v.DownloadedBytes()
			uploaded := v.UploadedBytes()
			downloaded_str := humanize.Bytes(downloaded)
			uploaded_str := humanize.Bytes(uploaded)
			reconnected_count := v.ReconnectedCount()
			log.Printf("Sent: %s, Received: %s, Reconnect Count: %d", uploaded_str, downloaded_str, reconnected_count)
		}
		//log.Println("Transport Stats: ", v.Transport.GetStats())
	}
	log.Printf("Print Stats Stopped")
}

func main() {

	stop_context, cancel_function := context.WithCancel(context.TODO())
	flag.BoolVar(&server_mode, "l", false, "Listen. This means it will run as server mode. Default is client mode")
	flag.StringVar(&server_address, "s", "", "Server to Connect To. Required client param; no default")
	flag.StringVar(&bind_string, "b", "", "Bind address. Required server param; no default")
	flag.StringVar(&laddr, "laddr", "", "Local address in CIDR notation(e.g. 10.1.0.10/24). Default server: `10.54.0.10/24`, default client: `10.54.0.11/24`")
	flag.StringVar(&routes, "route", "", "Network to ask remote to route to local in cidr;cidr; format (10.0.0.0/8;192.168.44.7/32;...). Default is local address only")
	flag.StringVar(&commonName, "commonName", "", "Allowed remote certificate common name, default is No Check")
	flag.StringVar(&device_name, "tunname", "TUN17", "Use alternate device name. Default is `TUN17`")
	flag.Parse()
	var global_stats = stats.New()
	go print_stats(global_stats, stop_context)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT, syscall.SIGHUP, syscall.SIGQUIT)
	go func() {
		<-sigs
		fmt.Println("")
		cancel_function()
	}()
	// // // if laddr == "" {
	// // // 	if server_mode {
	// // // 		laddr = "10.54.0.10/24"
	// // // 		log.Printf("Default local addres set to %s\n", laddr)
	// // // 	} else {
	// // // 		laddr = "10.54.0.11/24"
	// // // 		log.Printf("Default local addres set to %s\n", laddr)
	// // // 	}
	// // // }
	if routes == "" {
		log.Printf("Not requesting additional routing from other party. you can specify -route parameter to request")
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
					log.Println("Deleting interface ", device_name)
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
			config.Name = device_name
			var err error
			iface, err = water.New(config)
			if err != nil {
				log.Fatal(err)
			}
			if !common.BringUpLink(device_name) {
				log.Fatalf("Failed to bring link UP\n")
			}
			if laddr != "" {
				log.Printf("Using specified local address %s\n", laddr)
				if !common.SetIPAddress(device_name, laddr) {
					log.Fatalf("Failed to set IP Address to %s\n", laddr)
				}
			}
			if server_mode {
				trans, err = setup_server_transport(stop_context, commonName)
			} else {
				trans, err = setup_client_transport(stop_context, commonName)
			}
			if err != nil {
				log.Printf("Setup Transport Error: %s\n", err)
				return
			}

			pipe, err = piper.NewPipe(iface, trans, generate_routes(routes, laddr), global_stats)

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
			global_stats.IncreaseReconnectCount()
		}()
	}
}

func generate_routes(routes string, laddr string) []string {
	result := make([]string, 0)
	result = append(result, simplify(laddr))
	result = append(result, common.ToArray(routes)...)
	return result
}

func simplify(addr string) string {
	tokens := strings.Split(addr, "/")
	return tokens[0]
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
