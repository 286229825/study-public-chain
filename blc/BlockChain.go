package blc

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"log"
	"math/big"
	"os"
	"time"
)

//数据库名
const dbName = "blockchain.db"

//数据库中的表名
const tableName = "blocks"

//最新的区块的哈希值存在数据库中的键
const lastHashKey = "L"

//区块链结构
type blockChain struct {
	//最新的区块的哈希值
	Tip []byte
	//bolt数据库对象，该数据库中存储了区块链中所有的区块
	Db *bolt.DB
}

//判断当前区块链的数据库是否存在
func dbExist() bool {
	if _, err := os.Stat(dbName); os.IsNotExist(err) {
		return false
	}
	return true
}

//创建区块链
func CreateBlockChain(address string) *blockChain {
	//判断当前区块链的数据库是否存在
	if dbExist() {
		log.Fatal("当前区块链已存在，不能重复创建")
	}
	//打开或创建数据库
	db, err := bolt.Open(dbName, 0600, nil)
	if err != nil {
		log.Panic(err)
	}
	//创世块的哈希值
	var hash []byte
	err = db.Update(func(tx *bolt.Tx) error {
		//创建数据库表
		bucket, err := tx.CreateBucket([]byte(tableName))
		if err != nil {
			return err
		}
		if bucket != nil {
			//创建coinbase transaction
			coinbaseTx := NewCoinbaseTransaction(address)
			//创建创世块
			genesisBlock := NewGenesisBlock([]*transaction{coinbaseTx})
			hash = genesisBlock.Hash
			//将创世块存储到数据库中
			err := bucket.Put(hash, genesisBlock.Serialize())
			if err != nil {
				return err
			}
			err = bucket.Put([]byte(lastHashKey), hash)
			if err != nil {
				return err
			}
			return nil
		}
		return errors.New("数据库表创建失败")
	})
	if err != nil {
		log.Panic(err)
	}
	//创建区块链类型，其中的最新的区块的哈希值为创世块的哈希值
	bc := &blockChain{hash, db}
	//重置UTXO池
	utxoSet := UTXOSet{bc}
	utxoSet.ResetUTXOSet()
	//返回区块链类型
	return bc
}

//从数据库中获取区块链
func GetBlockChain() *blockChain {
	if !dbExist() {
		log.Fatal("当前区块链不存在，请先创建！")
	}
	db, err := bolt.Open(dbName, 0600, nil)
	if err != nil {
		log.Panic(err)
	}
	var lastHash []byte
	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(tableName))
		if bucket != nil {
			lastHash = bucket.Get([]byte(lastHashKey))
			return nil
		}
		return errors.New("当前数据库表不存在，可能是因为区块链未创建")
	})
	if err != nil {
		log.Panic(err)
	}
	return &blockChain{lastHash, db}
}

//区块链迭代器结构
type BlockChainIterator struct {
	currHash []byte
	db       *bolt.DB
}

//创建区块链迭代器
func (bc *blockChain) Iterator() *BlockChainIterator {
	bci := BlockChainIterator{currHash: bc.Tip, db: bc.Db}
	return &bci
}

//迭代区块链，返回区块链中下一个区块，从第一个区块开始返回
func (bci *BlockChainIterator) Next() *Block {
	var blockBytes []byte
	err := bci.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(tableName))
		if bucket != nil {
			blockBytes = bucket.Get(bci.currHash)
			return nil
		}
		return errors.New("当前数据库表不存在，可能是因为区块链未创建")
	})
	if err != nil {
		log.Panic(err)
	}
	b := Deserialize(blockBytes)
	bci.currHash = b.PrevBlockHash
	return b
}

//打印区块链
func (bc *blockChain) PrintChain() {
	//全局的当前区块
	var b *Block
	//全局的当前区块的哈希值的int形式
	var hashInt big.Int
	//创建当前区块链的迭代器
	iterator := bc.Iterator()
	for {
		//迭代出区块链中的区块
		b = iterator.Next()
		if b != nil {
			//打印当前区块
			fmt.Printf("Height:%d\n", b.Height)
			fmt.Printf("PrevBlockHash:%x\n", b.PrevBlockHash)
			fmt.Printf("Transactions:%v\n", b.Txs)
			fmt.Printf("Timestamp:%s\n", time.Unix(b.Timestamp, 0).Format("2006-01-02 15:04:05"))
			fmt.Printf("Hash:%x\n", b.Hash)
			fmt.Printf("Nonce:%d\n", b.Nonce)
			fmt.Println("===============================================================")
		}
		//如果当前区块的前一个区块的哈希值为0，则认为当前区块已经是创世区块了，跳出循环
		hashInt.SetBytes(b.PrevBlockHash)
		if hashInt.Cmp(big.NewInt(0)) == 0 {
			break
		}
	}
}

