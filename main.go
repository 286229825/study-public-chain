package main

import (
	"fmt"
	"log"
	"net"
	"study-public-chain/blc"
)

func main() {
	//设置日志输出格式为 2019/01/19 17:35:18 message 的形式
	log.SetFlags(3)
	//根据命令行中的命令执行指定的操作
	cli := blc.CLI{}
	cli.Run()


}

func accept(){

}

func doServerStuff(conn net.Conn) {
	for {
		buf := make([]byte, 512)
		_, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Error reading", err.Error())
			return //终止程序
		}
		fmt.Printf("Received data: %v", string(buf))
	}
}