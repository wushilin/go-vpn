package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"log"
)

type Coder interface {
	Encrypt(from []byte, to []byte) []byte
	Decrypt(from []byte, to []byte) []byte
}

type DummyCoder struct{}

func (v DummyCoder) Encrypt(env []byte, to []byte) []byte {
	copy(to, env)
	return to
}

func (v DummyCoder) Decrypt(env []byte, to []byte) []byte {
	copy(to, env)
	return to
}

type AESCoder struct {
	Cipher cipher.Block
	Random io.Reader
}

func NewAESCoder(key []byte) (*AESCoder, error) {
	cipher, err := aes.NewCipher(key)
	r := rand.Reader
	if err != nil {
		return nil, err
	}
	return &AESCoder{Cipher: cipher, Random: r}, nil
}
func (v *AESCoder) Encrypt(env []byte, out []byte) []byte {
	iv := out[:aes.BlockSize]
	_, err := io.ReadFull(v.Random, iv)
	if err != nil {
		log.Fatal("Can't read random: ", err)
	}
	cfb := cipher.NewCFBEncrypter(v.Cipher, iv)
	cfb.XORKeyStream(out[aes.BlockSize:], env)
	return out[:aes.BlockSize+len(env)]
}

func (v *AESCoder) Decrypt(ciphertext []byte, out []byte) []byte {
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(v.Cipher, iv)
	cfb.XORKeyStream(out, ciphertext)
	return out[:len(ciphertext)]
}
