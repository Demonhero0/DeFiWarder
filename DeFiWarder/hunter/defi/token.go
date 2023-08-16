package defi

import (
	"encoding/hex"
	"strings"
	"strconv"
	"os"
	"fmt"
	"encoding/csv"
	"io"
	"io/ioutil"
	"math/big"
	"encoding/json"

	vm "github.com/ethereum/go-ethereum/core/vm"
	common "github.com/ethereum/go-ethereum/common"
)

type SwapInfo struct {
	SourceToken *big.Int
	TargetToken *big.Int
	BlockNumber *big.Int
}

type TokenInfo struct {
	Symbol string
	SwapToWeth common.Address
	SwapHistory []SwapInfo
}

type SwapEvent struct {
	BlockNumber *big.Int
	TxIndex int
	EventIndex uint
	Event *vm.Event
}


// write swap events
func DumpSwapEvent(swapEvent SwapEvent, dumpPath string){
	err := os.Mkdir(dumpPath + "/" + strings.ToLower(swapEvent.Event.Address.String()), os.ModePerm)

	b, err := json.Marshal(swapEvent)
	if err != nil{
		fmt.Println("SwapEvent Dump", err)
	}
	ioutil.WriteFile(dumpPath + "/" + strings.ToLower(swapEvent.Event.Address.String()) + "/" + swapEvent.BlockNumber.String() + "_" + strconv.Itoa(swapEvent.TxIndex) + "_" + strconv.FormatUint(uint64(swapEvent.EventIndex),10) + ".json", b, 0644)
}

type TokenCollector struct {
	TokenSwapMap map[string]string // usdc_eth => address.string()
	DumpPath string
	TokenMap map[common.Address]bool
}

func (tokenCollector *TokenCollector) LoadInitTokenInfo(filePath string, dumpPath string) map[common.Address]TokenInfo {
	tokenCollector.TokenSwapMap = make(map[string]string)
	csvFile, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Load error", err)
	}
	defer csvFile.Close()
	tokenCollector.DumpPath = dumpPath
	err = os.Mkdir(tokenCollector.DumpPath, os.ModePerm)
	tokenInfoMap := make(map[common.Address]TokenInfo)
	// Parse the file and continue the first row
	r := csv.NewReader(csvFile)
	_, _ = r.Read()
	// iterate through the records
	for{
		// Read each record from csv
		record, err := r.Read()
		if len(record) == 0 {
			break
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Read Line error", err)
		}
		// symbol,tokenAddress,decimals,toToken,swapAddress
		symbol := strings.TrimSpace(strings.ToLower(record[0]))
		btoken := strings.TrimSpace(strings.ToLower(record[3]))
		tokenAddress := common.HexToAddress(strings.ToLower(strings.TrimSpace(record[4])))
		if strings.ToLower(strings.TrimSpace(record[2])) != "0x0000000000000000000000000000000000000000"{
			tokenCollector.TokenSwapMap[strings.ToLower(strings.TrimSpace(record[4]))] = symbol + "_" + btoken
		}
		tokenInfoMap[tokenAddress] = TokenInfo{
			Symbol : symbol,
			SwapToWeth : common.HexToAddress(strings.ToLower(strings.TrimSpace(record[2]))),
		}
		err = os.Mkdir(tokenCollector.DumpPath + "/" + symbol + "_" + btoken, os.ModePerm)
	}
	// fmt.Println(len(tokenCollector.TokenSwapMap))
	// fmt.Println(tokenCollector.TokenSwapMap)
	return tokenInfoMap
}

func (tokenCollector *TokenCollector) CollectTokenSwap(ExTx *vm.ExternalTx, dumpBool bool){
	token_dump_dict := make(map[string]string)
	if len(ExTx.InTxs) == 1{
		tokenCollector.collectTokenSwapUtil(ExTx, ExTx.InTxs[0], token_dump_dict)
	}
	if dumpBool {
		for token := range(token_dump_dict){
			ExTx.DumpTree(tokenCollector.DumpPath + "/" + token_dump_dict[token])
		}
	}
}

func (tokenCollector *TokenCollector) collectTokenSwapUtil(ExTx *vm.ExternalTx, InTx *vm.InternalTx, token_dump_dict map[string]string){
	var functionSiginature string
	if len(hex.EncodeToString(InTx.Input)) >= 8{
		functionSiginature = "0x" + hex.EncodeToString(InTx.Input)[:8]
	}
	callTo := strings.ToLower(InTx.To.String())
	// if the callTo is related to proxy or stake token, add it 
	if _, ok := tokenCollector.TokenSwapMap[callTo]; ok && functionSiginature == "0x022c0d9f" {
		// fmt.Println(ExTx.BlockNumber, functionSiginature, callTo, tokenSwapMap[callTo], ExTx.InTxs[0].From)
		token_dump_dict[callTo] = tokenCollector.TokenSwapMap[callTo]
	}

	for _, Tx := range InTx.InTxs {
		tokenCollector.collectTokenSwapUtil(ExTx, Tx, token_dump_dict)
	}

	for _, event := range(InTx.Events) {
		tokenCollector.extractSwapEvent(ExTx, event)
	}
}

