package defi

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	vm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

type TxCollector struct {
	AddressMap map[string]string
}

// find related txs and dump json files
func (txCollector *TxCollector) ParseTxAndDump(ExTx *vm.ExternalTx, dumpBool bool){
	address_dump_dict := make(map[string]string)

	if printCall{
		fmt.Println("Sender", ExTx.InTxs[0].From)
	}
	if len(ExTx.InTxs) == 1{
		txCollector.ParseTxTreeUtil(ExTx.InTxs[0], 0, address_dump_dict)
	}
	// fmt.Println(ExTx.InTxs[0].From)
	if dumpBool {
		for proxy := range(address_dump_dict){
			ExTx.DumpTree("hunter/defi_apps/" + address_dump_dict[proxy] + "/historyTx/" + proxy)
		}
		// for stake := range(stake_dump_dict){
		// 	fmt.Println(ExTx, stake, stake_dump_dict[stake])
		// 	ExTx.DumpTree("hunter/defi_apps/" + stake_dump_dict[stake] + "/historyTx/" + stake)
		// }
	}
}

func (txCollector *TxCollector) ParseTxTreeUtil(InTx *vm.InternalTx, depth int, address_dump_dict map[string]string){
	// printCall
	if printCall{
		var functionSiginature string
		if len(hex.EncodeToString(InTx.Input)) >= 8{
			functionSiginature = "0x" + hex.EncodeToString(InTx.Input)[:8]
		}
		if InTx.Value != nil {
			fmt.Println(strings.Repeat("-",depth+1),depth, InTx.CallType, InTx.To, functionSiginature, InTx.Value)
		} else {
			fmt.Println(strings.Repeat("-",depth+1),depth, InTx.CallType, InTx.To, functionSiginature)
		}
	}
	if InTx.From == common.HexToAddress("0xaed9fdc9681d61edb5f8b8e421f5cee8d7f4b04f"){
		fmt.Println("create 0xaed9fdc9681d61edb5f8b8e421f5cee8d7f4b04f")
	}
	callTo := strings.ToLower(InTx.To.String())
	// if the callTo is related to proxy or stake token, add it 
	if _, ok := txCollector.AddressMap[callTo]; ok && InTx.CallType != "StaticCall" {
		address_dump_dict[callTo] = txCollector.AddressMap[callTo]
	}

	// if _, ok := txCollector.StakeMap[callTo]; ok && InTx.CallType != "StaticCall" {
	// 	stake_dump_dict[callTo] = txCollector.StakeMap[callTo]
	// }
	
	// the event before
	for _, Tx := range InTx.InTxs {
		txCollector.ParseTxTreeUtil(Tx, depth+1, address_dump_dict)
	}

	for _, event := range(InTx.Events) {
		txCollector.parseEvent(event, depth, address_dump_dict)
	}
}

func (txCollector *TxCollector) parseEvent(event *vm.Event, depth int, address_dump_dict map[string]string) {

	// fmt.Println(strings.Repeat("-",depth+2), depth+1, "event", event.Index)
	// identify erc20 transfer
	if len(event.Topics) > 0 && event.Topics[0].String() == "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" && len(event.Topics) == 3{		
		parsed, err := abi.JSON(strings.NewReader(erc20string))
		if err != nil {
			fmt.Println("Parser error", err)
		}
		res, err := parsed.Unpack("Transfer", event.Data)
		if err != nil {
			fmt.Println("Parser error", err)
		}
		// fmt.Println(res,err)
		amount := res[0]
		sender := strings.ToLower(common.HexToAddress(event.Topics[1].String()).String())
		to := strings.ToLower(common.HexToAddress(event.Topics[2].String()).String())
		if printEvent {
			fmt.Println(strings.Repeat("-",depth+2), depth+1, "event", event.Address, "Transfer from", sender, "to", to, "amount", amount)
		}
		// if the sender or to is related to proxy, add it
		if _, ok := txCollector.AddressMap[sender]; ok {
			address_dump_dict[sender] = txCollector.AddressMap[sender]
		} else if _, ok := txCollector.AddressMap[to]; ok {
			address_dump_dict[to] = txCollector.AddressMap[to]
		}
	}
}