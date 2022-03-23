package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"main/levelDbTree"
)

func main()  {
	current()
}

func current()  {
	ldbPath := "../.ethereum-test/geth/chaindata"
	// ldbPath := "../.ethereum-testnet/goerli/geth/chaindata"
	ldb, err := rawdb.NewLevelDBDatabase(ldbPath, 0, 0, "", true)
	if err != nil {
		panic(err)
	}

	stateRootNode, _ := levelDbTree.GetLastestStateTree(ldb)
	
	storageRootNodes := make(chan common.Hash)
	size := make(chan int)
	total := 0
 	
	go levelDbTree.GetStorageRootNodes(ldb, stateRootNode, storageRootNodes)

	go levelDbTree.RunTreeSize(ldb, storageRootNodes, size)

	for s := range size {
		total += s
	}

	fmt.Printf("Size in byte :%v\n", total)
}

func test()  {
	// ldbPath := "../.ethereum-test/geth/chaindata"
	ldbPath := "../.ethereum-testnet/goerli/geth/chaindata"
	
	ldb, err := rawdb.NewLevelDBDatabase(ldbPath, 128, 1024, "", true)
	if err != nil {
		panic(err)
	}

	stateRootNode, _ := levelDbTree.GetLastestStateTree(ldb)
	fmt.Printf("State root found :%v\n\n", stateRootNode)

	// fmt.Printf("sans lib :\n")
	// levelDbTree.NewStateExplorer(ldb, stateRootNode)
	
	// fmt.Printf("\n")

	fmt.Printf("avec lib :\n")
	levelDbTree.OldStateExplorer(ldb, stateRootNode)
}