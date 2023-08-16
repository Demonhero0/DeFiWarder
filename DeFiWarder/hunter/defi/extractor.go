package defi

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"math/big"

	common "github.com/ethereum/go-ethereum/common"
	vm "github.com/ethereum/go-ethereum/core/vm"
	abi "github.com/ethereum/go-ethereum/accounts/abi"
)

type UserTokenTxMap map[common.Address]TokenTxMap // userAddress => TokenTxMap
type TokenTxMap map[common.Address][]TokenTx // tokenAddress => []TokenTx

type TokenTx struct {
	BlockNumber *big.Int
	TxIndex int
	TransferIndex uint
	Sender common.Address
	To common.Address
	Amount *big.Int
	TokenAddress common.Address
	
	Action string
}

type UserRelatedMap map[common.Address]RelatedMap
type RelatedMap map[common.Address]*big.Int

func LoadABIJSON(path string) abi.ABI {
	jsonFile, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
	}	
	defer jsonFile.Close()
	abiByte, _ := ioutil.ReadAll(jsonFile)
	temp_abi, _ := abi.JSON(strings.NewReader(string(abiByte)))
	return temp_abi
}

func LoadActionMap(path string) map[string]string {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println("Load action json error:", err)
	}
	temp_map := make(map[string][]string)
	err = json.Unmarshal([]byte(file), &temp_map)
	if err != nil {
		fmt.Println("Json to struct error:",err)
	}
	actionMap := make(map[string]string)
	for action := range temp_map{
		for _, name := range temp_map[action]{
			actionMap[name] = action
		}
	}
	// fmt.Println(actionMap)
	return actionMap
}

type InputData struct{
	Data []byte
	Name string
	Args []interface{}
}

type LogicABIMap map[common.Address]abi.ABI

type TxExtractor struct {
	Dapp string
	ActionMap map[string]string
	ProxyInfo ProxyInfo
	ProxyABIMap map[common.Address]LogicABIMap
	UserTokenTxMap UserTokenTxMap
	UserRelatedMap UserRelatedMap
	ProxyRelatedAddresses map[common.Address]bool

	ActionInfoList map[int]ActionInfo
	AddressCountMap map[common.Address]int
	CommonAddressMap map[common.Address]bool
	CommonRelatedUser map[common.Address]bool
	RelatedToken map[common.Address]int
	FlowMergeMode string // single: single token; usd: pledged to usd; eth: pledged to eth; btc: pledged to btc; mix: conver to usd;
	
	LPTokenMap map[common.Address]bool
	StableTokenMap map[common.Address]StableToken
	TokenSwapMap map[common.Address]TokenSwap

	// for exploring
	MethodCountMap map[string]uint
	MethodCallMap map[common.Address][]InputData
	RoleMap map[common.Address]string // user, vault, manager
	RateMapPath string
}

type ActionInfo struct {
	Action string
	Function string
	Initiator common.Address
	BlockNumber *big.Int
	TxIndex int
	TokenTxList []TokenTx
	UserSuppliedAddress map[common.Address]bool
	ProxyRelatedAddress map[common.Address]bool
}

func (txExtractor *TxExtractor) Init(defiPath string){
	txExtractor.ProxyABIMap = make(map[common.Address]LogicABIMap)
	for _, proxy := range txExtractor.ProxyInfo.ProxyContracts{
		txExtractor.ProxyABIMap[proxy] = make(map[common.Address]abi.ABI)

		// load logic abi
		dirPath := defiPath + "/" + txExtractor.Dapp + "/abi/" + strings.ToLower(proxy.String())
		files, err := ioutil.ReadDir(dirPath)
		if err != nil {
			fmt.Println(err)
		}
		for _ , f := range files{
			addressString := strings.Split(f.Name(),".")[0]
			txExtractor.ProxyABIMap[proxy][common.HexToAddress(addressString)] = LoadABIJSON(dirPath + "/" + addressString + ".json")
		}
	}
	txExtractor.LPTokenMap = make(map[common.Address] bool)
	for _, token := range txExtractor.ProxyInfo.LPTokens{
		txExtractor.LPTokenMap[token] = true
	}
	// fmt.Println(txExtractor.ProxyABIMap)
	// for key := range txExtractor.ProxyABIMap{
	// 	fmt.Println("Proxy", key)
	// 	for logic := range txExtractor.ProxyABIMap[key]{
	// 		fmt.Println("Logic", logic)
	// 	}
	// }
}

