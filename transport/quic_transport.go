package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"log"
	"os"
	"time"

	quic "github.com/quic-go/quic-go"
)

type QuicConfig struct {
	KeyFile  string
	CertFile string
	CAFile   string
}

func (v QuicConfig) GenerateTLSConfig(is_server bool) *tls.Config {
	key_bytes, err := os.ReadFile(v.KeyFile)
	if err != nil {
		log.Fatal(err)
	}

	cert_bytes, err := os.ReadFile(v.CertFile)
	if err != nil {
		log.Fatal(err)
	}

	ca_bytes, err := os.ReadFile(v.CAFile)
	if err != nil {
		log.Fatal(err)
	}

	tlsCert, err := tls.X509KeyPair(cert_bytes, key_bytes)
	if err != nil {
		panic(err)
	}
	cert_pool := x509.NewCertPool()
	block, _ := pem.Decode(ca_bytes)

	ca, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		panic(err)
	}
	cert_pool.AddCert(ca)
	if is_server {
		return &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			ClientAuth:   tls.RequestClientCert, // used by server
			ClientCAs:    cert_pool,             // used by server
			NextProtos:   []string{"quic"},
		}
	} else {
		return &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			RootCAs:      cert_pool, // used by client
			NextProtos:   []string{"quic"},
		}
	}
}

func DefaultConfig() *quic.Config {
	return &quic.Config{
		KeepAlivePeriod: 3 * time.Second,
		MaxIdleTimeout:  10 * time.Second,
	}
}
func NewQuicServerTransport(config QuicConfig, bind_string string) Transport {
	listener, err := quic.ListenAddr(bind_string, config.GenerateTLSConfig(true), DefaultConfig())
	log.Println("Server listening on ", bind_string)
	if err != nil {
		panic(err)
	}
	conn, err := listener.Accept(context.Background())
	if err != nil {
		panic(err)
	}
	log.Println("Server Accept ok")
	stream, err := conn.AcceptStream(context.Background())

	if err != nil {
		panic(err)
	}
	log.Println("Server Accept stream ok")
	return stream
}

func NewQuicClientTransport(config QuicConfig, server_addr string) Transport {
	log.Println("Client connecting to ", server_addr)
	conn, err := quic.DialAddr(server_addr, config.GenerateTLSConfig(false), DefaultConfig())
	if err != nil {
		panic(err)
	}
	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		panic(err)
	}
	log.Println("Client connected OK")
	stream.Write([]byte{0, 0, 0})
	return stream
}
