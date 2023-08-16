package vm

import (
	"math/big"
	"encoding/json"
	"strconv"
	"fmt"
	"io/ioutil"
	"strings"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/common"
)

type Event struct {
	Address common.Address
	Topics []common.Hash
	Data []byte
	Index uint
}

type InternalTx struct {
	From common.Address
	To common.Address

	IsContract bool
	Value *big.Int
	CallType string // Call, StaticCall, CallCode, DelegateCall, Create
	Input []byte
	Events []*Event
	InTxs []*InternalTx
}

type ExternalTx struct {
	BlockNumber *big.Int 
	Timestamp *big.Int
	TxIndex int
	InTxs []*InternalTx
}

func (ExTx *ExternalTx) DumpTree(dumpPath string){
	b, err := json.Marshal(*ExTx)
	if err != nil{
		fmt.Println("TxDump", err)
	}
	ioutil.WriteFile(dumpPath + "/" + ExTx.BlockNumber.String() + "_" + strconv.Itoa(ExTx.TxIndex) + ".json", b, 0644)
}

func LoadTx(path string) ExternalTx {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println("LoadTx error:", err)
	}
	ExTx := ExternalTx{}
	err = json.Unmarshal([]byte(file), &ExTx)
	if err != nil {
		fmt.Println("Json to struct error:",err)
	}
	return ExTx
}

func (ExTx *ExternalTx) ParseTxTree(){
	fmt.Println(ExTx.BlockNumber, ExTx.Timestamp, ExTx.TxIndex)
	fmt.Println("Sender", ExTx.InTxs[0].From)
	parseTxTreeUtil(ExTx.InTxs[0], 0)
}

func parseEvent(event *Event, depth int) {
	fmt.Println(strings.Repeat("-",depth+2), depth+1, "event", event.Index)
}

func parseTxTreeUtil(InTx *InternalTx, depth int){
	// fmt.Println(InTxs)
	if len(InTx.Input) >= 4 {
		fmt.Println(strings.Repeat("-",depth+1),depth, InTx.CallType, InTx.To, "0x"+hex.EncodeToString(InTx.Input[:4]))
	} else {
		fmt.Println(strings.Repeat("-",depth+1),depth, InTx.CallType, InTx.To)
	}
	// if the callTo is related to proxy or stake token, add it 
	// the event before
	// thisEventIndex := 0 
	for _, Tx := range InTx.InTxs {

		parseTxTreeUtil(Tx, depth+1)
	}

	for _, event := range(InTx.Events) {
		parseEvent(event, depth)
	}
}