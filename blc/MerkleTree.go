package blc

import "crypto/sha256"

type MerkleTree struct {
	RootNode *MerkleNode
}

type MerkleNode struct {
	Left  *MerkleNode
	Right *MerkleNode
	Data  []byte
}

//创建梅克尔树
func NewMerkleTree(datas [][]byte) *MerkleTree {
	var nodes []MerkleNode
	//如果交易的字节数据数组长度为奇数，则拷贝最后一个交易的字节数据
	if len(datas)%2 != 0 {
		datas = append(datas, datas[len(datas)-1])
	}
	//创建叶子节点
	for _, data := range datas {
		node := NewMerkleNode(nil, nil, data)
		nodes = append(nodes, *node)
	}
	for i := 0; i < len(datas)/2; i++ {
		var newNodes []MerkleNode
		for j := 0; j < len(nodes); j += 2 {
			node := NewMerkleNode(&nodes[j], &nodes[j+1], nil)
			newNodes = append(newNodes, *node)
		}
		if len(newNodes)%2 != 0 {
			newNodes = append(newNodes, newNodes[len(newNodes)-1])
		}
		nodes = newNodes
	}
	return &MerkleTree{&nodes[0]}
}

//创建梅克尔树的节点
func NewMerkleNode(left, right *MerkleNode, data []byte) *MerkleNode {
	merkleNode := MerkleNode{}
	if left == nil && right == nil { //创建叶子节点
		hash := sha256.Sum256(data)
		merkleNode.Data = hash[:]
	} else { //创建非叶子节点
		preHashes := append(left.Data, right.Data...)
		hash := sha256.Sum256(preHashes)
		merkleNode.Data = hash[:]
	}
	merkleNode.Left = left
	merkleNode.Right = right
	return &merkleNode
}
