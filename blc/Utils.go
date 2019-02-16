package blc

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"log"
	"reflect"
)

//将int64类型直接转换为字节数组
func IntToBytes(num int64) []byte {
	buff := bytes.NewBuffer([]byte{})
	err := binary.Write(buff, binary.BigEndian, &num)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

func BytesToInt(b []byte) int64 {
	var result int64
	buffer := bytes.NewBuffer(b)
	err := binary.Read(buffer, binary.BigEndian, &result)
	if err != nil {
		log.Panic(err)
	}
	return result
}

//将命令字符串转为12个字节的字节数组，若不满12个字节，则将剩下的字节为空
func CommandToBytes(command string) []byte {
	//创建一个 12 字节的缓冲区
	var bytes [COMMANDLENGTH]byte
	for i, c := range command { //c的类型为int32
		t := reflect.TypeOf(c) //
		log.Println(t)
		bytes[i] = byte(c)
	}
	return bytes[:] //返回的为12个字节的数组，若当前字符串长度小于12，那么其余的字节皆为空
}

//字节数组转命令字符串
func BytesToCommand(bytes []byte) string {
	var command []byte
	for _, b := range bytes { //b的类型为uint8
		command = append(command, b)
	}
	return fmt.Sprintf("%s", command)
}

// 将结构体序列化成字节数组
func GobEncode(data interface{}) []byte {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}