func (txExtractor *TxExtractor) extractAction(input []byte, abi abi.ABI) (haveMethod bool, inputData InputData, method *abi.Method) {
	// fmt.Println(abi)
	method, err := abi.MethodById(input)
	haveMethod = false

	if err != nil{
		// fmt.Println(err)
	} else {
		methodString := method.Name + "(0x" + hex.EncodeToString(method.ID) + ")"
		args, e := method.Inputs.Unpack(input[4:])
		if e != nil {
			fmt.Println(e)
		}
		txExtractor.MethodCountMap[methodString] += 1
		haveMethod = true

		// for exploring
		inputData = InputData{
			Data: input,
			Name: methodString,
			Args: args,
		}
	}
	return haveMethod, inputData, method
}

// analyze a tx and extract token tranfer information
func (txExtractor *TxExtractor) ExtractTokenTx(ExTx *vm.ExternalTx) {

	userSuppliedAddress := make(map[common.Address]bool)
	proxyRelatedAddress := make(map[common.Address]bool)
	var transferIndex uint
	fromUserOrProxy := "user"
	if len(ExTx.InTxs) == 1{
		userSuppliedAddress[ExTx.InTxs[0].From] = true
		for proxy := range txExtractor.ProxyABIMap {
			proxyRelatedAddress[proxy] = true
		}
		txExtractor.extractInTxInfo(ExTx, ExTx.InTxs[0], userSuppliedAddress, proxyRelatedAddress, fromUserOrProxy, &transferIndex)
	}
	for relatedAddress := range(proxyRelatedAddress){
		txExtractor.ProxyRelatedAddresses[relatedAddress] = true
	}
}

