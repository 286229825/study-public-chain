package blc

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

//命令使用说明
const usage = `
	createChain --address <ADDRESS>  				"创建区块链"
	send --from <FROM> --to <TO>  	"转账, 例如: send --from Tom --to Alice:10,Jack:12
	getBalance --address <ADDRESS>				"获取余额"
	printChain									"打印区块链信息"
	createWallet								"创建钱包"
	getAddressList								"获取所有钱包地址"
`

const createChain = "createChain"

const send = "send"

const getBalance = "getBalance"

const printChain = "printChain"

const createWallet = "createWallet"

const getAddressList = "getAddressList"

type CLI struct{}

func (cli *CLI) printUsage() {
	fmt.Print(usage)
	os.Exit(1)
}

func (cli *CLI) createChain(address string) {
	if !ValidateAddress(address) {
		log.Panic("收款人地址" + address + "无效")
	}
	bc := CreateBlockChain(address)
	defer bc.Db.Close()
	log.Println("区块链创建成功")
}

func (cli *CLI) send(sendCmdFromParam, sendCmdToParam string) {
	if !ValidateAddress(sendCmdFromParam) {
		log.Panic("汇款人地址" + sendCmdFromParam + "无效")
	}
	bc := GetBlockChain()
	defer bc.Db.Close()
	tos := make(map[string]float64)
	toArr := strings.Split(sendCmdToParam, ",")
	for _, value := range toArr {
		arr := strings.Split(value, ":")
		if len(arr) != 2 {
			log.Println("命令错误，请查看以下命令说明")
			cli.printUsage()
			return
		} else {
			to := arr[0]
			if !ValidateAddress(to) {
				log.Panic("收款人地址" + to + "无效")
			}
			amount, err := strconv.ParseFloat(arr[1], 64)
			if err != nil {
				log.Println("命令错误，金额不是float类型，请查看以下命令说明")
				cli.printUsage()
				return
			}
			tos[arr[0]] = amount
		}
	}
	tx := NewTransaction(sendCmdFromParam, tos, bc)
	//每一次交易都会打包一个区块，这是不对的，应该是将一定的时间内的所有交易一起打包成一个区块，以后会进行完善
	bc.AddBlock([]*transaction{tx})
	log.Println("交易创建成功")
}

func (cli *CLI) printChain() {
	bc := GetBlockChain()
	defer bc.Db.Close()
	bc.PrintChain()
}

func (cli *CLI) getBalance(address string) {
	if !ValidateAddress(address) {
		log.Panic("余额地址" + address + "无效")
	}
	bc := GetBlockChain()
	defer bc.Db.Close()
	balance := bc.GetBalance(address)
	log.Printf("%s的余额为：%f", address, balance)
}

func (cli *CLI) createWallet() {
	wallet := NewWallet()
	address := wallet.GetAddress()
	log.Printf("钱包创建成功，地址为：%s", string(address))
}

func (cli *CLI) getAddressList() {
	allAddress := GetAllAddress()
	for i, address := range allAddress {
		log.Printf("第%d个地址为：%s\n", i+1, address)
	}
}

func (cli *CLI) paramsCheck() {
	if len(os.Args) < 2 {
		fmt.Println("invalid input")
		cli.printUsage()
		os.Exit(1)
	}
}

func (cli *CLI) Run() {
	cli.paramsCheck()
	//命令解析器
	createChainCmd := flag.NewFlagSet(createChain, flag.ExitOnError)
	sendCmd := flag.NewFlagSet(send, flag.ExitOnError)
	getBalanceCmd := flag.NewFlagSet(getBalance, flag.ExitOnError)
	printChainCmd := flag.NewFlagSet(printChain, flag.ExitOnError)
	//获取命令中的参数值（以 -- 开头的参数的值）
	createChainCmdParam := createChainCmd.String("address", "", "address info")
	sendCmdFromParam := sendCmd.String("from", "", "source address info")
	sendCmdToParam := sendCmd.String("to", "", "target address info")
	getBalanceCmdParam := getBalanceCmd.String("address", "", "address info")
	//筛选命令中的第2个参数
	switch os.Args[1] {
	case createChain:
		//用createChain解析器去转换输入命令中的第3个参数以后的字符
		err := createChainCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
		if createChainCmd.Parsed() {
			if *createChainCmdParam == "" {
				log.Println("命令错误，请查看以下命令说明")
				cli.printUsage()
				return
			}
			//若命令校验成功，则调用相应方法
			cli.createChain(*createChainCmdParam)
		}
	case send:
		//用sendCmd解析器去转换输入命令中的第3个参数以后的字符
		err := sendCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
		if sendCmd.Parsed() {
			if *sendCmdFromParam == "" || *sendCmdToParam == "" {
				log.Println("命令错误，请查看以下命令说明")
				cli.printUsage()
				return
			}
			cli.send(*sendCmdFromParam, *sendCmdToParam)
		}
	case printChain:
		err := printChainCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
		if printChainCmd.Parsed() {
			cli.printChain()
		}
	case getBalance:
		err := getBalanceCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
		if getBalanceCmd.Parsed() {
			if *getBalanceCmdParam == "" {
				log.Println("命令错误，请查看以下命令说明")
				cli.printUsage()
				return
			}
			//若命令校验成功，则调用相应方法
			cli.getBalance(*getBalanceCmdParam)
		}
	case createWallet:
		cli.createWallet()
	case getAddressList:
		cli.getAddressList()
	default:
		cli.printUsage()
	}
}
