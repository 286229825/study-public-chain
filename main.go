package main

import (
	"log"
	"study-public-chain/blc"
)

func main() {
	//设置日志输出格式为 2019/01/19 17:35:18 message 的形式
	log.SetFlags(3)
	//根据命令行中的命令执行指定的操作
	cli := blc.CLI{}
	cli.Run()
}
