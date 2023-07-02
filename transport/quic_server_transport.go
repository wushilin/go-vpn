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

type QuicServerTransport struct {
	Listener      *quic.Listener
	Conn          quic.Connection
	Streams       []quic.Stream
	ControlStream quic.Stream
	BufferChannel chan Buffer
	BufferPool    *pool.Pool[[]byte]
}

// Sync functino to perform all reading. When it returns, all streams are closed
func (v *QuicServerTransport) RunReaders() error {
	return runReaders(v.BufferPool, v.Conn, v.Streams, v.BufferChannel, true)
}

// Read may read from a random channel by order of insertion
func (v *QuicServerTransport) Read(buffer []byte) (int, error) {
	return qRead(v.BufferPool, v.BufferChannel, buffer)
}

func (v *QuicServerTransport) ReadControlCommand() (message.Command, error) {
	return ReadCommand(v.ControlStream)
}

func (v *QuicServerTransport) WriteControlCommand(command message.Command) (int, error) {
	return WriteCommand(v.ControlStream, command)
}
func (v *QuicServerTransport) GetStats() string {
	return get_stats(v.BufferPool)
}

// Write write to a random channel
func (v *QuicServerTransport) Write(buffer []byte) (int, error) {
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
		stream.Write([]byte{byte(size / 256)})
		stream.Write([]byte{byte(size % 256)})
		return stream.Write(buffer)
	}
}

func (v *QuicServerTransport) Close() error {
	if v.Listener != nil {
		v.Listener.Close()
	}
	v.ControlStream.Close()
	return CloseConn(v.Conn, CLOSE)
}

func NewQuicServerTransport(config QuicConfig, bind_string string, ctx context.Context, certName string) (result Transport, cause error) {
	var listener *quic.Listener = nil
	var conn quic.Connection = nil
	var control_stream quic.Stream = nil
	var err error
	cleanup := func() {
		if result == nil {
			log.Printf("Setup ServerTransport failed: %s\n", cause)
			if control_stream != nil {
				control_stream.Close()
			}
			if conn != nil {
				CloseConn(conn, CLOSE)
			}
			if listener != nil {
				listener.Close()
			}
			log.Printf("Cleaning up of ServerTransport done\n")
		}
	}
	defer cleanup()
	listener, err = quic.ListenAddr(bind_string, config.GenerateTLSConfig(true), DefaultConfig())
	log.Println("Server listening on ", bind_string)
	if err != nil {
		return nil, err
	}
	conn, err = listener.Accept(ctx)
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

	control_stream, err = conn.AcceptStream(context.Background())
	if err != nil {
		return nil, err
	}
	if err := Pong(control_stream); err != nil {
		return nil, err
	}

	resultp := &QuicServerTransport{
		Listener:      listener,
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
