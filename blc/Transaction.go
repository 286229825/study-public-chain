package blc

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"log"
	"math/big"
	"os"
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

//对交易进行数字签名
func (tx *transaction) Sign(privKey ecdsa.PrivateKey, prevTXs map[string]transaction) {
	if tx.isCoinbase() {
		return
	}

	txCopy := tx.TrimmedCopy()

	for inID, vin := range txCopy.TxInputs {
		prevTx := prevTXs[hex.EncodeToString(vin.TXHash)]
		txCopy.TxInputs[inID].Signature = nil
		txCopy.TxInputs[inID].PubKey = prevTx.TxOutputs[vin.Vout].Ripemd160Hash
		txCopy.TxHash = txCopy.Hash()
		txCopy.TxInputs[inID].PubKey = nil

		r, s, _ := ecdsa.Sign(rand.Reader, &privKey, txCopy.TxHash)
		signature := append(r.Bytes(), s.Bytes()...)

		tx.TxInputs[inID].Signature = signature
	}
}

//拷贝一份新的transaction用于数字签名
func (tx *transaction) TrimmedCopy() transaction {
	var inputs []*TxInput
	var outputs []*TxOutput

	for _, input := range tx.TxInputs {
		inputs = append(inputs, &TxInput{input.TXHash, input.Vout, nil, nil})
	}

	for _, output := range tx.TxOutputs {
		outputs = append(outputs, &TxOutput{output.Value, output.Ripemd160Hash})
	}

	txCopy := transaction{tx.TxHash, inputs, outputs}

	return txCopy
}

func (tx *transaction) Hash() []byte {
	txCopy := tx
	txCopy.TxHash = []byte{}
	hash := sha256.Sum256(txCopy.Serialize())
	return hash[:]
}

func (tx *transaction) Serialize() []byte {
	var encoded bytes.Buffer
	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}
	return encoded.Bytes()
}

//验证数字签名
func (tx *transaction) Verify(prevTXs map[string]transaction) bool {
	txCopy := tx.TrimmedCopy()
	curve := elliptic.P256()
	for i, input := range tx.TxInputs {
		prevTx := prevTXs[hex.EncodeToString(input.TXHash)]
		txCopy.TxInputs[i].Signature = nil
		txCopy.TxInputs[i].PubKey = prevTx.TxOutputs[input.Vout].Ripemd160Hash
		txCopy.TxHash = txCopy.Hash()
		txCopy.TxInputs[i].PubKey = nil

		r := big.Int{}
		s := big.Int{}
		sigLen := len(input.Signature)
		r.SetBytes(input.Signature[:(sigLen / 2)])
		s.SetBytes(input.Signature[(sigLen / 2):])

		x := big.Int{}
		y := big.Int{}
		keyLen := len(input.PubKey)
		x.SetBytes(input.PubKey[:(keyLen / 2)])
		y.SetBytes(input.PubKey[(keyLen / 2):])

		rawPubKey := ecdsa.PublicKey{curve, &x, &y}
		if ecdsa.Verify(&rawPubKey, txCopy.TxHash, &r, &s) == false {
			return false
		}
	}
	return true
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
	txInput := &TxInput{[]byte{}, -1, nil, []byte{}}
	txOutput := NewTXOutput(10, address)
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
	nodeId := os.Getenv("NODE_ID")
	if nodeId == "" {
		log.Fatal("无法获取NODE_ID的环境变量")
	}
	wallets, err := getAllWallets(nodeId)
	if err != nil {
		log.Panic(err)
	}
	wallet := wallets[from]
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
	for txHashStr, index := range suitableUTXOs {
		txHash, err := hex.DecodeString(txHashStr)
		if err != nil {
			log.Panic(err)
		}
		input := &TxInput{
			TXHash:    []byte(txHash),
			Vout:      index,
			Signature: nil,
			PubKey:    wallet.PublicKey,
		}
		inputs = append(inputs, input)
	}
	//创建输出
	//给对方支付
	for to, amount := range tos {
		output := NewTXOutput(amount, to)
		outputs = append(outputs, output)
	}
	//找零给自己
	if total > totalAmount {
		output := NewTXOutput(total-totalAmount, from)
		outputs = append(outputs, output)
	}
	tx := &transaction{[]byte{}, inputs, outputs}
	tx.TxHash = tx.hashTransaction()
	//进行数字签名。签名的作用在于当A转账给B的时候，A只能花费属于他自己的钱来转给B
	bc.SignTransaction(tx, wallet.PrivateKey)
	return tx
}

//输入结构
type TxInput struct {
	//所引用TXOutput的交易哈希值
	TXHash []byte
	//所引用TXOutput的索引值
	Vout int64
	//数字签名
	Signature []byte
	//原生的公钥
	PubKey []byte
}

func (input *TxInput) UnlockRipemd160Hash(Ripemd160Hash []byte) bool {
	hashPubKey := HashPubKey(input.PubKey)
	return bytes.Compare(hashPubKey, Ripemd160Hash) == 0
}

//输出结构
type TxOutput struct {
	//支付给收款方的金额
	Value float64
	//经过一次256哈希，再经过一次160哈希之后的收款方的公钥，用于锁定当前输出中的钱只属于收款方的
	Ripemd160Hash []byte
}

//设置Ripemd160Hash
func (output *TxOutput) Lock(address string) {
	publicKeyHash := Base58Decode([]byte(address))
	output.Ripemd160Hash = publicKeyHash[1 : len(publicKeyHash)-4]
}

//判断当前output的锁定脚本能否被当前地址的解锁脚本解锁
func (output *TxOutput) UnLockScriptPubKeyWithAddress(address string) bool {
	publicKeyHash := Base58Decode([]byte(address))
	hash160 := publicKeyHash[1 : len(publicKeyHash)-4]
	return bytes.Compare(hash160, output.Ripemd160Hash) == 0
}

//创建输入
func NewTXOutput(value float64, address string) *TxOutput {
	txOutput := &TxOutput{value, nil}
	//设置Ripemd160Hash
	txOutput.Lock(address)
	return txOutput
}
