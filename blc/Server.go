package blc

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"github.com/boltdb/bolt"
	"io"
	"io/ioutil"
	"log"
	"net"
)

//第一个终端：端口为3000，主节点
//第二个终端：端口为3001，钱包节点
//第三个终端：端口为3002，矿工节点

var knowNodes = []string{"localhost:3000"}
var nodeAddress string //全局变量，节点地址
// 存储hash值
var transactionArray [][]byte
var minerAddress string
var memoryTxPool = make(map[string]*transaction)

type GetData struct {
	AddrFrom string
	Type     string
	Hash     []byte
}

type Version struct {
	Version    int64  //版本
	BestHeight int64  //当前节点区块的高度
	AddrFrom   string //当前节点的地址
}

type Inv struct {
	AddrFrom string   //自己的地址
	Type     string   //类型 block或tx
	Items    [][]byte //hash二维数组
}

type Tx struct {
	AddrFrom string
	Tx       *transaction
}

type BlockData struct {
	AddrFrom string
	Block    []byte
}

type GetBlocks struct {
	AddrFrom string
}

func startServer(nodeID string, minerAdd string) {
	// 当前节点的IP地址
	nodeAddress = fmt.Sprintf("localhost:%s", nodeID)
	minerAddress = minerAdd
	ln, err := net.Listen(PROTOCOL, nodeAddress)
	if err != nil {
		log.Panic(err)
	}
	defer ln.Close()
	bc := GetBlockChain(nodeID)
	defer bc.Db.Close()
	//第一个终端：端口为3000，主节点
	//第二个终端：端口为3001，钱包节点
	//第三个终端：端口为3002，矿工节点
	if nodeAddress != knowNodes[0] {
		//若当前节点是钱包节点或者矿工节点，则需要向主节点发送当前节点的Version数据信息
		sendVersion(knowNodes[0], bc)
	}
	for {
		// 接收客户端发送过来的数据
		// 收到的数据的格式是固定的，12字节+结构体字节数组
		conn, err := ln.Accept()
		if err != nil {
			log.Panic(err)
		}
		go handleConnection(conn, bc)
	}
}

func handleConnection(conn net.Conn, bc *blockChain) {
	//读取客户端发送过来的所有的数据
	request, err := ioutil.ReadAll(conn)
	if err != nil {
		log.Panic(err)
	}
	fmt.Printf("Receive a Message:%s\n", request[:COMMANDLENGTH])
	//version
	command := BytesToCommand(request[:COMMANDLENGTH])
	// 12字节 + 某个结构体序列化以后的字节数组
	switch command {
	case COMMAND_VERSION:
		handleVersion(request, bc)
	case COMMAND_ADDR:
		handleAddr(request, bc)
	case COMMAND_BLOCK:
		handleBlock(request, bc)
	case COMMAND_GETBLOCKS:
		handleGetblocks(request, bc)
	case COMMAND_GETDATA:
		handleGetData(request, bc)
	case COMMAND_INV:
		handleInv(request, bc)
	case COMMAND_TX:
		handleTx(request, bc)
	default:
		fmt.Println("Unknown command!")
	}
	conn.Close()
}

//判断当前节点地址是否为已知节点地址
func nodeIsKnown(addr string) bool {
	for _, node := range knowNodes {
		if node == addr {
			return true
		}
	}
	return false
}

func handleVersion(request []byte, bc *blockChain) {
	var buff bytes.Buffer
	var payload Version
	dataBytes := request[COMMANDLENGTH:]
	// 反序列化出Version结构体
	buff.Write(dataBytes)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	//获取当前节点中最长的区块链高度
	bestHeight := bc.GetBestHeight()
	//获取请求节点中最长的区块链高度
	foreignerBestHeight := payload.BestHeight
	if bestHeight > foreignerBestHeight { //如果当前节点中最长的区块链高度大于请求节点中最长的区块链高度，则向请求节点中发送当前节点的Version信息
		sendVersion(payload.AddrFrom, bc)
	} else if bestHeight < foreignerBestHeight { //如果当前节点中最长的区块链高度小于请求节点中最长的区块链高度，则去向请求节点获取区块
		sendGetBlocks(payload.AddrFrom)
	}
	//如果请求节点地址不在已知节点地址列表中，则添加到已知节点地址列表
	if !nodeIsKnown(payload.AddrFrom) {
		knowNodes = append(knowNodes, payload.AddrFrom)
	}
}

func handleAddr(request []byte, bc *blockChain) {

}

func handleGetblocks(request []byte, bc *blockChain) {
	var buff bytes.Buffer
	var payload GetBlocks
	dataBytes := request[COMMANDLENGTH:]
	//反序列化出GetBlocks
	buff.Write(dataBytes)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	blocks := bc.GetBlockHashes()
	//主节点将自己的所有的区块hash发送给钱包节点
	sendInv(payload.AddrFrom, BLOCK_TYPE, blocks)
}

func handleGetData(request []byte, bc *blockChain) {
	var buff bytes.Buffer
	var payload GetData
	dataBytes := request[COMMANDLENGTH:]
	// 反序列化
	buff.Write(dataBytes)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	if payload.Type == BLOCK_TYPE {
		block, err := bc.GetBlock([]byte(payload.Hash))
		if err != nil {
			return
		}
		sendBlock(payload.AddrFrom, block)
	}
	if payload.Type == TX_TYPE {
		tx := memoryTxPool[hex.EncodeToString(payload.Hash)]
		sendTx(payload.AddrFrom, tx)
	}
}

