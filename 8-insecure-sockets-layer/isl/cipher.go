package isl

import (
	"bytes"
	"errors"
	"io"
	"math/bits"

	"github.com/anantadwi13/protohackers/8-insecure-sockets-layer/util"
)

var (
	ErrCipherOperationNilUnderlyingRW = errors.New("underlying reader/writer not initialized")
	ErrCipherSpecInvalid              = errors.New("cipher spec is invalid")
)

type CipherOperation interface {
	SetUnderlyingReader(reader io.Reader)
	SetUnderlyingWriter(writer io.Writer)
	io.ReadWriter
	Copy() CipherOperation
}

type CipherSpec struct {
	Operations []CipherOperation
}

func (c *CipherSpec) NewReader(reader io.Reader) io.Reader {
	for i := len(c.Operations) - 1; i >= 0; i-- {
		op := c.Operations[i].Copy()
		op.SetUnderlyingReader(reader)
		reader = op
	}
	return reader
}

func (c *CipherSpec) NewWriter(writer io.Writer) io.Writer {
	for i := len(c.Operations) - 1; i >= 0; i-- {
		op := c.Operations[i].Copy()
		op.SetUnderlyingWriter(writer)
		writer = op
	}
	return writer
}

const byteSize = 1 << 8

type cipherOperation struct {
	reader io.Reader
	writer io.Writer
}

func (c *cipherOperation) SetUnderlyingReader(reader io.Reader) {
	if c.reader != nil {
		return
	}
	c.reader = reader
}

func (c *cipherOperation) SetUnderlyingWriter(writer io.Writer) {
	if c.writer != nil {
		return
	}
	c.writer = writer
}

type CipherOperationReverseBits struct {
	cipherOperation
}

func (c *CipherOperationReverseBits) Read(p []byte) (n int, err error) {
	if c.reader == nil {
		return 0, ErrCipherOperationNilUnderlyingRW
	}
	n, err = c.reader.Read(p)
	for i, b := range p {
		p[i] = bits.Reverse8(b)
	}
	return
}

func (c *CipherOperationReverseBits) Write(p []byte) (n int, err error) {
	if c.writer == nil {
		return 0, ErrCipherOperationNilUnderlyingRW
	}
	for i, b := range p {
		p[i] = bits.Reverse8(b)
	}
	n, err = c.writer.Write(p)
	return
}

func (c *CipherOperationReverseBits) Copy() CipherOperation {
	return &CipherOperationReverseBits{}
}

type CipherOperationXor struct {
	N byte
	cipherOperation
}

func (c *CipherOperationXor) Read(p []byte) (n int, err error) {
	if c.reader == nil {
		return 0, ErrCipherOperationNilUnderlyingRW
	}
	n, err = c.reader.Read(p)
	for i, b := range p {
		p[i] = b ^ c.N
	}
	return
}

func (c *CipherOperationXor) Write(p []byte) (n int, err error) {
	if c.writer == nil {
		return 0, ErrCipherOperationNilUnderlyingRW
	}
	for i, b := range p {
		p[i] = b ^ c.N
	}
	n, err = c.writer.Write(p)
	return
}

func (c *CipherOperationXor) Copy() CipherOperation {
	return &CipherOperationXor{N: c.N}
}

type CipherOperationXorPos struct {
	pos int
	cipherOperation
}

func (c *CipherOperationXorPos) Read(p []byte) (n int, err error) {
	if c.reader == nil {
		return 0, ErrCipherOperationNilUnderlyingRW
	}
	n, err = c.reader.Read(p)
	for i, b := range p {
		p[i] = b ^ byte(c.pos+i)
	}
	c.pos += n
	return
}

func (c *CipherOperationXorPos) Write(p []byte) (n int, err error) {
	if c.writer == nil {
		return 0, ErrCipherOperationNilUnderlyingRW
	}
	for i, b := range p {
		p[i] = b ^ byte(c.pos+i)
	}
	n, err = c.writer.Write(p)
	c.pos += n
	return
}

func (c *CipherOperationXorPos) Copy() CipherOperation {
	return &CipherOperationXorPos{}
}

