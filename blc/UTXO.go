package blc

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/boltdb/bolt"
	"log"
)

//UTXO池所在的数据库表
const utxoTableName = "utxos"

//未话费的输出
type UTXO struct {
	Output *TxOutput
	Vout   int64
}

//UTXO池
type UTXOSet struct {
	bc *blockChain
}

//重置UTXO池
func (utxoSet *UTXOSet) ResetUTXOSet() {
	err := utxoSet.bc.Db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(utxoTableName))
		if bucket != nil {
			tx.DeleteBucket([]byte(utxoTableName))
		}
		bucket, err := tx.CreateBucket([]byte(utxoTableName))
		if err != nil {
			return err
		}
		outputsMap := utxoSet.bc.FindUTXOs()
		for txHashStr, outputs := range outputsMap {
			txHash, err := hex.DecodeString(txHashStr)
			if err != nil {
				return err
			}
			utxos, err := json.Marshal(outputs)
			if err != nil {
				return err
			}
			err = bucket.Put(txHash, utxos)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

//更新UTXO池
func (utxoSet *UTXOSet) UpdateUTXOSet(txs []*transaction) bool {
	if txs != nil {
		err := utxoSet.bc.Db.Update(func(boltTx *bolt.Tx) error {
			bucket := boltTx.Bucket([]byte(utxoTableName))
			if bucket != nil {
				for _, tx := range txs {
					inputs := tx.TxInputs
					for _, input := range inputs {
						utxosBytes := bucket.Get(input.TXHash)
						var utxos []UTXO
						json.Unmarshal(utxosBytes, &utxos)
						for i, utxo := range utxos {
							if input.Vout == utxo.Vout {
								utxos = append(utxos[:i], utxos[i+1:]...)
								utxosBytes, err := json.Marshal(utxos)
								if err != nil {
									return err
								}
								err = bucket.Put(input.TXHash, utxosBytes)
								if err != nil {
									return err
								}
								break
							}
						}
					}
					txHash := tx.TxHash
					outputs := tx.TxOutputs
					var utxos []UTXO
					for i, output := range outputs {
						utxo := UTXO{output, int64(i)}
						utxos = append(utxos, utxo)
					}
					utxosBytes, err := json.Marshal(utxos)
					if err != nil {
						return err
					}
					err = bucket.Put(txHash, utxosBytes)
					if err != nil {
						return err
					}
				}
				return nil
			}
			return errors.New("UTXOSet数据不存在")
		})
		if err != nil {
			log.Panic(err)
		}
	}
	return true
}