func (txExtractor *TxExtractor) extractInTxInfo(ExTx *vm.ExternalTx, InTx *vm.InternalTx, userSuppliedAddress, proxyRelatedAddress map[common.Address]bool, fromUserOrProxy string, transferIndex *uint) (tokenTxList []TokenTx) {

	action := false
	var inputData InputData
	methodString := "none"
	if _, ok := txExtractor.ProxyABIMap[InTx.To]; ok && InTx.CallType != "StaticCall" {
		if InTx.CallType == "Create"{
			txExtractor.RoleMap[InTx.From] = "manager"
		}

		if userSuppliedAddress[InTx.From]{
			action = true
			fromUserOrProxy = "proxy"
		}
		if len(InTx.Input) >= 4 {
			haveMethod := false
			var method *abi.Method
			for logic := range(txExtractor.ProxyABIMap[InTx.To]){
				haveMethod, inputData, method = txExtractor.extractAction(InTx.Input, txExtractor.ProxyABIMap[InTx.To][logic])
				methodString = inputData.Name
				// when user call proxy, parse the arguments to update userSuppliedAddress
				if (haveMethod == true){
					// fmt.Println(inputData.Name)
					for index, arg := range method.Inputs.NonIndexed(){
						switch arg.Type.T {
							case abi.AddressTy:
								if userSuppliedAddress[InTx.From] && !proxyRelatedAddress[inputData.Args[index].(common.Address)] && txExtractor.RoleMap[inputData.Args[index].(common.Address)] != "manager" {
									userSuppliedAddress[inputData.Args[index].(common.Address)] = true
								}
								if txExtractor.RoleMap[InTx.From] == "manager" && methodString != "transferFrom(0x23b872dd)" && methodString != "transfer(0xa9059cbb)"{
									txExtractor.RoleMap[inputData.Args[index].(common.Address)] = "manager"
								}
						}
					}	
					break
				}
			}
		}
		proxyRelatedAddress[InTx.To] = true
	}
	
	if InTx.CallType != "StaticCall" && InTx.CallType != "DelegateCall"{
		if !userSuppliedAddress[InTx.To] && !proxyRelatedAddress[InTx.To]{
			if  userSuppliedAddress[InTx.From] && txExtractor.RoleMap[InTx.To] != "manager" {
				userSuppliedAddress[InTx.To] = true
			} else if proxyRelatedAddress[InTx.From] {
				proxyRelatedAddress[InTx.To] = true
			}
		}
	}


	// extract ether transfer
	// delegatecall and static call have no Value
	if InTx.Value != nil && InTx.Value.Cmp(new(big.Int)) > 0 {
		tokenTxList = append(tokenTxList, TokenTx{
			Sender : InTx.From,
			To : InTx.To,
			TransferIndex : *transferIndex,
			Amount : new(big.Int).Add(InTx.Value, new(big.Int)),
			TokenAddress : common.HexToAddress("0x0"),
			BlockNumber : new(big.Int).Add(ExTx.BlockNumber, big.NewInt(int64(0))),
			TxIndex : ExTx.TxIndex,
		})
		*transferIndex = *transferIndex + 1
	}
	for _, Tx := range InTx.InTxs {
		temp_tokenTxList := txExtractor.extractInTxInfo(ExTx, Tx, userSuppliedAddress, proxyRelatedAddress, fromUserOrProxy, transferIndex)
		tokenTxList = append(tokenTxList, temp_tokenTxList...)
	}

	// check the function is deposit or withdraw
	for _, event := range(InTx.Events) {
		tokenTx, isTokenTx := extractEvent(event)
		tokenTx.TransferIndex = *transferIndex
		*transferIndex = *transferIndex + 1
		if isTokenTx{
			tokenTx.BlockNumber = new(big.Int).Add(ExTx.BlockNumber, big.NewInt(int64(0)))
			tokenTx.TxIndex = ExTx.TxIndex
			tokenTxList = append(tokenTxList, tokenTx)
			if _, ok := txExtractor.LPTokenMap[tokenTx.TokenAddress]; ok {
				sender := tokenTx.Sender
				to := tokenTx.To
				var zeroAddress common.Address
				if sender != zeroAddress && to != zeroAddress {
					// add sender in to's related users
					if _, ok := txExtractor.ProxyABIMap[sender]; !ok {
						if _, ok1 := txExtractor.ProxyABIMap[to]; !ok1 {
							updateUserRelatedMap(txExtractor.UserRelatedMap, to, sender, ExTx.BlockNumber)
						}
					}
				}
			}
		}
	}

	if action == true{
		// fmt.Println(functionSiginature)
		zero, _ := new(big.Int).SetString("0",10)
		var zeroAddress common.Address
		for _, tokenTx := range(tokenTxList){
			if _, ok := txExtractor.ProxyABIMap[tokenTx.TokenAddress]; ok {
				if (tokenTx.Sender == zeroAddress || tokenTx.To == zeroAddress){
				// txExtractor.LPTokenMap[tokenTx.TokenAddress] = true
				}
			} else if tokenTx.Sender != zeroAddress && tokenTx.To != zeroAddress {
				txExtractor.AddressCountMap[tokenTx.Sender] += 1
				txExtractor.AddressCountMap[tokenTx.To] += 1
			}
			// delete(userSuppliedAddress, tokenTx.TokenAddress)
			// delete(proxyRelatedAddress, tokenTx.TokenAddress)
		}
		actionInfo := ActionInfo{
			Action : "none",
			Function : methodString,
			Initiator : InTx.From,
			BlockNumber : new(big.Int).Add(ExTx.BlockNumber, zero),
			TxIndex : ExTx.TxIndex,
			TokenTxList : tokenTxList,
			UserSuppliedAddress : userSuppliedAddress,
			ProxyRelatedAddress : proxyRelatedAddress,
		}
		txExtractor.ActionInfoList[len(txExtractor.ActionInfoList)] = actionInfo
		var emptyTokenTxList []TokenTx
		return emptyTokenTxList
	}
	return tokenTxList
}

func (txExtractor *TxExtractor) UpdateCommonRelatedUser(){
	for userAddress := range txExtractor.UserRelatedMap{
		if len(txExtractor.UserRelatedMap[userAddress]) > 50{
			txExtractor.CommonRelatedUser[userAddress] = true
		}
	}
}

func (txExtractor *TxExtractor) UpdateCommonAddressMap(){
	length := len(txExtractor.ActionInfoList)
	txExtractor.CommonAddressMap = make(map[common.Address]bool)
	for proxy := range txExtractor.ProxyABIMap{
		txExtractor.CommonAddressMap[proxy] = true
	}
	if length > 100{
		for address := range txExtractor.AddressCountMap{
			// if txExtractor.AddressCountMap[address] > 350 || 
			if float64(txExtractor.AddressCountMap[address]) > float64(length) * float64(0.3) {
				fmt.Println(address, txExtractor.AddressCountMap[address], len(txExtractor.ActionInfoList))
				txExtractor.CommonAddressMap[address] = true
			}
		}
	}
	// fmt.Println("commonAddressMap", txExtractor.CommonAddressMap)
}

