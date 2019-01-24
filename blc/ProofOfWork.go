package blc

import (
	"bytes"
	"crypto/sha256"
	"log"
	"math/big"
)

//难度系数，表示生成的256位的哈希值的前面至少要有多少个零。
//这里的 targetBits 为16，表示生成的256位的哈希值的前面至少要有16个零。
const targetBits = 26

//工作量证明结构
type proofOfWork struct {
	//当前要验证的区块
	b *Block
	//目标值
	target *big.Int
}

//将区块中的所有属性拼接成字节数组
func (pow *proofOfWork) prepareData(nonce int64) []byte {
	return bytes.Join(
		[][]byte{
			IntToBytes(pow.b.Height),
			pow.b.PrevBlockHash,
			pow.b.hashTransactions(),
			IntToBytes(pow.b.Timestamp),
			pow.b.Hash,
			IntToBytes(nonce),
		},
		[]byte{},
	)
}

//判断当前哈希值是否有效
func (pow *proofOfWork) IsValid() bool {
	var hashInt big.Int
	hashInt.SetBytes(pow.b.Hash)
	if hashInt.Cmp(pow.target) == -1 { // 有效的条件：hashInt < pow.target
		return true
	}
	return false
}

//运行工作量证明，也就是挖矿
func (pow *proofOfWork) Run() ([]byte, int64) {
	//随机数从0开始
	var nonce int64 = 0
	var hash [32]byte
	var hashInt big.Int
	log.Println("start proofOfWork")
	for {
		//1、将区块的所有属性拼接成字节数组
		blockBytes := pow.prepareData(nonce)
		//2、生成哈希值
		hash = sha256.Sum256(blockBytes)
		hashInt.SetBytes(hash[:]) //将哈希值转为big.Int类型
		//3、判断哈希值的有效性，如果有效，则跳出循环
		if hashInt.Cmp(pow.target) == -1 { // 有效的条件：hashInt < pow.target
			break
		}
		//随机数每次循环都加1，直到找到符合 hashInt < pow.target 条件的随机数
		nonce++
	}
	log.Println("proofOfWork finished")
	log.Printf("find hash : %x\n", hash)   //将找到的哈希值以十六进制打印
	log.Printf("find nonce : %d\n", nonce) //将找到的随机数以十进制打印
	return hash[:], nonce
}

//创建新的工作量证明
func NewProofOfWork(b *Block) *proofOfWork {
	//1、创建一个初始值为1的target
	target := big.NewInt(1)
	//2、将target左移 256-targetBits 位
	target = target.Lsh(target, 256-targetBits)
	//3、创建工作量证明类型并返回
	return &proofOfWork{b, target}
}
