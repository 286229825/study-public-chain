package blc

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"log"
)

//交易结构
type transaction struct {
	//交易的哈希值
	TxHash []byte
	//输入
	TxInputs []*TxInput
	//输出
	TxOutputs []*TxOutput
}

//判断是否为创世交易
func (tx *transaction) isCoinbase() bool {
	if len(tx.TxInputs) == 1 && tx.TxInputs[0].Vout == -1 {
		return true
	}
	return false
}

//生成交易的哈希值
func (tx *transaction) hashTransaction() []byte {
	//将交易序列化
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(tx)
	if err != nil {
		log.Panic(err)
	}
	//将序列化后的交易字节数组进行哈希运算，生成哈希值
	hash := sha256.Sum256(result.Bytes())
	return hash[:]
}

//创建 Coinbase 交易。
//Coinbase 交易是矿工创建的，主要是为了奖励矿工为了进行 POW 挖矿而付出的努力。
//奖励分为两部分，
//一部分是出块奖励，这部分是相对固定的，当前每个区块的出块奖励是12.5BTC，每四年减半一次；
//另外一部分是手续费，当前区块的每个交易中都会包含一定的对矿工的奖励，也就是交易手续费。
//创建 Coinbase 交易的时候，矿工会把所有交易中的手续费累加到一起，然后把这笔钱转账给自己。
//Coinbase 交易的特点是没有“父交易”，
//普通交易中需要 input ，而 input 是来自父交易的 output ，所以普通交易是有父交易的，
//但是 Coinbase 交易是没有父交易的，因为币是直接由系统生成的。
func NewCoinbaseTransaction(address string) *transaction {
	//设置交易的输入输出
	txInput := &TxInput{[]byte{}, -1, "Genesis Data"}
	txOutput := &TxOutput{10, address}
	txCoinbase := &transaction{[]byte{}, []*TxInput{txInput}, []*TxOutput{txOutput}}
	//设置交易的哈希值
	txCoinbase.TxHash = txCoinbase.hashTransaction()
	return txCoinbase
}

//创建一个新的交易，可以有多个输入（就是同一个人可以引用的以前的多个输出）和多个输出，
//但是同一个交易只能向同一个人输出一次
//from：出钱的人，只能有一个
//tos：收钱的人，可以有多个
func NewTransaction(from string, tos map[string]float64, bc *blockChain) *transaction {
	var totalAmount float64 = 0
	for _, amount := range tos {
		totalAmount += amount
	}
	suitableUTXOs, total := bc.findSuitableUTXOs(from, totalAmount)
	if total < totalAmount {
		log.Fatal("余额不足，无法创建当前交易！")
	}
	var inputs []*TxInput
	var outputs []*TxOutput
	//创建输入
	for txHash, indexs := range suitableUTXOs {
		for _, index := range indexs {
			input := &TxInput{
				TXHash:    []byte(txHash),
				Vout:      index,
				ScriptSig: from,
			}
			inputs = append(inputs, input)
		}
	}
	//创建输出
	//给对方支付
	for to, amount := range tos {
		output := &TxOutput{amount, to}
		outputs = append(outputs, output)
	}
	//找零给自己
	if total > totalAmount {
		output := &TxOutput{total - totalAmount, from}
		outputs = append(outputs, output)
	}
	tx := &transaction{[]byte{}, inputs, outputs}
	tx.TxHash = tx.hashTransaction()
	return tx
}

//输入结构
type TxInput struct {
	//所引用TXOutput的交易哈希值
	TXHash []byte
	//所引用TXOutput的索引值
	Vout int64
	//数字签名，对应一个输出
	ScriptSig string
}

func (input *TxInput) isTheSameScriptSig(address string) bool {
	return address == input.ScriptSig
}

//输出结构
type TxOutput struct {
	//支付给收款方的金额
	Value float64
	//公钥
	ScriptPubKey string
}

//判断当前output的锁定脚本能否被当前地址的解锁脚本解锁
func (output *TxOutput) canBeUnlock(address string) bool {
	return address == output.ScriptPubKey
}
