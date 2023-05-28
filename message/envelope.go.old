package message

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"sync/atomic"
	"time"
)

// CLIENT CONNECT
// CHALLENGE: SHA256; HELLO_WORLD;
// RESPONSE: SHA256; HASH(KEY+HELLO_WORLD)

// bidirectional pipe
// if: ANNOUNCE: Sequence CIDR1;CIDR2;CIDR3;
// ans: ANNOUNCE OK/NOTOK
// if: data
// fall through and ignore
// byte command type, 0x00 general OK, 0x01 auth_challenge, 0x02 auth response,  0x03 reannounce subnet 0xff data frame
// 2 byte body length, if no, set 0,0
// data in bytes, should be exactly the same as the data
const CMD_OK byte = 0x0

// It is not successful!
const CMD_NOT_OK byte = 0x10

// IN AUTH CHALLENGE IT SENDS CHALLENGE_SEQUENCE;HASH_ALGORITHM;STRING
const CMD_AUTH_CHALLENGE = 0x1

// IN AUTH CHALLENGE RESPONSE, it sends SEQUENCE;STRING
const CMD_AUTH_RESPONSE = 0x2

// in ANNOUNCE_SUBNETS, each party can request routing in the format of
// sequence CIDR1;CIDE2;CIDR3;? format. The last ; is optional
const CMD_ANNOUNCE_SUBNETS = 0x3

// in ANNOUNCE_OK: sequence accepted(CIDR;CIDR;)# revoked(CIDR;CIDR;)
const CMD_ANNOUCE_RESULT = 0x4

// Other party wants to close.
const CMD_BYE = 0xfe

const CMD_DATAFRAME = 0xff

type Envelope []byte

func OK() Envelope {
	return []byte{CMD_OK, 0, 0}
}

func NotOK() Envelope {
	return []byte{CMD_NOT_OK, 0, 0}
}

func WrapEnvelope(etype byte, data []byte) Envelope {
	buffer := make([]byte, len(data)+3)
	buffer[0] = etype
	buffer[1] = byte(len(data) / 256)
	buffer[2] = byte(len(data) % 256)

	for i := 0; i < len(data); i++ {
		buffer[3+i] = data[i]
	}
	return buffer
}

// Decode a Envelope from the reader, and not anything more, not anything less
func DecodeEnvelopFrom(reader io.Reader, env Envelope) (int, error) {
	buffer := []byte(env)
	read_count := 0
	for read_count < 3 {
		nread, err := reader.Read(buffer[read_count:3])
		read_count += nread
		if err != nil {
			return read_count, err
		}
	}
	size_int := int(buffer[1])*256 + int(buffer[2])
	if size_int >= len(buffer)-3 {
		// not enough
		return read_count, errors.New("Client intended size too big " + fmt.Sprintf("%d", size_int))
	}
	limit := size_int + 3
	for read_count < limit {
		nread, err := reader.Read(buffer[read_count:limit])
		read_count += nread
		if err != nil {
			return read_count, err
		}
	}
	return read_count, nil
}

func (v Envelope) IsOK() bool {
	return v[0] == CMD_OK && v[1] == 0 && v[2] == 0
}

func (v Envelope) IsNotOk() bool {
	return v[0] == CMD_NOT_OK && v[1] == 0 && v[2] == 0
}

func (v Envelope) Type() byte {
	return v[0]
}

func (v Envelope) DataSize() int {
	return int(v[1])*256 + int(v[2])
}

func (v Envelope) Data() []byte {
	return v[3:]
}

var SERVER_SEQUENCE int64 = 0

func NextSequence() int {
	return int(atomic.AddInt64(&SERVER_SEQUENCE, 1) % 100000000)
}

func RandomBytes(buffer []byte) (int, error) {
	return rand.Read(buffer)
}

func NewChallenge() Envelope {
	buffer := make([]byte, 64)
	buffer[0] = CMD_AUTH_CHALLENGE
	buffer[1] = 0
	buffer[2] = 61
	timepart := time.Now().UnixNano() / int64(time.Millisecond)
	dynamic_part := fmt.Sprintf("%016d-%08d", timepart, NextSequence())
	fmt.Println(dynamic_part)
	for i := 0; i < len(dynamic_part); i++ {
		buffer[3+i] = dynamic_part[i]
	}
	RandomBytes(buffer[3+len(dynamic_part):])

	return buffer
}

func Respond(challenge Envelope, secret []byte) Envelope {
	if len(secret) != 32 {
		log.Fatal("Secret must be 32 characters long")
	}
	hash := sha256.New()
	data := challenge.Data()
	if _, err := io.Copy(hash, bytes.NewReader(data)); err != nil {
		log.Fatal(err)
	}
	if _, err := io.Copy(hash, bytes.NewReader(secret)); err != nil {
		log.Fatal(err)
	}
	response := hash.Sum(nil)

	return WrapEnvelope(CMD_AUTH_RESPONSE, response)
}

func EnvelopeEqual(env1, env2 Envelope) bool {
	if len(env1) != len(env2) {
		return false
	}

	for i := 0; i < len(env1); i++ {
		if env1[i] != env2[i] {
			return false
		}
	}
	return true
}

func VerifyChallenge(secret []byte, challenge Envelope, response Envelope) bool {
	local_response := Respond(challenge, secret)
	log.Printf("Expect %v, got %v\n", local_response, response)
	return EnvelopeEqual(local_response, response)
}
