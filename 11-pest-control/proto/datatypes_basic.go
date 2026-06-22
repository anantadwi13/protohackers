package proto

import (
	"encoding/binary"
	"errors"
	"io"
	"reflect"
)

var (
	ErrInvalidType = errors.New("invalid type")
)

type Marshaler interface {
	Marshal(w io.Writer) (preChecksum byte, err error)
}

type Unmarshaler interface {
	Unmarshal(r io.Reader) (n int, preChecksum byte, err error)
}

type DataType interface {
	Marshaler
	Unmarshaler
	Reset()
	BytesLength() uint32
}

type Byte byte

func NewByte(val byte) *Byte {
	return new(Byte(val))
}

func (b *Byte) Value() byte {
	return byte(*b)
}

func (b *Byte) BytesLength() uint32 {
	return 1
}

func (b *Byte) Marshal(w io.Writer) (byte, error) {
	val := byte(*b)
	return val, binary.Write(w, binary.BigEndian, val)
}

func (b *Byte) Unmarshal(r io.Reader) (int, byte, error) {
	var (
		buf = make([]byte, 1) // todo use pool
		val byte
		n   int
	)
	n, err := io.ReadFull(r, buf)
	if err != nil {
		return n, 0, err
	}
	val = buf[0]
	*b = Byte(val)
	return n, val, nil
}

func (b *Byte) Reset() {
	var val byte
	*b = Byte(val)
}

type U32 uint32

func (u *U32) BytesLength() uint32 {
	return 4
}

func NewU32(val uint32) *U32 {
	return new(U32(val))
}

func (u *U32) Value() uint32 {
	return uint32(*u)
}

func (u *U32) Marshal(w io.Writer) (byte, error) {
	buf := make([]byte, 4) // todo use pool
	binary.BigEndian.PutUint32(buf, uint32(*u))
	preChecksum := preChecksumBytes(buf)

	_, err := w.Write(buf)
	if err != nil {
		return preChecksum, err
	}
	return preChecksum, nil
}

func (u *U32) Unmarshal(r io.Reader) (int, byte, error) {
	var (
		buf         = make([]byte, 4) // todo use pool
		val         uint32
		n           int
		preChecksum byte
	)
	newN, err := io.ReadFull(r, buf)
	n += newN
	preChecksum = preChecksumBytes(buf)
	if err != nil {
		return n, preChecksum, err
	}
	val = binary.BigEndian.Uint32(buf)
	*u = U32(val)
	return n, preChecksum, nil
}

func (u *U32) Reset() {
	var val uint32
	*u = U32(val)
}

type String string

func NewString(val string) *String {
	return new(String(val))
}

func (s *String) Value() string {
	return string(*s)
}

func (s *String) BytesLength() uint32 {
	return uint32(len(*s)) + 4
}

func (s *String) Marshal(w io.Writer) (byte, error) {
	buf := make([]byte, 4) // todo use pool
	binary.BigEndian.PutUint32(buf, uint32(len(*s)))
	preChecksum := preChecksumBytes(buf)
	_, err := w.Write(buf)
	if err != nil {
		return preChecksum, err
	}

	_, err = io.WriteString(w, string(*s))
	preChecksum += preChecksumBytes([]byte(*s))
	if err != nil {
		return preChecksum, err
	}
	return preChecksum, nil
}

func (s *String) Unmarshal(r io.Reader) (int, byte, error) {
	var (
		buf         = make([]byte, 4) // todo use pool
		length      = uint32(0)
		n           int
		preChecksum byte
	)

	newN, err := io.ReadFull(r, buf)
	n += newN
	preChecksum = preChecksumBytes(buf)
	if err != nil {
		return n, preChecksum, err
	}
	length = binary.BigEndian.Uint32(buf)

	buf = make([]byte, length)      // todo use pool
	newN, err = io.ReadFull(r, buf) // todo check
	n += newN
	preChecksum += preChecksumBytes(buf)
	if err != nil {
		return n, preChecksum, err
	}
	*s = String(buf)
	return n, preChecksum, nil
}

func (s *String) Reset() {
	var val string
	*s = String(val)
}

type Array[T DataType] []T

func NewArray[T DataType](data ...T) *Array[T] {
	return new(Array[T](data))
}

func (a *Array[T]) Value() []T {
	return *a
}

func (a *Array[T]) BytesLength() uint32 {
	length := uint32(4)
	for _, v := range *a {
		length += v.BytesLength()
	}
	return length
}

func newDataTypeValue[T DataType]() T {
	var zero T
	typ := reflect.TypeOf(zero)
	if typ != nil && typ.Kind() == reflect.Pointer {
		return reflect.New(typ.Elem()).Interface().(T)
	}
	return zero
}

func (a *Array[T]) Elements() []T {
	return *a
}

func (a *Array[T]) Marshal(w io.Writer) (byte, error) {
	var (
		buf         = make([]byte, 4) // todo use pool
		length      = len(*a)
		preChecksum byte
	)

	binary.BigEndian.PutUint32(buf, uint32(length))
	preChecksum += preChecksumBytes(buf)
	_, err := w.Write(buf)
	if err != nil {
		return preChecksum, err
	}

	for _, v := range *a {
		var curPreChecksum byte
		curPreChecksum, err = v.Marshal(w)
		preChecksum += curPreChecksum
		if err != nil {
			return preChecksum, err
		}
	}
	return preChecksum, nil
}

func (a *Array[T]) Unmarshal(r io.Reader) (int, byte, error) {
	var (
		buf         = make([]byte, 4) // todo use pool
		length      = uint32(0)
		n           int
		preChecksum byte
	)
	newN, err := io.ReadFull(r, buf)
	n += newN
	preChecksum += preChecksumBytes(buf)
	if err != nil {
		return n, preChecksum, err
	}
	length = binary.BigEndian.Uint32(buf)

	*a = make([]T, 0, length)
	for i := uint32(0); i < length; i++ {
		var (
			curVal         = newDataTypeValue[T]()
			curPreChecksum byte
			curN           int
		)
		curN, curPreChecksum, err = curVal.Unmarshal(r)
		n += curN
		preChecksum += curPreChecksum
		if err != nil {
			return n, preChecksum, err
		}
		*a = append(*a, curVal)
	}
	return n, preChecksum, nil
}

func (a *Array[T]) Reset() {
	*a = make([]T, 0)
}

func preChecksumBytes(data []byte) byte {
	val := byte(0)
	for _, b := range data {
		val += b
	}
	return val
}