func (txExtractor *TxExtractor) ExtractActionInfoListNew() {
	var zeroAddress common.Address
	userTokenTxMap := txExtractor.UserTokenTxMap
	userRelatedMap := txExtractor.UserRelatedMap
	actionMapCount := make(map[string]int)
	for _, actionInfo := range txExtractor.ActionInfoList{
		var depositTo []common.Address
		var withdrawFrom []common.Address
		var TokenTxListWithoutLP []TokenTx
		for _, tokenTx := range(actionInfo.TokenTxList){
			sender := tokenTx.Sender
			to := tokenTx.To
			tokenAddress := tokenTx.TokenAddress
			// isMintOrBurn := false
			if _, ok := txExtractor.LPTokenMap[tokenAddress]; ok {
				if _, ok1 := txExtractor.CommonAddressMap[to]; !ok1 && sender == zeroAddress{
					// deposit
					// isMintOrBurn = true
					depositTo = append(depositTo, to)
				} else if _, ok1 := txExtractor.CommonAddressMap[sender]; !ok1 && to == zeroAddress{
					// withdraw
					// isMintOrBurn = true
					withdrawFrom = append(withdrawFrom, sender)
				}
			} else if sender != zeroAddress && to != zeroAddress { 
				TokenTxListWithoutLP = append(TokenTxListWithoutLP, tokenTx)
			}
			
			// else if tokenTx.Sender == actionInfo.Initiator || tokenTx.To == actionInfo.Initiator{
			// 	TokenTxListWithoutLP = append(TokenTxListWithoutLP, tokenTx)
			// } else if _, ok := txExtractor.CommonAddressMap[tokenTx.To]; ok {
			// 	TokenTxListWithoutLP = append(TokenTxListWithoutLP, tokenTx)
			// } else if _, ok := txExtractor.CommonAddressMap[tokenTx.Sender]; ok {
			// 	TokenTxListWithoutLP = append(TokenTxListWithoutLP, tokenTx)
			// }
				
			// only add the token transfer cross users and proxys
			// else if _, ok := actionInfo.ProxyRelatedAddress[sender]; ok {
			// 	if _, ok1 := actionInfo.UserSuppliedAddress[to]; ok1 {
			// 		TokenTxListWithoutLP = append(TokenTxListWithoutLP, tokenTx)
			// 		isWithdraw = true
			// 	}
			// } else if _, ok := actionInfo.ProxyRelatedAddress[to]; ok {
			// 	if _, ok1 := actionInfo.UserSuppliedAddress[sender]; ok1 {
			// 		TokenTxListWithoutLP = append(TokenTxListWithoutLP, tokenTx)
			// 		isDeposit = true
			// 	}
			// } 
		}

		// action := "none_"
		// if isWithdraw && isDeposit {
		// 	action = "borrow_" + actionInfo.Function
		// } else if isDeposit {
		// 	action = "deposit_" + actionInfo.Function
		// } else if isWithdraw {
		// 	action = "withdraw_" + actionInfo.Function
		// }
		// actionMapCount[action] += 1

		for _, tokenTx := range(TokenTxListWithoutLP){
			// tokenTx.TxIndex = actionInfo.TxIndex
			// tokenTx.BlockNumber = actionInfo.BlockNumber
			// deposit
			if actionInfo.UserSuppliedAddress[tokenTx.Sender] && !txExtractor.CommonAddressMap[tokenTx.Sender] && !actionInfo.UserSuppliedAddress[tokenTx.To] && (txExtractor.CommonAddressMap[tokenTx.To] || actionInfo.ProxyRelatedAddress[tokenTx.To] || tokenTx.Sender == actionInfo.Initiator) {
			// if actionInfo.ProxyRelatedAddress[tokenTx.To] && tokenTx.Sender != zeroAddress{
				tokenTx.Action = "deposit"
				if _, ok := userTokenTxMap[tokenTx.Sender]; !ok {
					userTokenTxMap[tokenTx.Sender] = make(TokenTxMap)
				}
				userTokenTxMap[tokenTx.Sender][tokenTx.TokenAddress] = append(userTokenTxMap[tokenTx.Sender][tokenTx.TokenAddress], tokenTx)
				// handle the case that sender deposit to relatedUser
				for _, relatedUser := range(depositTo){
					updateUserRelatedMap(userRelatedMap, relatedUser, tokenTx.Sender, actionInfo.BlockNumber)
				}
				if actionInfo.UserSuppliedAddress[tokenTx.Sender] && !actionInfo.ProxyRelatedAddress[tokenTx.Sender] && txExtractor.RoleMap[tokenTx.Sender] != "manager"{
					updateUserRelatedMap(userRelatedMap, actionInfo.Initiator, tokenTx.Sender, actionInfo.BlockNumber)
				}
				// for relatedUser := range(actionInfo.UserSuppliedAddress){
				// 	updateUserRelatedMap(userRelatedMap, relatedUser, tokenTx.Sender, actionInfo.BlockNumber)
				// }
				txExtractor.RelatedToken[tokenTx.TokenAddress] += 1
			// withdraw
			} else if actionInfo.UserSuppliedAddress[tokenTx.To] && !txExtractor.CommonAddressMap[tokenTx.To] && !actionInfo.UserSuppliedAddress[tokenTx.Sender] && (txExtractor.CommonAddressMap[tokenTx.Sender] || actionInfo.ProxyRelatedAddress[tokenTx.Sender] || tokenTx.To == actionInfo.Initiator) {
			// } else if actionInfo.ProxyRelatedAddress[tokenTx.Sender] && tokenTx.To != zeroAddress {
				tokenTx.Action = "withdraw"
				if _, ok := userTokenTxMap[tokenTx.To]; !ok {
					userTokenTxMap[tokenTx.To] = make(TokenTxMap)
				}
				userTokenTxMap[tokenTx.To][tokenTx.TokenAddress] = append(userTokenTxMap[tokenTx.To][tokenTx.TokenAddress], tokenTx)
				// handle the case that to withdraw from relatedUser
				for _, relatedUser := range(withdrawFrom){
					updateUserRelatedMap(userRelatedMap, tokenTx.To, relatedUser, actionInfo.BlockNumber)
				}
				if actionInfo.UserSuppliedAddress[tokenTx.To] && !actionInfo.ProxyRelatedAddress[tokenTx.To] && txExtractor.RoleMap[tokenTx.To] != "manager"{
					updateUserRelatedMap(userRelatedMap, tokenTx.To, actionInfo.Initiator, actionInfo.BlockNumber)
				}
				// for relatedUser := range(actionInfo.UserSuppliedAddress){
				// 	updateUserRelatedMap(userRelatedMap, tokenTx.To, relatedUser, actionInfo.BlockNumber)
				// }
				txExtractor.RelatedToken[tokenTx.TokenAddress] += 1
			// handle the transfer between users
			} 
		}
	}

	mode := "single"
	modeMap := make(map[string]int)
	for token := range txExtractor.RelatedToken{
		if txExtractor.StableTokenMap[token].XToken == common.HexToAddress("0x1"){
			modeMap["eth"] += 1
		} else if txExtractor.StableTokenMap[token].XToken == common.HexToAddress("0x2"){
			modeMap["usd"] += 1
		} else if txExtractor.StableTokenMap[token].XToken == common.HexToAddress("0x3"){
			modeMap["btc"] += 1
		} else {
			modeMap["others"] += 1
		}
	}
	if len(modeMap) == 1{
		if modeMap["eth"] > 0{
			mode = "eth"
		} else if modeMap["usd"] > 0{
			mode = "usd"
		} else if modeMap["btc"] > 0{
			mode = "btc"
		} else if modeMap["others"] > 1{
			mode = "mix"
		}
	} else {
		mode = "mix"
	}
	txExtractor.FlowMergeMode = mode
	fmt.Println("tokens", len(txExtractor.RelatedToken), mode)
	// fmt.Println(actionMapCount)
	fmt.Println("actionMapCount")
	for key := range(actionMapCount){
		fmt.Println(key, actionMapCount[key])
	}
}