//向区块链中添加新的区块
func (bc *blockChain) AddBlock(address string, txs []*transaction) {
	//判断当前区块链的数据库是否存在
	exist := dbExist()
	if !exist {
		log.Println("当前区块链不存在，请先创建区块链")
		os.Exit(1)
	}
	//在添加新区块之前对txs进行签名验证
	for _, tx := range txs {
		if !bc.VerifyTransaction(tx) {
			log.Panic("签名验证失败")
		}
	}
	//挖矿奖励
	tx := NewCoinbaseTransaction(address)
	txs = append(txs, tx)
	//向区块链中添加新的区块
	err := bc.Db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(tableName))
		if bucket != nil {
			//获取区块链中最新的区块
			blockBytes := bucket.Get(bc.Tip)
			lastBlock := Deserialize(blockBytes)
			//创建新的区块
			b := NewBlock(lastBlock.Height+1, lastBlock.Hash, txs)
			//将创建的新区块添加到数据库中
			err := bucket.Put(b.Hash, b.Serialize())
			if err != nil {
				return err
			}
			err = bucket.Put([]byte(lastHashKey), b.Hash)
			if err != nil {
				return err
			}
			//更新区块链中最新的区块的哈希值
			bc.Tip = b.Hash
			return nil
		}
		return errors.New("当前数据库表不存在，可能是因为区块链未创建")
	})
	if err != nil {
		log.Panic(err)
	}
	//更新UTXO池
	utxoSet := UTXOSet{bc}
	utxoSet.UpdateUTXOSet(txs)
}

//找出当前用户所有可用的UTXO所在的交易数组
//func (bc *blockChain) findUTXOTransactions(address string) []*transaction {
//	publicKeyHash := Base58Decode([]byte(address))
//	ripemd160Hash := publicKeyHash[1 : len(publicKeyHash)-4]
//	iterator := bc.Iterator()
//	spentedOutputs := make(map[string]int64)
//	var unSpentedTx []*transaction
//	var hashInt big.Int
//	for {
//		b := iterator.Next()
//		for _, tx := range b.Txs {
//			if tx != nil && !tx.isCoinbase() {
//				for _, input := range tx.TxInputs {
//					if input.UnlockRipemd160Hash(ripemd160Hash) {
//						spentedOutputs[string(input.TXHash)] = input.Vout
//					}
//				}
//			}
//			for _, output := range tx.TxOutputs {
//				if output.UnLockScriptPubKeyWithAddress(address) {
//					if _, isPresent := spentedOutputs[string(tx.TxHash)]; !isPresent {
//						unSpentedTx = append(unSpentedTx, tx)
//						break
//					}
//				}
//			}
//		}
//		hashInt.SetBytes(b.PrevBlockHash)
//		//如果当前区块的前一个区块的哈希值为0，则认为当前区块已经是创世区块了，跳出循环
//		if hashInt.Cmp(big.NewInt(0)) == 0 {
//			break
//		}
//	}
//	return unSpentedTx
//}

//找出适用于当前交易的UTXO
func (bc *blockChain) findSuitableUTXOs(from string, amount float64) (map[string]int64, float64) {
	//	transactions := bc.findUTXOTransactions(from)
	//	suitableUTXOs := make(map[string][]int64)
	//	var total float64 = 0
	//LABEL1:
	//	for _, tx := range transactions {
	//		for index, output := range tx.TxOutputs {
	//			if output.UnLockScriptPubKeyWithAddress(from) {
	//				//判断当前搜集的UTXO的总金额是否大于所需要花费的金额
	//				if total < amount {
	//					suitableUTXOs[string(tx.TxHash)] = append(suitableUTXOs[string(tx.TxHash)], int64(index))
	//					total += output.Value
	//				} else {
	//					break LABEL1
	//				}
	//			}
	//		}
	//	}
	//	return suitableUTXOs, total
	suitableUTXOs := make(map[string]int64)
	var total float64 = 0
	utxoMap := bc.FindUTXOAndTxHashForAddress(from)
	for txHashStr, utxo := range utxoMap {
		if total < amount {
			suitableUTXOs[txHashStr] = utxo.Vout
			total += utxo.Output.Value
		} else {
			break
		}
	}
	return suitableUTXOs, total
}

func (bc *blockChain) GetBalance(address string) float64 {
	//txs := bc.findUTXOTransactions(address)
	//var total float64 = 0
	//for _, tx := range txs {
	//	for _, output := range tx.TxOutputs {
	//		if output.UnLockScriptPubKeyWithAddress(address) {
	//			total += output.Value
	//		}
	//	}
	//}
	//return total
	utxos := bc.FindUTXOForAddress(address)
	var total float64 = 0
	for _, utxo := range utxos {
		total += utxo.Output.Value
	}
	return total
}

