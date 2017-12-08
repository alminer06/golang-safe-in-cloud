package sic

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/binary"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"
	"golang.org/x/crypto/pbkdf2"
)

// ErrIncorrectPassword means that the credentials are incorrect
var ErrIncorrectPassword = errors.New("Incorrect credentials")

// Decrypt decrypts a SafeInCloud database by a given file (e.g. os.Open)
// and a password
func Decrypt(file io.Reader, password string) ([]byte, error) {
	data := bufio.NewReader(file)
	var magic uint16
	// Decrypt the FD
	if err := binary.Read(data, binary.LittleEndian, &magic); err != nil {
		return nil, errors.Wrap(err, "could not read magic")
	}
	if _, err := data.ReadByte(); err != nil {
		return nil, errors.Wrap(err, "could not read sver")
	}
	salt, err := readByteArray(data)
	if err != nil {
		return nil, errors.Wrap(err, "could not read salt")
	}
	nonce, err := readByteArray(data)
	if err != nil {
		return nil, errors.Wrap(err, "could not read nonce")
	}
	pwd := pbkdf2.Key([]byte(password), salt, 10000, 32, sha1.New)
	_, err = readByteArray(data) // Idk what this is; salt but not necessary?!
	if err != nil {
		return nil, errors.Wrap(err, "could not read salt")
	}
	block, err := readByteArray(data)
	if err != nil {
		return nil, errors.Wrap(err, "could not read subfd")
	}
	if err := decryptAES(pwd, nonce, &block); err != nil {
		return nil, errors.Wrap(err, "could not decrypt aes")
	}
	fd := bufio.NewReader(bytes.NewBuffer(block))
	encFile, err := ioutil.ReadAll(data)
	if err != nil {
		return nil, errors.Wrap(err, "could not read remaining encrypted content")
	}
	nonce, err = readByteArray(fd)
	if err == io.ErrUnexpectedEOF {
		return nil, ErrIncorrectPassword
	} else if err != nil {
		return nil, errors.Wrap(err, "could not read nonce")
	}
	pwd, err = readByteArray(fd)
	if err != nil {
		return nil, ErrIncorrectPassword
	}
	if _, err = readByteArray(fd); err != nil {
		return nil, err
	}
	if err := decryptAES(pwd, nonce, &encFile); err != nil {
		return nil, errors.Wrap(err, "could not decrypt aes")
	}
	zReader, err := zlib.NewReader(bytes.NewReader(encFile))
	if err != nil {
		return nil, err
	}
	defer zReader.Close()
	return ioutil.ReadAll(zReader)
}

func decryptAES(pwd, nonce []byte, content *[]byte) error {
	block, err := aes.NewCipher(pwd)
	if err != nil {
		return errors.Wrap(err, "could not create cipher")
	}
	cipher.NewCBCDecrypter(block, nonce).CryptBlocks(*content, *content)
	return nil
}

// readByteArray reads a byte array with the given size in the next byte
func readByteArray(data *bufio.Reader) ([]byte, error) {
	size, err := data.ReadByte()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, size)
	if err = binary.Read(data, binary.LittleEndian, &buf); err != nil {
		return nil, err
	}
	return buf, nil
}
