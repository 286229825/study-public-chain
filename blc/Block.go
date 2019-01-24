package blc

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"log"
	"time"
)

//区块结构
type Block struct {
	//1、区块高度，也就是区块编号
	Height int64
	//2、上一个区块的哈希值。二进制表示为256位，十六进制表示为64位，字节表示为32字节
	PrevBlockHash []byte
	//3、存储的数据，这里是交易，也可以是其他的数据
	Txs []*transaction
	//4、时间戳
	Timestamp int64
	//5、当前区块的哈希值。二进制表示为256位，十六进制表示为64位，字节表示为32字节
	Hash []byte
	//6、随机数
	Nonce int64
}

//将交易数据转换为字节数组
func (b *Block) hashTransactions() []byte {
	var txHashes [][]byte
	//将每一个交易的hash拼成二维数组
	for _, tx := range b.Txs {
		txHashes = append(txHashes, tx.TxHash)
	}
	//对二维切片进行拼接，生成一维切片
	data := bytes.Join(txHashes, []byte{})
	hash := sha256.Sum256(data)
	return hash[:]
}

//将区块序列化成字节数组
func (b *Block) Serialize() []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(b)
	if err != nil {
		log.Panic(err)
	}
	return result.Bytes()
}

//将字节数组反序列化成区块
func Deserialize(blockBytes []byte) *Block {
	if len(blockBytes) == 0 {
		return nil
	}
	b := Block{}
	decoder := gob.NewDecoder(bytes.NewReader(blockBytes))
	err := decoder.Decode(&b)
	if err != nil {
		log.Panic(err)
	}
	return &b
}

//创建新区块
func NewBlock(height int64, prevBlockHash []byte, txs []*transaction) *Block {
	b := &Block{
		height,
		prevBlockHash,
		txs,
		time.Now().Unix(), //因为区块链中产生一个新区块的时间很长，在比特币系统中是平均每10分钟产生一个新区快，所以这里精确到秒是可行的。
		nil,
		0,
	}
	//调用工作量证明的方法，返回有效的哈希值和随机数值
	pow := NewProofOfWork(b)
	hash, nonce := pow.Run()
	//设置区块的哈希值和随机数值
	b.Hash = hash
	b.Nonce = nonce
	return b
}

//创建创世块
func NewGenesisBlock(txs []*transaction) *Block {
	//创世块的上一个区块的哈希值为0
	return NewBlock(0, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, txs)
}