//对一个transaction中的所有input进行数字签名
func (bc *blockChain) SignTransaction(tx *transaction, privateKey ecdsa.PrivateKey) {
	//如果是coinbase交易，则不需要进行数字签名
	if tx.isCoinbase() {
		return
	}
	//交易的哈希值与交易的映射
	preTXs := make(map[string]transaction)
	//找到当前交易中的所有input所对应的交易
	for _, input := range tx.TxInputs {
		//通过input中的交易的哈希找到所对应的交易
		preTX, err := bc.FindTransaction(input.TXHash)
		if err != nil {
			log.Panic(err)
		}
		//先将交易的哈希值由字节数组编码为字符串，然后将交易的哈希值（字符的形式）与交易进行映射
		preTXs[hex.EncodeToString(preTX.TxHash)] = preTX
	}
	//进行数字签名
	tx.Sign(privateKey, preTXs)
}

//根据交易的哈希值找出当前交易
func (bc *blockChain) FindTransaction(txHash []byte) (transaction, error) {
	var hashInt big.Int
	iterator := bc.Iterator()
	for {
		b := iterator.Next()
		for _, tx := range b.Txs {
			if bytes.Compare(tx.TxHash, txHash) == 0 {
				return *tx, nil
			}
		}
		hashInt.SetBytes(b.PrevBlockHash)
		//如果当前区块的前一个区块的哈希值为0，则认为当前区块已经是创世区块了，跳出循环
		if hashInt.Cmp(big.NewInt(0)) == 0 {
			break
		}
	}
	return transaction{}, errors.New("交易不存在")
}

//验证交易的数字签名
func (bc *blockChain) VerifyTransaction(tx *transaction) bool {
	//交易的哈希值与交易的映射
	preTXs := make(map[string]transaction)
	//找到当前交易中的所有input所对应的交易
	for _, input := range tx.TxInputs {
		//通过input中的交易的哈希找到所对应的交易
		preTX, err := bc.FindTransaction(input.TXHash)
		if err != nil {
			log.Panic(err)
		}
		//先将交易的哈希值由字节数组编码为字符串，然后将交易的哈希值（字符的形式）与交易进行映射
		preTXs[hex.EncodeToString(preTX.TxHash)] = preTX
	}
	//数字签名验证
	return tx.Verify(preTXs)
}

//找出所有未花费的输出
func (bc *blockChain) FindUTXOs() map[string][]UTXO {
	iterator := bc.Iterator()
	spentedOutputs := make(map[string][]int64)
	unSpentedOutputs := make(map[string][]UTXO)
	var hashInt big.Int
	for {
		b := iterator.Next()
		for _, tx := range b.Txs {
			if tx != nil && !tx.isCoinbase() {
				for _, input := range tx.TxInputs {
					txHashStr := hex.EncodeToString(input.TXHash)
					vouts := spentedOutputs[txHashStr]
					vouts = append(vouts, input.Vout)
					spentedOutputs[txHashStr] = vouts
				}
			}
			for index, output := range tx.TxOutputs {
				txHashStr := hex.EncodeToString(tx.TxHash)
				if _, isPresent := spentedOutputs[txHashStr]; !isPresent {
					utxo := UTXO{output, int64(index)}
					utxos := unSpentedOutputs[txHashStr]
					utxos = append(utxos, utxo)
					unSpentedOutputs[txHashStr] = utxos
				}
			}
		}
		hashInt.SetBytes(b.PrevBlockHash)
		//如果当前区块的前一个区块的哈希值为0，则认为当前区块已经是创世区块了，跳出循环
		if hashInt.Cmp(big.NewInt(0)) == 0 {
			break
		}
	}
	return unSpentedOutputs
}

//查找某个地址所对应的所有UTXO
func (bc *blockChain) FindUTXOForAddress(address string) []UTXO {
	var result []UTXO
	err := bc.Db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(utxoTableName))
		if bucket != nil {
			//游标
			c := bucket.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				var utxos []UTXO
				json.Unmarshal(v, &utxos)
				for _, utxo := range utxos {
					if utxo.Output.UnLockScriptPubKeyWithAddress(address) {
						result = append(result, utxo)
					}
				}
			}
			return nil
		}
		return errors.New("UTXOSet数据不存在")
	})
	if err != nil {
		log.Panic(err)
	}
	return result
}

//查找某个地址所对应的所有UTXO及其对应的交易哈希
func (bc *blockChain) FindUTXOAndTxHashForAddress(address string) map[string]UTXO {
	result := make(map[string]UTXO)
	err := bc.Db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(utxoTableName))
		if bucket != nil {
			//游标
			c := bucket.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				var utxos []UTXO
				json.Unmarshal(v, &utxos)
				txHashStr := hex.EncodeToString(k)
				for _, utxo := range utxos {
					if utxo.Output.UnLockScriptPubKeyWithAddress(address) {
						result[txHashStr] = utxo
						break
					}
				}
			}
			return nil
		}
		return errors.New("UTXOSet数据不存在")
	})
	if err != nil {
		log.Panic(err)
	}
	return result
}