type CipherOperationAdd struct {
	N byte
	cipherOperation
}

func (c *CipherOperationAdd) Read(p []byte) (n int, err error) {
	if c.reader == nil {
		return 0, ErrCipherOperationNilUnderlyingRW
	}
	n, err = c.reader.Read(p)
	for i, b := range p {
		p[i] = b - c.N
	}
	return
}

func (c *CipherOperationAdd) Write(p []byte) (n int, err error) {
	if c.writer == nil {
		return 0, ErrCipherOperationNilUnderlyingRW
	}
	for i, b := range p {
		p[i] = b + c.N
	}
	n, err = c.writer.Write(p)
	return
}

func (c *CipherOperationAdd) Copy() CipherOperation {
	return &CipherOperationAdd{N: c.N}
}

type CipherOperationAddPos struct {
	pos int
	cipherOperation
}

func (c *CipherOperationAddPos) Read(p []byte) (n int, err error) {
	if c.reader == nil {
		return 0, ErrCipherOperationNilUnderlyingRW
	}
	n, err = c.reader.Read(p)
	for i, b := range p {
		p[i] = b - byte(c.pos+i)
	}
	c.pos += n
	return
}

func (c *CipherOperationAddPos) Write(p []byte) (n int, err error) {
	if c.writer == nil {
		return 0, ErrCipherOperationNilUnderlyingRW
	}
	for i, b := range p {
		p[i] = b + byte(c.pos+i)
	}
	n, err = c.writer.Write(p)
	c.pos += n
	return
}

func (c *CipherOperationAddPos) Copy() CipherOperation {
	return &CipherOperationAddPos{}
}

var dataValidation = []byte("4x dog,5x car\n")

func ReadCipherSpec(r io.Reader) (*CipherSpec, error) {
	buf := util.GetBytes(1)
	defer util.PutBytes(buf)

	cipherSpec := &CipherSpec{}

LOOP:
	for {
		n, err := r.Read(buf)
		if err != nil {
			return nil, errors.Join(ErrCipherSpecInvalid, err)
		}
		if n != 1 {
			return nil, errors.Join(ErrCipherSpecInvalid, errors.New("can't get cipher type from reader"))
		}
		switch buf[0] {
		case 0x00:
			break LOOP
		case 0x01:
			cipherSpec.Operations = append(cipherSpec.Operations, &CipherOperationReverseBits{})
		case 0x02:
			n, err = r.Read(buf)
			if err != nil {
				return nil, errors.Join(ErrCipherSpecInvalid, err)
			}
			if n != 1 {
				return nil, errors.Join(ErrCipherSpecInvalid, errors.New("can't get cipher value from reader"))
			}
			cipherSpec.Operations = append(cipherSpec.Operations, &CipherOperationXor{
				N: buf[0],
			})
		case 0x03:
			cipherSpec.Operations = append(cipherSpec.Operations, &CipherOperationXorPos{})
		case 0x04:
			n, err = r.Read(buf)
			if err != nil {
				return nil, errors.Join(ErrCipherSpecInvalid, err)
			}
			if n != 1 {
				return nil, errors.Join(ErrCipherSpecInvalid, errors.New("can't get cipher value from reader"))
			}
			cipherSpec.Operations = append(cipherSpec.Operations, &CipherOperationAdd{
				N: buf[0],
			})
		case 0x05:
			cipherSpec.Operations = append(cipherSpec.Operations, &CipherOperationAddPos{})
		default:
			return nil, errors.Join(ErrCipherSpecInvalid, errors.New("invalid cipher type"))
		}
	}

	// validate no-op
	reader := cipherSpec.NewReader(bytes.NewReader(dataValidation))
	bufValidation := util.GetBytes(len(dataValidation))
	defer util.PutBytes(bufValidation)
	n, err := reader.Read(bufValidation)
	if err != nil {
		return nil, errors.Join(ErrCipherSpecInvalid, err)
	}
	if bytes.Equal(dataValidation, bufValidation[:n]) {
		return nil, errors.Join(ErrCipherSpecInvalid, errors.New("no-op ciphers"))
	}

	return cipherSpec, nil
}