func (txExtractor *TxExtractor) ExtractActionInfoList(){
	var zeroAddress common.Address
	userTokenTxMap := txExtractor.UserTokenTxMap
	userRelatedMap := txExtractor.UserRelatedMap
	for _, actionInfo := range txExtractor.ActionInfoList{
		var depositTo []common.Address
		var withdrawFrom []common.Address
		var TokenTxListWithoutLP []TokenTx
		for _, tokenTx := range(actionInfo.TokenTxList){
			sender := tokenTx.Sender
			to := tokenTx.To
			tokenAddress := tokenTx.TokenAddress
			// isMintOrBurn := false
			if _, ok := txExtractor.LPTokenMap[tokenAddress]; ok {
				if sender == zeroAddress{
					// deposit
					// isMintOrBurn = true
					depositTo = append(depositTo, to)
				} else if to == zeroAddress{
					// withdraw
					// isMintOrBurn = true
					withdrawFrom = append(withdrawFrom, sender)
				}
			} else {
			// if isMintOrBurn == false {
				TokenTxListWithoutLP = append(TokenTxListWithoutLP, tokenTx)
			}
		}

		// var linkTokenTxList []TokenTx
		// for _, tokenTx := range(TokenTxListWithoutLP){
		// 	for _,
		// }
		if actionInfo.Action == "deposit"{
			for _, tokenTx := range(TokenTxListWithoutLP){
				tokenTx.TxIndex = actionInfo.TxIndex
				tokenTx.BlockNumber = actionInfo.BlockNumber
				tokenTx.Action = actionInfo.Action
				if txExtractor.CommonAddressMap[tokenTx.To] && tokenTx.Sender != zeroAddress{
					if _, ok := userTokenTxMap[tokenTx.Sender]; !ok {
						userTokenTxMap[tokenTx.Sender] = make(TokenTxMap)
					}
					userTokenTxMap[tokenTx.Sender][tokenTx.TokenAddress] = append(userTokenTxMap[tokenTx.Sender][tokenTx.TokenAddress], tokenTx)

					// handle the case that sender deposit to relatedUser
					for _, relatedUser := range(depositTo){
						updateUserRelatedMap(userRelatedMap, relatedUser, tokenTx.Sender, actionInfo.BlockNumber)
					}
					txExtractor.RelatedToken[tokenTx.TokenAddress] += 1
				}
			}
		} else if actionInfo.Action == "withdraw"{
			for _, tokenTx := range(TokenTxListWithoutLP){
				tokenTx.TxIndex = actionInfo.TxIndex
				tokenTx.BlockNumber = actionInfo.BlockNumber
				tokenTx.Action = actionInfo.Action
				if txExtractor.CommonAddressMap[tokenTx.Sender] && tokenTx.To != zeroAddress {
					if _, ok := userTokenTxMap[tokenTx.To]; !ok {
						userTokenTxMap[tokenTx.To] = make(TokenTxMap)
					}
					userTokenTxMap[tokenTx.To][tokenTx.TokenAddress] = append(userTokenTxMap[tokenTx.To][tokenTx.TokenAddress], tokenTx)
					// handle the case that to withdraw from relatedUser
					for _, relatedUser := range(withdrawFrom){
						updateUserRelatedMap(userRelatedMap, tokenTx.To, relatedUser, actionInfo.BlockNumber)
					}
					txExtractor.RelatedToken[tokenTx.TokenAddress] += 1
				}
			} 
		} else if actionInfo.Action == "borrow"{
			for _, tokenTx := range(TokenTxListWithoutLP){
				tokenTx.TxIndex = actionInfo.TxIndex
				tokenTx.BlockNumber = actionInfo.BlockNumber
				if txExtractor.CommonAddressMap[tokenTx.To] && tokenTx.Sender != zeroAddress{
					tokenTx.Action = "borrow-in"
					if _, ok := userTokenTxMap[tokenTx.Sender]; !ok {
						userTokenTxMap[tokenTx.Sender] = make(TokenTxMap)
					}
					userTokenTxMap[tokenTx.Sender][tokenTx.TokenAddress] = append(userTokenTxMap[tokenTx.Sender][tokenTx.TokenAddress], tokenTx)

					txExtractor.RelatedToken[tokenTx.TokenAddress] += 1
				} else if txExtractor.CommonAddressMap[tokenTx.Sender] && tokenTx.To != zeroAddress{
					tokenTx.Action = "borrow-out"
					if _, ok := userTokenTxMap[tokenTx.To]; !ok {
						userTokenTxMap[tokenTx.To] = make(TokenTxMap)
					}
					userTokenTxMap[tokenTx.To][tokenTx.TokenAddress] = append(userTokenTxMap[tokenTx.To][tokenTx.TokenAddress], tokenTx)

					txExtractor.RelatedToken[tokenTx.TokenAddress] += 1
				}
			}
		} else if actionInfo.Action == "swap"{
			for _, tokenTx := range(TokenTxListWithoutLP){
				tokenTx.TxIndex = actionInfo.TxIndex
				tokenTx.BlockNumber = actionInfo.BlockNumber
				if txExtractor.CommonAddressMap[tokenTx.To] && tokenTx.Sender != zeroAddress{
					tokenTx.Action = "swap-in"
					if _, ok := userTokenTxMap[tokenTx.Sender]; !ok {
						userTokenTxMap[tokenTx.Sender] = make(TokenTxMap)
					}
					userTokenTxMap[tokenTx.Sender][tokenTx.TokenAddress] = append(userTokenTxMap[tokenTx.Sender][tokenTx.TokenAddress], tokenTx)

					txExtractor.RelatedToken[tokenTx.TokenAddress] += 1
				} else if txExtractor.CommonAddressMap[tokenTx.Sender] && tokenTx.To != zeroAddress{
					tokenTx.Action = "swap-out"
					if _, ok := userTokenTxMap[tokenTx.To]; !ok {
						userTokenTxMap[tokenTx.To] = make(TokenTxMap)
					}
					userTokenTxMap[tokenTx.To][tokenTx.TokenAddress] = append(userTokenTxMap[tokenTx.To][tokenTx.TokenAddress], tokenTx)

					txExtractor.RelatedToken[tokenTx.TokenAddress] += 1
				}
			}
		}
	}

	mode := "single"
	modeMap := make(map[string]int)
	for token := range txExtractor.RelatedToken{
		if txExtractor.StableTokenMap[token].XToken == common.HexToAddress("0x1"){
			modeMap["eth"] += 1
		} else if txExtractor.StableTokenMap[token].XToken == common.HexToAddress("0x2"){
			modeMap["usd"] += 1
		} else if txExtractor.StableTokenMap[token].XToken == common.HexToAddress("0x3"){
			modeMap["btc"] += 1
		} else {
			modeMap["others"] += 1
		}
	}
	if len(modeMap) == 1{
		if modeMap["eth"] > 0{
			mode = "eth"
		} else if modeMap["usd"] > 0{
			mode = "usd"
		} else if modeMap["btc"] > 0{
			mode = "btc"
		} else if modeMap["others"] > 1{
			mode = "mix"
		}
	} else {
		mode = "mix"
	}
	txExtractor.FlowMergeMode = mode
	fmt.Println("tokens", len(txExtractor.RelatedToken), mode)
}

