package fuzz

import (
	"strconv"
	"math/big"
	"encoding/json"
	// "encoding/hex"
	"io/ioutil"
	"os"
	"fmt"
	"strings"
	// "reflect"

	types "github.com/ethereum/go-ethereum/core/types"
	common "github.com/ethereum/go-ethereum/common"
)

type MsgCollector struct {
	ProxyMap map[string]string
	StakeMap map[string]string
}

type MessageToDump struct {
	To         *common.Address
	From       common.Address
	Nonce      uint64
	Amount     *big.Int
	GasLimit   uint64
	GasPrice   *big.Int
	GasFeeCap  *big.Int
	GasTipCap  *big.Int
	Data       []byte
	AccessList types.AccessList
	IsFake     bool
}

func dumpMsgToJson(msg types.Message, path string){
	var err error
	msgToDump := MessageToDump{
		To         : msg.To(),
		From       : msg.From(),
		Nonce      : msg.Nonce(),
		Amount     : msg.Value(),
		GasLimit   : msg.Gas(),
		GasPrice   : msg.GasPrice(),
		GasFeeCap  : msg.GasFeeCap(),
		GasTipCap  : msg.GasTipCap(),
		Data       : msg.Data(),
		AccessList : msg.AccessList(),
		IsFake     : msg.IsFake(),
	}
	b, _ := json.Marshal(msgToDump)
	err = ioutil.WriteFile(path, b, 0644)
	if err != nil{
		fmt.Println(err)
	}
}

func DumpMsgList(msgList []types.Message, path string){
	os.Mkdir(path, os.ModePerm)
	for index, msg := range msgList{
		dumpMsgToJson(msg, path + "/" + strconv.Itoa(index) + ".json")
	}
}

func (msgCollector *MsgCollector) CollectMsg(msg types.Message, blockNumber *big.Int, txIndex int){
	
	callTo := strings.ToLower((*msg.To()).String())
	if _, ok := msgCollector.ProxyMap[callTo]; ok {
		name := msgCollector.ProxyMap[callTo]
		dumpMsgToJson(msg, "hunter/seed_and_env/" + name + "/msg/proxy/" + blockNumber.String() + "_" + strconv.Itoa(txIndex) + ".json")
	}
	if _, ok := msgCollector.StakeMap[callTo]; ok {
		name := msgCollector.ProxyMap[callTo]
		dumpMsgToJson(msg, "hunter/seed_and_env/" + name + "/msg/stake/" + blockNumber.String() + "_" + strconv.Itoa(txIndex) + ".json")
	}
}

func loadMsg(path string) types.Message {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println("LoadTx error:", err)
	}
	temp_msg := MessageToDump{}
	err = json.Unmarshal([]byte(file), &temp_msg)
	if err != nil {
		fmt.Println("Json to struct error:",err)
	}
	
	// fmt.Println(temp_msg)
	msg := types.NewMessage(
		temp_msg.From,
		temp_msg.To,
		temp_msg.Nonce,
		temp_msg.Amount,
		temp_msg.GasLimit,
		temp_msg.GasPrice,
		temp_msg.GasFeeCap,
		temp_msg.GasTipCap,
		temp_msg.Data,
		temp_msg.AccessList,
		temp_msg.IsFake,
	)
	return msg
}

func LoadMsgs(path string, block uint64) ([]types.Message){
	var msgList []types.Message
	proxy_tx_path := path + "/proxy/"
	files, _ := ioutil.ReadDir(proxy_tx_path)
	for _ , f := range files[:]{
		temp_list := strings.Split(strings.Split(f.Name(),".")[0], "_")
		b, _ := strconv.ParseUint(temp_list[0],10,64)
		if b < block{
			msgList = append(msgList, loadMsg(proxy_tx_path + f.Name()))
		} else {
			break
		}
	}
	return msgList
}