//接收新的区块
func handleBlock(request []byte, bc *blockChain) {
	var buff bytes.Buffer
	var payload BlockData
	dataBytes := request[COMMANDLENGTH:]
	//反序列化出Block
	buff.Write(dataBytes)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	blockBytes := payload.Block
	block := Deserialize(blockBytes)
	//将当前区块添加到区块链中
	bc.AddBlockToBlockchain(block)
	//更新UTXO池
	UTXOSet := &UTXOSet{bc}
	UTXOSet.UpdateUTXOSet(block.Txs)
	if len(transactionArray) > 0 {
		blockHash := transactionArray[0]
		sendGetData(payload.AddrFrom, "block", blockHash)
		transactionArray = transactionArray[1:]
	} else {
		//fmt.Println("数据库重置......")
		//UTXOSet := &UTXOSet{bc}
		//UTXOSet.ResetUTXOSet()
	}
}

func handleTx(request []byte, bc *blockChain) {
	var buff bytes.Buffer
	var payload Tx
	dataBytes := request[COMMANDLENGTH:]
	// 反序列化
	buff.Write(dataBytes)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	tx := payload.Tx
	memoryTxPool[hex.EncodeToString(tx.TxHash)] = tx
	// 说明主节点自己
	if nodeAddress == knowNodes[0] {
		// 给矿工节点发送交易hash
		for _, nodeAddr := range knowNodes {
			if nodeAddr != nodeAddress && nodeAddr != payload.AddrFrom {
				sendInv(nodeAddr, TX_TYPE, [][]byte{tx.TxHash})
			}
		}
	}
	// 矿工进行挖矿验证
	if len(minerAddress) > 0 {
		utxoSet := &UTXOSet{bc}
		txs := []*transaction{tx}
		//奖励
		coinbaseTx := NewCoinbaseTransaction(minerAddress)
		txs = append(txs, coinbaseTx)
		_txs := []*transaction{}
		//fmt.Println("开始进行数字签名验证.....")
		for _, tx := range txs {
			//fmt.Printf("开始第%d次验证...\n",index)
			// 作业，数字签名失败
			if bc.VerifyTransaction(tx) != true {
				log.Panic("ERROR: Invalid transaction")
			}
			//fmt.Printf("第%d次验证成功\n",index)
			_txs = append(_txs, tx)
		}
		//fmt.Println("数字签名验证成功.....")
		//1. 通过相关算法建立Transaction数组
		var block *Block
		bc.Db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(tableName))
			if b != nil {
				hash := b.Get([]byte("l"))
				blockBytes := b.Get(hash)
				block = Deserialize(blockBytes)
			}
			return nil
		})
		//2. 建立新的区块
		block = NewBlock(block.Height+1, block.Hash, txs)
		//将新区块存储到数据库
		bc.Db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(tableName))
			if b != nil {
				b.Put(block.Hash, block.Serialize())
				b.Put([]byte("l"), block.Hash)
				bc.Tip = block.Hash
			}
			return nil
		})
		utxoSet.UpdateUTXOSet(txs)
		sendBlock(knowNodes[0], block.Serialize())
	}
}

func handleInv(request []byte, bc *blockChain) {
	var buff bytes.Buffer
	var payload Inv
	dataBytes := request[COMMANDLENGTH:]
	// 反序列化出Inv
	buff.Write(dataBytes)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	if payload.Type == BLOCK_TYPE {
		blockHash := payload.Items[0]
		sendGetData(payload.AddrFrom, BLOCK_TYPE, blockHash)
		if len(payload.Items) >= 1 {
			transactionArray = payload.Items[1:]
		}
	}
	if payload.Type == TX_TYPE {
		txHash := payload.Items[0]
		if memoryTxPool[hex.EncodeToString(txHash)] == nil {
			sendGetData(payload.AddrFrom, TX_TYPE, txHash)
		}
	}
}

//向远程地址发送Version信息
func sendVersion(toAddress string, bc *blockChain) {
	bestHeight := bc.GetBestHeight()
	payload := GobEncode(Version{NODE_VERSION, bestHeight, nodeAddress})
	request := append(CommandToBytes(COMMAND_VERSION), payload...)
	sendData(toAddress, request)
}

//向远程地址发送获取区块请求
func sendGetBlocks(toAddress string) {
	payload := GobEncode(GetBlocks{nodeAddress})
	request := append(CommandToBytes(COMMAND_GETBLOCKS), payload...)
	sendData(toAddress, request)
}

//主节点将自己的所有的区块hash发送给钱包节点
func sendInv(toAddress string, kind string, hashes [][]byte) {
	payload := GobEncode(Inv{nodeAddress, kind, hashes})
	request := append(CommandToBytes(COMMAND_INV), payload...)
	sendData(toAddress, request)
}

//在接收到Block数据后，发送获取成功的响应
func sendGetData(toAddress string, kind string, blockHash []byte) {
	payload := GobEncode(GetData{nodeAddress, kind, blockHash})
	request := append(CommandToBytes(COMMAND_GETDATA), payload...)
	sendData(toAddress, request)
}

func sendBlock(toAddress string, block []byte) {
	payload := GobEncode(BlockData{nodeAddress, block})
	request := append(CommandToBytes(COMMAND_BLOCK), payload...)
	sendData(toAddress, request)
}

//向远程地址发送交易信息
func sendTx(toAddress string, tx *transaction) {
	payload := GobEncode(Tx{nodeAddress, tx})
	request := append(CommandToBytes(COMMAND_TX), payload...)
	sendData(toAddress, request)
}

//向远程地址发送数据
func sendData(to string, data []byte) {
	conn, err := net.Dial("tcp", to)
	if err != nil {
		panic("error")
	}
	defer conn.Close()
	// 附带要发送的数据
	_, err = io.Copy(conn, bytes.NewReader(data))
	if err != nil {
		log.Panic(err)
	}
}
