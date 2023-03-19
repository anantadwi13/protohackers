package main

import (
	"bytes"
	"encoding/binary"
)

func bytesToInt32(data []byte) (int32, error) {
	var (
		val int32
		buf = bytes.NewBuffer(data)
	)

	err := binary.Read(buf, binary.BigEndian, &val)
	if err != nil {
		return 0, err
	}
	return val, nil
}

func int32ToBytes(data int32) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