func updateUserRelatedMap(userRelatedMap UserRelatedMap, mainUser common.Address, relatedUser common.Address, blockNumber *big.Int){
	if mainUser != relatedUser{
		if _, ok := userRelatedMap[mainUser]; !ok{
			userRelatedMap[mainUser] = make(RelatedMap)
		}
		if _, ok := (userRelatedMap[mainUser][relatedUser]); !ok{
			userRelatedMap[mainUser][relatedUser] = blockNumber
		} else if userRelatedMap[mainUser][relatedUser].Cmp(blockNumber) == 1 {
			userRelatedMap[mainUser][relatedUser] = blockNumber
		}
	}
}

func (txExtractor *TxExtractor) ExtractStakeTokenTx(ExTx *vm.ExternalTx){
	if len(ExTx.InTxs) == 1{
		txExtractor.extractInTxForStake(ExTx, ExTx.InTxs[0])
	}
}

func (txExtractor *TxExtractor) extractInTxForStake(ExTx *vm.ExternalTx, InTx *vm.InternalTx) {
	userRelatedMap := txExtractor.UserRelatedMap
	for _, Tx := range InTx.InTxs {
		txExtractor.extractInTxForStake(ExTx, Tx)
	}

	for _, event := range(InTx.Events) {
		tokenTx, isTokenTx := extractEvent(event)
		if isTokenTx{
			if _, ok := txExtractor.LPTokenMap[tokenTx.TokenAddress]; ok {
				sender := tokenTx.Sender
				to := tokenTx.To
				var zeroAddress common.Address
				if sender != zeroAddress && to != zeroAddress {
					// add sender in to's related users
					if _, ok := txExtractor.ProxyABIMap[sender]; !ok {
						if _, ok1 := txExtractor.ProxyABIMap[to]; !ok1 {
							updateUserRelatedMap(userRelatedMap, to, sender, ExTx.BlockNumber)
						}
					}
				}
			}
		}
	}
}

