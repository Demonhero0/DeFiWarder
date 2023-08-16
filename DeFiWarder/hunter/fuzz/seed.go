package fuzz

import (
	"math/big"
	// "encoding/hex"
	"io/ioutil"
	"os"
	"fmt"
	"strings"
	// "reflect"
	"strconv"
	cryptoRand "crypto/rand"

	types "github.com/ethereum/go-ethereum/core/types"
	common "github.com/ethereum/go-ethereum/common"
	abi "github.com/ethereum/go-ethereum/accounts/abi"
	defi "github.com/ethereum/go-ethereum/hunter/defi"
	vm "github.com/ethereum/go-ethereum/core/vm"
)

func loadABIJSON(path string) abi.ABI {
	jsonFile, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
	}	
	defer jsonFile.Close()
	abiByte, _ := ioutil.ReadAll(jsonFile)
	temp_abi, _ := abi.JSON(strings.NewReader(string(abiByte)))
	return temp_abi
}

type Input []byte
type Contract struct {
	Abi abi.ABI
	MethodCallMap map[string][]Input
	MethodList []string
}

type Seed struct {
	SenderSeedMap map[common.Address]uint
	SenderSeedList []common.Address
	ToSeedMap map[common.Address]uint
	ToSeedList []common.Address
	Contracts map[common.Address]*Contract
	// TypeMap map[byte]uint

	// build seed for each sender
	SeedPoolMap map[common.Address]SeedPool
}

func (seed *Seed) initContract(contractAddress common.Address, name string){
	seed.Contracts[contractAddress] = &Contract{
		Abi : loadABIJSON("hunter/defi_apps/" + name + "/" + strings.ToLower(contractAddress.String()) + ".json"),
		MethodCallMap: make(map[string][]Input),
	}

	// mList := [3]string{"transfer", "burn", "mint"}
	// mList := [1]string{"withdraw"}
	// for _, m := range mList{
	// 	seed.Contracts[contractAddress].MethodList = append(seed.Contracts[contractAddress].MethodList, m)
	// }
	for key, m := range seed.Contracts[contractAddress].Abi.Methods {
		if m.StateMutability != "view" {
			seed.Contracts[contractAddress].MethodList = append(seed.Contracts[contractAddress].MethodList, key)
		}
	}
	// fmt.Println("MethodList", seed.Contracts[contractAddress].MethodList)
}

func (seed *Seed) InitSeed(proxyInfo *defi.ProxyInfo){
	// init
	seed.SenderSeedMap = make(map[common.Address]uint)
	seed.ToSeedMap = make(map[common.Address]uint)
	seed.SeedPoolMap = make(map[common.Address]SeedPool)
	// seed.TypeMap = make(map[byte]uint)

	// load proxy abi
	seed.Contracts = make(map[common.Address]*Contract)
	for _, proxy := range proxyInfo.ProxyContracts{
		seed.initContract(proxy, proxyInfo.Name)
		seed.ToSeedList = append(seed.ToSeedList, proxy)
	}
	// load stake abi
	for _, token := range proxyInfo.LPTokens{
		if token != common.HexToAddress("0") {
			if _, ok := seed.Contracts[token]; !ok{
				seed.initContract(token, proxyInfo.Name)
				seed.ToSeedList = append(seed.ToSeedList, token)
			}
		}
	}
}

func (seed *Seed) RemoveSender(index int64){
	seed.SenderSeedList = append(seed.SenderSeedList[:index], seed.SenderSeedList[index+1:]...)
	fmt.Println("remove sender, exist", len(seed.SenderSeedList))
}

func (seed *Seed) GenerateMsgWithMethod(from common.Address, to common.Address, name string) types.Message {
	var msg types.Message
	var accessList types.AccessList
	var data []byte
	msg = types.NewMessage(
		from, 		// from       common.Address
		&to, 		// to         *common.Address
		uint64(0),	// nonce      uint64
		new(big.Int).SetInt64(0), // amount     *big.Int
		uint64(8000000),		// gasLimit   uint64
		new(big.Int).SetInt64(350000000000),	// gasPrice   *big.Int
		new(big.Int).SetInt64(350000000000),	// gasFeeCap  *big.Int
		new(big.Int).SetInt64(350000000000),	// gasTipCap  *big.Int
		data, // data []byte
		accessList, // accessList AccessList
		false) // isFake     bool

	data = seed.generateInputData(seed.Contracts[to], name, &msg)

	msg.SetData(data)
	return msg
}

