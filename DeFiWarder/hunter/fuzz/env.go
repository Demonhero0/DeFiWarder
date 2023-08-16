package fuzz

import (
	"strconv"
	"encoding/json"
	"io/ioutil"
	"fmt"
	"strings"

	vm "github.com/ethereum/go-ethereum/core/vm"
	research "github.com/ethereum/go-ethereum/research"
)

type EnvCollector struct {
	ProxyMap map[string]string
	StakeMap map[string]string
}

func (envCollector *EnvCollector) CollectEnv(ExTx *vm.ExternalTx, inputAlloc research.SubstateAlloc, outputAlloc research.SubstateAlloc, dumpBool bool){

	var err error
	proxy_dump_dict := make(map[string]string)
	stake_dump_dict := make(map[string]string)
	// fmt.Println("Sender", ExTx.InTxs[0].From)
	if len(ExTx.InTxs) == 1{
		envCollector.ParseTxTreeUtil(ExTx.InTxs[0], 0, proxy_dump_dict, stake_dump_dict)
	}
	// fmt.Println(proxy_dump_dict)
	if dumpBool {
		for proxy := range(proxy_dump_dict){
			// fmt.Println(inputAlloc)
			// b, _ := json.Marshal(inputAlloc)
			// err = ioutil.WriteFile("hunter/seed_and_env/" + proxy_dump_dict[proxy] + "/statedb/input/" + ExTx.BlockNumber.String() + "_" + strconv.Itoa(ExTx.TxIndex) + ".json", b, 0644)
			b, _ := json.Marshal(outputAlloc)
			err = ioutil.WriteFile("hunter/seed_and_env/" + proxy_dump_dict[proxy] + "/statedb/output/" + ExTx.BlockNumber.String() + "_" + strconv.Itoa(ExTx.TxIndex) + ".json", b, 0644)
			if err != nil{
				fmt.Println(err)
			}
		}
		for stake := range(stake_dump_dict){
			// b, _ := json.Marshal(inputAlloc)
			// ioutil.WriteFile("hunter/seed_and_env/" + proxy_dump_dict[stake] + "/statedb/input/" + ExTx.BlockNumber.String() + "_" + strconv.Itoa(ExTx.TxIndex) + ".json", b, 0644)
			b, _ := json.Marshal(outputAlloc)
			ioutil.WriteFile("hunter/seed_and_env/" + proxy_dump_dict[stake] + "/statedb/output/" + ExTx.BlockNumber.String() + "_" + strconv.Itoa(ExTx.TxIndex) + ".json", b, 0644)
		}
	}
}

func (envCollector *EnvCollector) ParseTxTreeUtil(InTx *vm.InternalTx, depth int, proxy_dump_dict,stake_dump_dict map[string]string){

	callTo := strings.ToLower(InTx.To.String())
	// if the callTo is related to proxy or stake token, add it 
	if _, ok := envCollector.ProxyMap[callTo]; ok {
		proxy_dump_dict[callTo] = envCollector.ProxyMap[callTo]
	}

	if _, ok := envCollector.StakeMap[callTo]; ok && InTx.CallType != "StaticCall" {
		stake_dump_dict[callTo] = envCollector.StakeMap[callTo]
	}
	
	// the event before
	for _, Tx := range InTx.InTxs {
		envCollector.ParseTxTreeUtil(Tx, depth+1, proxy_dump_dict, stake_dump_dict)
	}

	// for _, event := range(InTx.Events) {
	// 	envCollector.parseEvent(event, depth, proxy_dump_dict, stake_dump_dict)
	// }
}

// func (envCollector *EnvCollector) parseEvent(event *vm.Event, depth int, proxy_dump_dict, stake_dump_dict map[string]string) {

// 	// fmt.Println(strings.Repeat("-",depth+2), depth+1, "event", event.Index)
// 	// identify erc20 transfer
// 	if len(event.Topics) > 0 && event.Topics[0].String() == "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" && len(event.Topics) == 3{		
// 		parsed, err := abi.JSON(strings.NewReader(erc20string))
// 		if err != nil {
// 			fmt.Println("Parser error", err)
// 		}
// 		res, err := parsed.Unpack("Transfer", event.Data)
// 		if err != nil {
// 			fmt.Println("Parser error", err)
// 		}
// 		// fmt.Println(res,err)
// 		amount := res[0]
// 		sender := strings.ToLower(common.HexToAddress(event.Topics[1].String()).String())
// 		to := strings.ToLower(common.HexToAddress(event.Topics[2].String()).String())
// 		if printEvent {
// 			fmt.Println(strings.Repeat("-",depth+2), depth+1, "event", event.Address, "Transfer from", sender, "to", to, "amount", amount)
// 		}
// 		// if the sender or to is related to proxy, add it
// 		if _, ok := envCollector.ProxyMap[sender]; ok {
// 			proxy_dump_dict[sender] = envCollector.ProxyMap[sender]
// 		} else if _, ok := envCollector.ProxyMap[to]; ok {
// 			proxy_dump_dict[to] = envCollector.ProxyMap[to]
// 		}
// 	}
// }

func loadAlloc(path string) (alloc research.SubstateAlloc) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println("LoadTx error:", err)
	}
	err = json.Unmarshal([]byte(file), &alloc)
	if err != nil {
		fmt.Println("Json to struct error:",err)
	}
	return alloc
}

func LoadAllocs(path string, block uint64) (allocs []research.SubstateAlloc) {
	input_path := path + "/output/"
	files, _ := ioutil.ReadDir(input_path)
	for _ , f := range files[:]{
		temp_list := strings.Split(strings.Split(f.Name(),".")[0], "_")
		b, _ := strconv.ParseUint(temp_list[0],10,64)
		if b < block{
			allocs = append(allocs, loadAlloc(input_path + f.Name()))
		} else{
			break
		}
	}
	return allocs
}