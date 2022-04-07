package tools

import (
	"bytes"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state/snapshot"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	log "github.com/inconshreveable/log15"
	"github.com/schollz/progressbar/v3"
)

func StateAndStorageTrees(ldbPath string) {
	ldb := getLDB(ldbPath)

	chan_display := make(chan string, 6)
	
	stateTrees := getStateTrees(ldb)
	latestStateTree := stateTrees[0]

	var wg sync.WaitGroup
	
	wg.Add(1)
	go func() {
		defer wg.Done()
		getStorageTreeSize(ldb, latestStateTree.stateRoot, chan_display)
	}()
	
	wg.Add(1)
	go func() {
		defer wg.Done()
		getStateTreeSize(ldb, latestStateTree.stateRoot, chan_display)
	}()
	
	wg.Wait()
	close(chan_display)

	fmt.Printf("\nTotal number of tree state : %v\n\n", len(stateTrees))
	
	fmt.Printf("Latest state tree : \n")
	fmt.Printf(" - Block number : %x\n", latestStateTree.blockNumber)
	fmt.Printf(" - State root : %x\n\n", latestStateTree.stateRoot)

	for res := range chan_display {
		fmt.Print(res)
	}
}

func CountStateTrees(ldbPath string) {
	ldb := getLDB(ldbPath)

	stateTrees := getStateTrees(ldb)

	fmt.Printf("\nTotal number of tree state : %v\n", len(stateTrees))
}

func ReadSnapshot(ldbPath string, addr string)  {
	ldb := getLDB(ldbPath)

	addrHash := crypto.Keccak256Hash(common.Hex2Bytes(addr))
	key := accountSnapshotKey(addrHash)
	
	value, err := ldb.Get(key)
	if err != nil {
		panic(err)
	}

	var data snapshot.Account 
	rlp.DecodeBytes(value, &data)

	fmt.Printf("Snapshot : \n")
	fmt.Printf("key : %x\n", key)
	fmt.Printf("value : %x\n\n", value)
	fmt.Printf("address : %v\n", addr)
	fmt.Printf("data : %v\n", data)
}

/*
==================================================================================================================================
*/

type stateFound struct {
	blockNumber *big.Int;
	stateRoot common.Hash;
}

func getStateTrees(ldb ethdb.Database) ([]stateFound) {
	var res []stateFound
	bar := progressbar.Default(-1, "Block crowled")
	fmt.Printf("\n")

	headerHash, _ := ldb.Get(headHeaderKey)
	for headerHash != nil {
		var blockHeader types.Header
		blockNb, _ := ldb.Get(append(headerNumberPrefix, headerHash...))
		if blockNb == nil {
			break
		}
		blockHeaderRaw, _ := ldb.Get(append(headerPrefix[:], append(blockNb, headerHash...)...))
		rlp.DecodeBytes(blockHeaderRaw, &blockHeader)

		stateRootNode, _ := ldb.Get(blockHeader.Root.Bytes())

		bar.Add(1)
		if len(stateRootNode) > 0 {
			res = append(res, stateFound{blockHeader.Number, blockHeader.Root})
		}

		headerHash = blockHeader.ParentHash.Bytes()
	}
	bar.Close()
	return res
}

func getStateTreeSize(ldb ethdb.Database, stateRootNode common.Hash, display chan string) {
	stateTreeSize, stateTreeLeafSize := getTreeSize(ldb, stateRootNode)

	display <- fmt.Sprintf("\nLatest state leaf size : %v bytes\n", stateTreeLeafSize)
	display <- fmt.Sprintf("Latest state tree size : %v bytes\n", stateTreeSize)
}

func getStorageTreeSize(ldb ethdb.Database, stateRootNode common.Hash, display chan string) {
	chan_storageRootNodes := make(chan common.Hash)

	go getStorageRootNodes(ldb, stateRootNode, chan_storageRootNodes, display)

	total := 0
	totalLeaf := 0

	for storageRoot := range chan_storageRootNodes {
		treeSize, leafSize := getTreeSize(ldb, storageRoot)
		total += treeSize
		totalLeaf += leafSize
	}

	display <- fmt.Sprintf("\nLatest storage leaf size : %v bytes\n", totalLeaf)
	display <- fmt.Sprintf("Latest storage tree size : %v bytes\n", total)
}

// Go through the state tree to put in the channel the hashes of the smartcontracts root nodes
func getStorageRootNodes(ldb ethdb.Database, stateRootNode common.Hash, c chan common.Hash, display chan string) {
	defer close(c)

	trieDB := trie.NewDatabase(ldb)
	treeState, _ := trie.New(stateRootNode, trieDB)

	it := trie.NewIterator(treeState.NodeIterator(nil))
	nbAccount := 0
	nbSmartcontract := 0
	for it.Next() {
		var acc snapshot.Account
		if err := rlp.DecodeBytes(it.Value, &acc); err != nil {
			panic(err)
		}

		nbAccount++
		if bytes.Compare(acc.Root, emptyStorageRoot) != 0 {
			nbSmartcontract++
			c <- common.BytesToHash(acc.Root)
		}

		if nbAccount%10000==0 {
			log.Info("Found", "Accounts", nbAccount, "Smartcontracts", nbSmartcontract)
		}
	}

	display <- fmt.Sprintf("\nFinal account number :%v\n", nbAccount)
	display <- fmt.Sprintf("Final smartcontract number :%v\n", nbSmartcontract)
}

// 
func getTreeSize(ldb ethdb.Database, rootNode common.Hash) (treeSize int, leafSize int) {

	chan_nodeSize := make(chan int)
	chan_leafSize := make(chan int)

	go func() {
		exploreTreeNodes(ldb, rootNode, chan_nodeSize, chan_leafSize)
		defer close(chan_nodeSize)
		defer close(chan_leafSize)
	}()

	total := 0
	totalLeaf := 0

	go func() {
		for s := range chan_leafSize {
			totalLeaf += s
		}
	}()

	for s := range chan_nodeSize {
		total += s
	}

	return total, totalLeaf
	
}

func exploreTreeNodes(ldb ethdb.Database, rootNode common.Hash, nodeSize chan int, leafSize chan int) {
	value, err := ldb.Get(rootNode[:])
	if err != nil {
		return
	}
	
	var nodes [][]byte
	rlp.DecodeBytes(value, &nodes)

	// send result in channel
	if len(nodes) == 2 {
		leafSize <- len(rootNode) + len(value)
	}
	nodeSize <- len(rootNode) + len(value)
	
	// explore next nodes
	for _, keyNode := range nodes {
		if len(keyNode) == 0 {
			continue
		}
		exploreTreeNodes(ldb, common.BytesToHash(keyNode), nodeSize, leafSize)
	}
}

func getLDB(ldbPath string) ethdb.Database {
	ldb, err := rawdb.NewLevelDBDatabase(ldbPath, 0, 0, "", true)
	if err != nil {
		panic(err)
	}
	fmt.Print("LevelDB ok\n")
	return ldb
}