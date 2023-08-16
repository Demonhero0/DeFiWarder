package main

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	defi "github.com/ethereum/go-ethereum/hunter/defi"
)

func main() {
	dapp := "opyn"
	userAddress := common.HexToAddress("0x0")

	startBlock := uint64(0)
	endBlock := uint64(15000000)
	fmt.Println("Start", dapp)

	source := "hunter"
	proxyInfoMap := defi.LoadDefi(source+"/defi_warder.csv", false, false)
	txExtractor := defi.TxExtractor{
		Dapp:                  dapp,
		ActionMap:             defi.LoadActionMap(source + "/defi/ActionMap.json"),
		ProxyInfo:             proxyInfoMap[dapp],
		ProxyRelatedAddresses: make(map[common.Address]bool),
		UserTokenTxMap:        make(defi.UserTokenTxMap),
		UserRelatedMap:        make(defi.UserRelatedMap),
		AddressCountMap:       make(map[common.Address]int),
		ActionInfoList:        make(map[int]defi.ActionInfo),
		MethodCountMap:        make(map[string]uint),
		RelatedToken:          make(map[common.Address]int),
		CommonRelatedUser:     make(map[common.Address]bool),
		StableTokenMap:        defi.LoadStableTokenInfo(source + "/stableToken"),
		TokenSwapMap:  make(map[common.Address]defi.TokenSwap),
		MethodCallMap: make(map[common.Address][]defi.InputData),
		RoleMap:       make(map[common.Address]string),
		RateMapPath:   source + "/priceDataETH",
	}
	txExtractor.Init(source + "/defi_apps")

	defi.InitTokenUserTxMap(startBlock, endBlock, txExtractor, source+"/defi_apps/")
	defi.InitUserRelatedMapWithStake(startBlock, endBlock, txExtractor, source+"/defi_apps/")

	fmt.Println("len of txExtractor.ActionInfoList", len(txExtractor.ActionInfoList))
	txExtractor.UpdateCommonAddressMap()
	txExtractor.UpdateCommonRelatedUser()
	txExtractor.ExtractActionInfoListNew()

	fmt.Println("userTokenTxMap", len(txExtractor.UserTokenTxMap))

	checker := defi.LeakingChecker{
		TokenRateMap: make(map[common.Address][]defi.RateRecord),
	}
	userTokenFlowMap, _ := defi.GenerateUserFlow(&txExtractor)
	checker.UserTokenFlowMap = userTokenFlowMap
	checker.RecordRate(&txExtractor)
	checker.AbnormalDetection()

	fmt.Println("tokenTx")
	for tokenAddress := range txExtractor.UserTokenTxMap[userAddress] {
		fmt.Println(tokenAddress, txExtractor.UserTokenTxMap[userAddress][tokenAddress])
	}
	fmt.Println("userflow")
	for tokenAddress := range userTokenFlowMap[userAddress] {
		fmt.Println("tokenAddress", tokenAddress)
		for i, tokenFlow := range userTokenFlowMap[userAddress][tokenAddress] {
			fmt.Println(i, tokenFlow)
		}
	}
	fmt.Println("related user")
	for relatedAddress := range txExtractor.UserRelatedMap[userAddress] {
		fmt.Println(relatedAddress, txExtractor.UserRelatedMap[userAddress][relatedAddress])
		for tokenAddress := range userTokenFlowMap[relatedAddress] {
			fmt.Println("tokenAddress", tokenAddress)
			for i, tokenFlow := range userTokenFlowMap[relatedAddress][tokenAddress] {
				fmt.Println(i, tokenFlow)
			}
		}
	}
	fmt.Println("strat single check")
	defi.CheckAttack(userAddress, userTokenFlowMap, &txExtractor)
}
