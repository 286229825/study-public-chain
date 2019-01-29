package blc

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"golang.org/x/crypto/ripemd160"
	"io/ioutil"
	"log"
	"os"
)

//第一步：创建钱包，生成私钥和公钥
//第二步：先将公钥进行一次256哈希，再进行一次160哈希，生成一个20字节的字节数组
//第三步：将版本号的1个字节和第二步得到的20字节的字节数组相加，生成一个21字节的字节数组
//第四步：将第三步得到的21字节的字节数组进行两次256哈希，生成一个32字节的字节数组
//第五步：将第四步得到的32字节的字节数组中的前面4个字节（也就是地址校验所需的长度）取出来，并添加到第三步得到的21字节的末尾，生成一个25字节的字节数组
//第六步：将第五步得到的25字节的字节数组进行base58编码，得到钱包地址

//版本号，占两个十六进制位，也就是一个字节
const version = byte(0X00)

//地址校验时需要用到的 checksum 算法中的字节长度，固定为4个字节
const addressChecksumLen = 4

//存储钱包数据的文件名称
const walletsFileName = "wallets.dat"

type Wallet struct {
	//私钥，类型为椭圆曲线数字签名算法的库中的私钥类型
	PrivateKey ecdsa.PrivateKey
	//由私钥生成的公钥
	PublicKey []byte
}

//创建钱包并保存到本地文件
func NewWallet() *Wallet {
	//创建钱包
	private, public := newKeyPair()
	wallet := &Wallet{private, public}
	//将创建的钱包保存到本地文件
	err := wallet.saveToFile()
	if err != nil {
		log.Panic(err)
	}
	return wallet
}

//获取所有钱包地址
func GetAllAddress() []string {
	wallets, err := GetAllWallets()
	if err != nil {
		log.Panic(err)
	}
	var addresses []string
	for k := range wallets {
		addresses = append(addresses, k)
	}
	return addresses
}

//从本地文件中获取所有已经创建的钱包
func GetAllWallets() (map[string]*Wallet, error) {
	//定义钱包数据集合，key为钱包地址的字符串，value为钱包
	var wallets map[string]*Wallet
	//校验钱包数据所在的文件是否存在
	if _, err := os.Stat(walletsFileName); os.IsNotExist(err) { //如果钱包数据所在的文件不存在，则初始化钱包数据集合
		wallets = make(map[string]*Wallet)
	} else { //如果钱包数据所在的文件已经存在，则读出文件中的数据并赋值给钱包数据集合
		fileContent, err := ioutil.ReadFile(walletsFileName)
		if err != nil {
			return nil, err
		}
		//注册目的在于：可以序列化任何类型
		gob.Register(elliptic.P256())
		decoder := gob.NewDecoder(bytes.NewReader(fileContent))
		err = decoder.Decode(&wallets)
		if err != nil {
			return nil, err
		}
	}
	return wallets, nil
}

//将当前钱包存储到本地文件
func (wallet *Wallet) saveToFile() error {
	wallets, err := GetAllWallets()
	if err != nil {
		return err
	}
	wallets[string(wallet.GetAddress())] = wallet
	var content bytes.Buffer
	gob.Register(elliptic.P256())
	encoder := gob.NewEncoder(&content)
	err = encoder.Encode(&wallets)
	if err != nil {
		return err
	}
	//将序列化以后的数据写入到文件中，原来文件的数据会被覆盖
	err = ioutil.WriteFile(walletsFileName, content.Bytes(), 0644)
	if err != nil {
		return err
	}
	return nil
}

//返回钱包地址
func (wallet *Wallet) GetAddress() (address []byte) {
	//第一步：先对公钥进行哈希运算，先进行一次256哈希，再进行一次160哈希，生成一个20字节的字节数组
	pubKeyHash := HashPubKey(wallet.PublicKey)
	//第二步：再将版本号的1个字节和第一步得到的20字节的字节数组相加，生成一个21字节的字节数组
	versionedPayload := append([]byte{version}, pubKeyHash...)
	//第三步：再将第二步得到的21字节的字节数组进行两次256哈希，并将生成的32字节的字节数组中的前面4个字节取出来
	checksum := checksum(versionedPayload)
	//第四步：再将第二步得到的21字节加上第三步得到的4个字节,生成一个25字节的字节数组
	fullPayload := append(versionedPayload, checksum...)
	//第五步：再将第四步得到的25字节的字节数组进行base58编码，得到钱包地址并返回
	return Base58Encode(fullPayload)
}

//创建私钥和公钥
func newKeyPair() (ecdsa.PrivateKey, []byte) {
	//用椭圆曲线的库生成曲线
	curve := elliptic.P256()
	//用椭圆曲线数字签名算法的库，结合曲线和随机数生成私钥
	privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		log.Panic(err)
	}
	//用私钥生成公钥
	publicKey := append(privateKey.PublicKey.X.Bytes(), privateKey.PublicKey.Y.Bytes()...)
	return *privateKey, publicKey
}

//对公钥进行哈希运算
func HashPubKey(pubKey []byte) []byte {
	//先进行一次256哈希
	SHA256Hasher := sha256.New()
	_, err := SHA256Hasher.Write(pubKey)
	if err != nil {
		log.Panic(err)
	}
	publicSHA256 := SHA256Hasher.Sum(nil)
	//再进行一次160哈希
	ripemd160.New()
	RIPEMD160Hasher := ripemd160.New()
	_, err = RIPEMD160Hasher.Write(publicSHA256[:])
	if err != nil {
		log.Panic(err)
	}
	return RIPEMD160Hasher.Sum(nil)
}

//校验地址的有效性
func ValidateAddress(address string) bool {
	//第一步：将地址由字符串转为字节数组，并进行base58解码得到一个25字节的字节数组
	pubKeyHash := Base58Decode([]byte(address))
	//第二步：将第一步得到的25字节的字节数组中的最后四个字节取出来，得到当前地址的checksum算法中返回的四个字节
	actualChecksum := pubKeyHash[len(pubKeyHash)-addressChecksumLen:]
	//第三步：将第一步得到的25字节的字节数组中的第一个字节取出来得到当前地址中的版本号
	version := pubKeyHash[0]
	//第四步：将第一步得到的25字节的字节数组中的第一个字节（也就是版本号）和最后四个字节去掉，得到一个20字节的字节数组
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-addressChecksumLen]
	//第五步：将目标版本号加上第四步得到的20字节的字节数组，并进行checksum算法，得到目标checksum算法中返回的四个字节
	targetChecksum := checksum(append([]byte{version}, pubKeyHash...))
	//第六步：目标checksum算法中返回的四个字节和当前地址的checksum算法中返回的四个字节相等，则认为当前地址是合法的
	return bytes.Compare(actualChecksum, targetChecksum) == 0
}

//将字节数组进行两次256哈希，并将生成的32字节的字节数组中的前面4个字节取出来并返回
func checksum(payload []byte) []byte {
	firstSHA := sha256.Sum256(payload)
	secondSHA := sha256.Sum256(firstSHA[:])
	return secondSHA[:addressChecksumLen]
}