func (seed *Seed) GenerateMsg(from common.Address, to common.Address) types.Message {
	var msg types.Message
	var accessList types.AccessList
	var data []byte
	n, _ := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(len(seed.Contracts[to].MethodList))))
	name := seed.Contracts[to].MethodList[n.Int64()]
	msgValue := new(big.Int)
	seedPool := seed.SeedPoolMap[from]
	if seed.Contracts[to].Abi.Methods[name].Payable{
		msgValue = seedPool.SelectEtherAmount()
	}
	msg = types.NewMessage(
		from, 		// from       common.Address
		&to, 		// to         *common.Address
		uint64(0),	// nonce      uint64
		msgValue, // amount     *big.Int
		uint64(8000000),		// gasLimit   uint64
		new(big.Int).SetInt64(350000000000),	// gasPrice   *big.Int
		new(big.Int).SetInt64(350000000000),	// gasFeeCap  *big.Int
		new(big.Int).SetInt64(350000000000),	// gasTipCap  *big.Int
		data, // data []byte
		accessList, // accessList AccessList
		false) // isFake     bool

	if _, ok := seed.Contracts[to].MethodCallMap[name]; ok {
	// if false {
		n, _ := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(len(seed.Contracts[to].MethodCallMap[name]))))
		data = seed.Contracts[to].MethodCallMap[name][n.Int64()]
		data = seed.mutateInputData(seed.Contracts[to], name, data, &msg)
	} else {
		data = seed.generateInputData(seed.Contracts[to], name, &msg)
	}
	msg.SetData(data)
	// fmt.Println(name, data)

	// parse data
	// fmt.Println(name, msg.Data(), data)
	// if len(msg.Data()) >= 4{
	// 	args, _ := seed.Contracts[to].Abi.Methods[name].Inputs.Unpack(msg.Data()[4:])
	// 	for index, input := range seed.Contracts[to].Abi.Methods[name].Inputs{
	// 		fmt.Println(input.Name, args[index])
	// 	}
	// }
	return msg
}

func (seed *Seed) mutateInputData(contract *Contract, name string, data []byte, msg *types.Message) []byte {
	// fmt.Println("Mutate", name, data)

	var newData []byte
	var err error
	var method abi.Method
	seedPool := seed.SeedPoolMap[msg.From()]
	method = contract.Abi.Methods[name]

	originArgs ,err := method.Inputs.Unpack(data[4:])
	if err != nil {
		fmt.Println(err)
	}
	var newArgs []interface{}
	for index, input := range contract.Abi.Methods[name].Inputs{
		// fmt.Println(arg.Type.T, args[index])
		switch input.Type.T {
			case abi.IntTy:
				newArgs = append(newArgs, seedPool.generateTypeInput(input.Type, msg))
			case abi.UintTy:
				newArgs = append(newArgs, seedPool.generateTypeInput(input.Type, msg))
			case abi.AddressTy:
				newArgs = append(newArgs, seedPool.generateTypeInput(input.Type, msg))
			case abi.BoolTy:
				newArgs = append(newArgs, seedPool.generateTypeInput(input.Type, msg))
			default:
				newArgs = append(newArgs, originArgs[index])
		}
	}
	// fmt.Println(newArgs)
	newData, err = contract.Abi.Pack(name, newArgs...)
	if err != nil{
		fmt.Println("Mutate error")
	}

	return newData
}


func (seed *Seed) generateInputData(contract *Contract, name string, msg *types.Message) []byte {

	var data []byte
	var args []interface{}
	seedPool := seed.SeedPoolMap[msg.From()]
	// fmt.Println(name, contract.Abi.Methods[name].Inputs)
	for _, input := range contract.Abi.Methods[name].Inputs{
		arg := seedPool.generateTypeInput(input.Type, msg)
		args = append(args, arg)
	}
	
	// fmt.Println("Generate", name, args)
	data, err := contract.Abi.Pack(name, args...)
	if err != nil {
		fmt.Println(err)
	}
	// fmt.Println(name, args, hex.EncodeToString((data)))
	return data
}

