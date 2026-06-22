package proto

import (
	"encoding/binary"
	"errors"
	"io"
	"reflect"
)

const (
	checksumMod = 256
)

var (
	ErrInvalidType = errors.New("invalid type")
)

type Marshaler interface {
	Marshal(w io.Writer) error
}

type Unmarshaler interface {
	Unmarshal(r io.Reader) error
}

type DataType interface {
	Marshaler
	Unmarshaler
	Set(val any) error
	Reset()
	PreChecksum() uint32
}

type U32 struct {
	val uint32
}

func (u *U32) PreChecksum() uint32 {
	return 4
}

func (u *U32) Set(val any) error {
	if v, ok := val.(uint32); ok {
		u.val = v
		return nil
	}
	return ErrInvalidType
}

func NewU32(val uint32) *U32 {
	return &U32{
		val: val,
	}
}

func (u *U32) Marshal(w io.Writer) error {
	return binary.Write(w, binary.BigEndian, u.val)
}

func (u *U32) Unmarshal(r io.Reader) error {
	return binary.Read(r, binary.BigEndian, &u.val)
}

func (u *U32) Reset() {
	var val uint32
	u.val = val
}

type String struct {
	val string
}

func (s *String) PreChecksum() uint32 {
	return 4 + uint32(len(s.val))%checksumMod
}

func (s *String) Set(val any) error {
	if v, ok := val.(string); ok {
		s.val = v
		return nil
	}
	return ErrInvalidType
}

func NewString(val string) *String {
	return &String{
		val: val,
	}
}

func (s *String) Marshal(w io.Writer) error {
	err := binary.Write(w, binary.BigEndian, uint32(len(s.val)))
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, s.val)
	if err != nil {
		return err
	}
	return nil
}

func (s *String) Unmarshal(r io.Reader) error {
	length := uint32(0)
	err := binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return err
	}
	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf) // todo check
	if err != nil {
		return err
	}
	s.val = string(buf)
	return nil
}

func (s *String) Reset() {
	var val string
	s.val = val
}

type Array[T DataType] struct {
	val []T
}

func (a *Array[T]) PreChecksum() uint32 {
	length := uint32(4) // length prefix
	for _, v := range a.val {
		length += v.PreChecksum()
		length %= checksumMod
	}
	return length
}

func (a *Array[T]) Set(val any) error {
	// todo check
	if v, ok := val.([]T); ok {
		a.val = v
		return nil
	}
	return ErrInvalidType
}

func newDataTypeValue[T DataType]() T {
	var zero T
	typ := reflect.TypeOf(zero)
	if typ != nil && typ.Kind() == reflect.Pointer {
		return reflect.New(typ.Elem()).Interface().(T)
	}
	return zero
}

func NewArrayWithData[T DataType](data ...T) *Array[T] {
	return &Array[T]{
		val: data,
	}
}

func (a *Array[T]) Elements() []T {
	return a.val
}

func (a *Array[T]) Marshal(w io.Writer) error {
	length := len(a.val)
	err := binary.Write(w, binary.BigEndian, uint32(length))
	if err != nil {
		return err
	}
	for _, v := range a.val {
		err = v.Marshal(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Array[T]) Unmarshal(r io.Reader) error {
	length := uint32(0)
	err := binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return err
	}
	a.val = make([]T, 0, length)
	for i := uint32(0); i < length; i++ {
		curVal := newDataTypeValue[T]()
		err = curVal.Unmarshal(r)
		if err != nil {
			return err
		}
		a.val = append(a.val, curVal)
	}
	return nil
}

func (a *Array[T]) Reset() {
	a.val = make([]T, 0)
}
