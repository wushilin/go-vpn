package transport

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	quic "github.com/quic-go/quic-go"
	"github.com/wushilin/go-vpn/message"
	"github.com/wushilin/pool"
)

type QuicClientTransport struct {
	Conn          quic.Connection
	Streams       []quic.Stream
	ControlStream quic.Stream
	BufferChannel chan Buffer
	BufferPool    *pool.Pool[[]byte]
}

// Read may read from a random channel by order of insertion
func (v *QuicClientTransport) Read(buffer []byte) (int, error) {
	return qRead(v.BufferPool, v.BufferChannel, buffer)
}

func (v *QuicClientTransport) GetStats() string {
	return get_stats(v.BufferPool)
}

func (v *QuicClientTransport) ReadControlCommand() (message.Command, error) {
	return ReadCommand(v.ControlStream)
}

func (v *QuicClientTransport) WriteControlCommand(command message.Command) (int, error) {
	return WriteCommand(v.ControlStream, command)
}

// Write write to a random channel
func (v *QuicClientTransport) Write(buffer []byte) (int, error) {
	size := len(buffer)
	if size > 0xFFFF {
		return 0, errors.New("buffer too long. Expect less than 0xffff bytes")
	}

	for {
		var selected int = rand.Intn(len(v.Streams))
		var stream = v.Streams[selected]
		if stream == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		stream.Write([]byte{byte(size / 256), byte(size % 256)})
		return stream.Write(buffer)
	}
}

func (v *QuicClientTransport) Close() error {
	v.ControlStream.Close()
	return CloseConn(v.Conn, CLOSE)
}
func (v *QuicClientTransport) RunReaders() error {
	return runReaders(v.BufferPool, v.Conn, v.Streams, v.BufferChannel, false)
}

func NewQuicClientTransport(config QuicConfig, server_addr string, ctx context.Context, certName string) (result Transport, cause error) {
	var conn quic.Connection
	var control_stream quic.Stream
	var err error
	var cleanup = func() {
		if result == nil {
			log.Printf("Cleaning up client connections %s\n", cause)
			if control_stream != nil {
				control_stream.Close()
			}
			if conn != nil {
				CloseConn(conn, CLOSE)
			}
		}
	}

	defer cleanup()
	conn, err = quic.DialAddr(ctx, server_addr, config.GenerateTLSConfig(false), DefaultConfig())
	if err != nil {
		return nil, err
	}
	if certName != "" {
		actual_cert_name := conn.ConnectionState().TLS.PeerCertificates[0].Subject.CommonName
		if certName != actual_cert_name {
			log.Printf("%v\n", conn.ConnectionState().TLS.PeerCertificates[0].Subject.CommonName)
			return nil, fmt.Errorf("invalid cert name %s != expected: %s", actual_cert_name, certName)
		}
	}
	control_stream, err = conn.OpenStreamSync(context.Background())
	if err != nil {
		return nil, err
	}
	if err := Ping(control_stream); err != nil {
		return nil, err
	}
	resultp := &QuicClientTransport{
		Conn:          conn,
		ControlStream: control_stream,
		Streams:       make([]quic.Stream, STREAMS),
		BufferChannel: make(chan Buffer, 1000),
		BufferPool: pool.NewFixedPool(300, func() ([]byte, error) {
			return make([]byte, 4096), nil
		}).WithIdleTimeout(99999999).WithTester(func(b []byte) bool {
			return true
		}),
	}
	go resultp.RunReaders()
	result = resultp
	cause = nil
	return result, cause
}