func (seed *Seed) SelectSender() (common.Address, int64) {
	n, _ := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(len(seed.SenderSeedList))))
	return seed.SenderSeedList[n.Int64()], n.Int64()
}

func (seed *Seed) SelectTo() common.Address {
	n, _ := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(len(seed.ToSeedList))))
	return seed.ToSeedList[n.Int64()]
}

func LoadTxs(path string, block uint64) []vm.ExternalTx {
	var TxList []vm.ExternalTx
	proxy_tx_path := path + "/historyTx/proxy/"
	files, _ := ioutil.ReadDir(proxy_tx_path)
	for _ , f := range files[:]{
		temp_list := strings.Split(strings.Split(f.Name(),".")[0], "_")
		b, _ := strconv.ParseUint(temp_list[0],10,64)
		if b <= block{
			TxList = append(TxList, vm.LoadTx(proxy_tx_path + f.Name()))
		} else {
			continue
		}
	}
	return TxList
}

func (seed *Seed) InitSeedFromFrontTxs(TxList []vm.ExternalTx){

	// from old txs
	for _, tx := range TxList[:]{
		seed.ParseTxForSeed(&tx)
	}
	for sender, seedPool := range seed.SeedPoolMap{
		seed.SeedPoolMap[sender] = seedPool.updateMapToList()
	}
	seed.updateMapToList()
}

func (seed *Seed) ParseTxForSeed(ExTx *vm.ExternalTx){

	if len(ExTx.InTxs) == 1{
		seed.SenderSeedMap[ExTx.InTxs[0].From] += 1
		var seedPool SeedPool
		if _, ok := seed.SeedPoolMap[ExTx.InTxs[0].From]; !ok{
			seedPool = SeedPool{}
			seedPool.initSeedPool()
		} else {
			seedPool = seed.SeedPoolMap[ExTx.InTxs[0].From]
		}
		seed.ParseTxForSeedUtil(ExTx.InTxs[0], &seedPool)
		seed.SeedPoolMap[ExTx.InTxs[0].From] = seedPool
	}
}

func (seed *Seed) ParseTxForSeedUtil(InTx *vm.InternalTx, seedPool *SeedPool){

	callTo := InTx.To

	if _, ok := seed.Contracts[callTo]; ok {
		seedPool.parseMethodArgs(seed.Contracts[callTo], InTx.Input)
		seedPool.EtherAmountSeedMap[InTx.Value.String()] += 1
	}
	
	for _, Tx := range InTx.InTxs {
		seed.ParseTxForSeedUtil(Tx, seedPool)
	}
}

func (seed *Seed) InitSeedFromFrontMsg(msgList []types.Message){

	// from old msg
	// fmt.Println(msgList[424])
	// userMsgMap := make(map[common.Address][]types.Message)
	for _, msg := range msgList[:]{
		if _, ok := seed.Contracts[*msg.To()]; ok {
			var seedPool SeedPool
			if _, ok1 := seed.SeedPoolMap[*msg.To()]; !ok1{
				seedPool = SeedPool{}
				seedPool.initSeedPool()
			} else {
				seedPool = seed.SeedPoolMap[*msg.To()]
			}
			seedPool.parseMethodArgs(seed.Contracts[*msg.To()], msg.Data())
			seed.SenderSeedMap[msg.From()] += 1
			seed.ToSeedMap[*msg.To()] += 1
			// userMsgMap[msg.From()] = append(userMsgMap[msg.From()], msg)
			seed.SeedPoolMap[*msg.To()] = seedPool
		}
	}
	for _, seedPool := range seed.SeedPoolMap{
		seedPool.updateMapToList()
	}
	seed.updateMapToList()
}

func (seed *Seed) updateMapToList(){
	for key, _ := range(seed.SenderSeedMap){
		seed.SenderSeedList = append(seed.SenderSeedList, key)
	}
	// for key, _ := range(seed.ToSeedMap){
	// 	seed.ToSeedList = append(seed.ToSeedList, key)
	// }
}