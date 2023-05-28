package transport

import (
	"io"

	"github.com/wushilin/go-vpn/message"
)

type Transport interface {
	io.ReadWriteCloser
	ReadControlCommand() (message.Command, error)
	WriteControlCommand(command message.Command) (int, error)
	GetStats() string
}

type Buffer struct {
	Slice []byte
	Start int
	End   int
}

func WrapBuffer(slice []byte, start, end int) Buffer {
	return Buffer{
		Slice: slice,
		Start: start,
		End:   end,
	}
}
