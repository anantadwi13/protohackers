package isl

import (
	"errors"
	"io"
	"math/bits"
)

var (
	ErrCipherOperationNilUnderlyingRW = errors.New("underlying reader/writer not initialized")
)

type CipherOperation interface {
	SetUnderlyingReader(reader io.Reader)
	SetUnderlyingWriter(writer io.Writer)
	io.ReadWriter
}

type CipherSpec struct {
	Operations []CipherOperation
}

func (c *CipherSpec) NewReader(reader io.Reader) io.Reader {
	for _, op := range c.Operations {
		op.SetUnderlyingReader(reader)
		reader = op
	}
	return reader
}

func (c *CipherSpec) NewWriter(writer io.Writer) io.Writer {
	for i := len(c.Operations) - 1; i >= 0; i-- {
		c.Operations[i].SetUnderlyingWriter(writer)
		writer = c.Operations[i]
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
