package message

import "errors"

type Command struct {
	Type   CMD_TYPE
	Length int
	Data   []byte
}

func (v Command) IsOK() bool {
	return v.Type == CMD_OK
}

func (v Command) IsFail() bool {
	return v.Type == CMD_FAIL
}
func OK() Command {
	result, _ := WrapCommand(CMD_OK, []byte{})
	return result
}

func FAIL() Command {
	result, _ := WrapCommand(CMD_FAIL, []byte{})
	return result
}

type CMD_TYPE byte

const CMD_SUBNET_UPDATE CMD_TYPE = 0x01
const CMD_OK CMD_TYPE = 0x00
const CMD_FAIL CMD_TYPE = 0xf0

func WrapCommand(cmdType CMD_TYPE, data []byte) (Command, error) {
	length := len(data)
	if length > 0xffff {
		return Command{}, errors.New("Command too large")
	}
	return Command{
		Type:   cmdType,
		Length: length,
		Data:   data,
	}, nil
}

func ParseCommand(data []byte) (Command, error) {
	length := int(data[1])*256 + int(data[2])
	if len(data) != length+3 {
		return Command{}, errors.New("length mismatch")
	}
	return Command{
		Type:   CMD_TYPE(data[0]),
		Length: length,
		Data:   data[3:],
	}, nil
}