func extractEvent(event *vm.Event) (TokenTx, bool) {
	
	var tokenTx TokenTx
	var isTokenTx bool
	if len(event.Topics) > 0 && event.Topics[0].String() == "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" && len(event.Topics) == 3{
		// parsed, err := abi.JSON(strings.NewReader(ERC20ABI))
		parsed, err := abi.JSON(strings.NewReader(erc20string))
		if err != nil {
			fmt.Println("Parser error", err)
		}
		res, err := parsed.Unpack("Transfer", event.Data)
		if err != nil {
			fmt.Println("Parser error", err)
		}
		// fmt.Println(reflect.TypeOf(res[0]),err)
		sender := common.HexToAddress(event.Topics[1].String())
		to := common.HexToAddress(event.Topics[2].String())
		amount, _ := res[0].(*big.Int)
		tokenTx = TokenTx{
			Sender : sender,
			To : to,
			Amount : amount,
			TokenAddress : event.Address,
		}
		// fmt.Println("event index", event.Index, tokenTx.EventIndex)
		isTokenTx = true
		// fmt.Println(tokenTx, isTokenTx)
	}
	return tokenTx, isTokenTx
}

func InitTokenUserTxMap(startBlock, endBlock uint64, txExtractor TxExtractor, path string) {
	
	tx_count := 0
	usedTxs := make(map[string]bool)
	for _, proxy := range txExtractor.ProxyInfo.ProxyContracts{
		proxy_tx_path := path + txExtractor.Dapp + "/historyTx/" + strings.ToLower(proxy.String()) + "/"
		files, err := ioutil.ReadDir(proxy_tx_path)
		if err != nil {
			fmt.Println(err)
		}

		for _ , f := range files[:]{
			if _, ok := usedTxs[f.Name()]; ok {
				continue
			} else {
				usedTxs[f.Name()] = true
			}
			temp_list := strings.Split(strings.Split(f.Name(),".")[0], "_")
			b, err := strconv.ParseUint(temp_list[0],10,64)
			if err != nil {
				fmt.Println(err)
			}
			if b <= endBlock && b >= startBlock{
				ExTx := vm.LoadTx(proxy_tx_path + f.Name())
				txExtractor.ExtractTokenTx(&ExTx)
				tx_count += 1
			} else {
				continue
			}
			if (tx_count % 1000 == 0){
				fmt.Println(tx_count)
			}
			// if tx_count > 200000{
			// 	break
			// }
		}
	}
	fmt.Println("proxy tx nums", tx_count)
}

