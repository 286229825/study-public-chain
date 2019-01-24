package blc

import (
	"bytes"
	"encoding/binary"
	"log"
)

//将int64类型直接转换为字节数组
func IntToBytes(num int64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}