func (tokenCollector *TokenCollector) extractSwapEvent(ExTx *vm.ExternalTx, event *vm.Event){
	if len(event.Topics) > 0 && event.Topics[0].String() == "0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822" && len(event.Topics) == 3{		
		if _, ok := tokenCollector.TokenMap[event.Address]; ok {
			swapEvent := SwapEvent{
				BlockNumber : ExTx.BlockNumber,
				TxIndex : ExTx.TxIndex,
				EventIndex : event.Index,
				Event : event,
			}
			DumpSwapEvent(swapEvent, "hunter/tokenSwapEvents")
		}
	}
}

type RateMap map[string]float64 // block => rate
type TokenSwap struct {
	Symbol string
	TokenAddress common.Address
	SwapToken string
	RateMap RateMap
}

func LoadRateMap(filePath string) RateMap {
	file, err := os.Open(filePath)
	if err != nil{
		// fmt.Println("Load RateMap err", filePath, err)
	}
	defer file.Close()

	rateMap := make(RateMap)
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&rateMap)
	if err != nil {
		// fmt.Println("json decode err", err)
	}
	return rateMap
}

func LoadRateMapBool(filePath string) (RateMap, bool) {
	flag := true
	file, err := os.Open(filePath)
	if err != nil{
		// fmt.Println("Load RateMap err", filePath, err)
		flag = false
	}
	defer file.Close()

	rateMap := make(RateMap)
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&rateMap)
	if err != nil {
		// fmt.Println("json decode err", err)
		flag = false
	}
	return rateMap, flag
}

type Token struct {
	Symbol string
	Address common.Address
	Decimals int
}

func LoadTokenInfo(filePath string) map[string]Token {
	file, err := os.Open(filePath)
	if err != nil{
		// fmt.Println("Load RateMap err", err)
	}
	defer file.Close()

	tokenMap := make(map[string]Token)
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&tokenMap)
	if err != nil {
		// fmt.Println("json decode err", err)
	}
	return tokenMap
}

func ExistToken(tokenAddress common.Address, tokenSwapMap map[common.Address]TokenSwap, rateMapPath string) (bool) {
	if _, ok := tokenSwapMap[tokenAddress]; ok {
		return true
	} else {
		var rateMap RateMap
		// rateMapPath := "hunter/priceDataETH"
		rateMap, flag := LoadRateMapBool(rateMapPath + "/" + strings.ToLower(tokenAddress.String()) + ".json")
		if flag{
			tokenSwapMap[tokenAddress] = TokenSwap{
				TokenAddress : tokenAddress,
				RateMap : rateMap,
			}
			return true
		} else {
			return false
		}
	}
}

func LoadTokenSwapInfo(filePath string, rateMapPath string) map[common.Address]TokenSwap {
	tokenSwapMap := make(map[common.Address]TokenSwap)
	tokenMap := LoadTokenInfo(filePath)

	var err error
	files, err := ioutil.ReadDir(rateMapPath)
	if err != nil {
		fmt.Println("readDir", err)
	}

	for _ , f := range files[:]{
		temp_list := strings.Split(strings.Split(f.Name(),".")[0], "_")
		atoken := temp_list[0]
		btoken := temp_list[1]
		tokenAddress := tokenMap[atoken].Address
		tokenSwapMap[tokenAddress] = TokenSwap{
			Symbol : atoken,
			TokenAddress : tokenAddress,
			SwapToken : btoken,
			RateMap : LoadRateMap(rateMapPath + "/" + f.Name()),
		}
	}
	return tokenSwapMap
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	if err != nil{
		if os.IsNotExist(err) {
			return false
		}
		return true
	}
	return true
}

type StableToken struct {
	Symbol string
	Decimals int
	XToken common.Address // 0x1 ETH 0x2 USD 0x3 BTC
	RateToXToken int64
}

func LoadStableTokenInfo(path string) map[common.Address]StableToken {

	stableTokenMap := make(map[common.Address]StableToken)
	files, err := ioutil.ReadDir(path)
	if err != nil {
		fmt.Println(err)
	}
	for _ , f := range files{
		var xtoken common.Address
		switch f.Name(){
		case "stableTokenToETH.csv":
			xtoken = common.HexToAddress("0x1") // eth
		case "stableTokenToUSD.csv":
			xtoken = common.HexToAddress("0x2") // usd
		case "stableTokenToBTC.csv":
			xtoken = common.HexToAddress("0x3") // btc
		}
		csvFile, err := os.Open(path + "/" + f.Name())
		if err != nil {
			fmt.Println("Load error", err)
		}
		defer csvFile.Close()
		// Parse the file and continue the first row
		r := csv.NewReader(csvFile)
		_, _ = r.Read()
		// iterate through the records
		for{
			// Read each record from csv
			record, err := r.Read()
			if len(record) == 0 {
				break
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Println("Read Line error", err)
			}
			symbol := strings.ToLower(strings.TrimSpace(record[0]))
			tokenAddress := common.HexToAddress(strings.ToLower(strings.TrimSpace(record[1])))
			rate, _ := strconv.ParseInt(strings.TrimSpace(record[2]), 10, 64)
			decimals, _ := strconv.Atoi(strings.TrimSpace(record[3]))
			stableTokenMap[tokenAddress] = StableToken{
				Symbol : symbol,
				Decimals : decimals,
				XToken : xtoken,
				RateToXToken : rate,
			} 
		}
	}
	return stableTokenMap
}