func InitUserRelatedMapWithStake(startBlock, endBlock uint64, txExtractor TxExtractor, path string){

	tx_count := 0
	for _, stake := range txExtractor.ProxyInfo.LPTokens{
		stake_tx_path := path + txExtractor.Dapp + "/historyTx/" + strings.ToLower(stake.String()) + "/"
		files, err := ioutil.ReadDir(stake_tx_path)
		if err != nil {
			fmt.Println(err)
		}
		for _ , f := range files{
			temp_list := strings.Split(strings.Split(f.Name(),".")[0], "_")
			b, err := strconv.ParseUint(temp_list[0],10,64)
			if err != nil {
				fmt.Println(err)
			}
			if b <= endBlock && b >= startBlock{
				ExTx := vm.LoadTx(stake_tx_path + f.Name())
				txExtractor.ExtractStakeTokenTx(&ExTx)
				tx_count += 1
			} else {
				continue
			}
		}
	}
	fmt.Println("lp tx nums", tx_count)
}

func DeepCpUserTokenTxMap(userTokenTxMap UserTokenTxMap) UserTokenTxMap {
	var newUserTokenTxMap UserTokenTxMap
	newUserTokenTxMap = make(UserTokenTxMap)
	for userAddress := range(userTokenTxMap){
		newUserTokenTxMap[userAddress] = make(TokenTxMap)
		for tokenAddress := range userTokenTxMap[userAddress]{
			for _, tokenTx := range userTokenTxMap[userAddress][tokenAddress] {
				newUserTokenTxMap[userAddress][tokenAddress] = append(newUserTokenTxMap[userAddress][tokenAddress], tokenTx) 
			}
		}
	}

	return newUserTokenTxMap
}

func DeepCpUserRelatedMap(userRelatedMap UserRelatedMap) UserRelatedMap {
	var newUserRelatedMap UserRelatedMap
	newUserRelatedMap = make(UserRelatedMap)
	for userAddress := range userRelatedMap{
		newUserRelatedMap[userAddress] = make(RelatedMap)
		for relatedAddress := range userRelatedMap[userAddress]{
			newUserRelatedMap[userAddress][relatedAddress] = userRelatedMap[userAddress][relatedAddress]
		}
	}
	return newUserRelatedMap
}