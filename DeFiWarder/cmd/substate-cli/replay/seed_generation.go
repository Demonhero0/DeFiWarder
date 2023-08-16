package replay

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/research"
	cli "gopkg.in/urfave/cli.v1"

	// for su
	defi "github.com/ethereum/go-ethereum/hunter/defi"
	fuzz "github.com/ethereum/go-ethereum/hunter/fuzz"
)

var envCollector fuzz.EnvCollector
var msgCollector fuzz.MsgCollector

// record-replay: substate-cli replay command
var SeedGenerationCommand = cli.Command{
	Action:    SeedGenerationAction,
	Name:      "seed-generation",
	Usage:     "collect related msg",
	ArgsUsage: "<blockNumFirst> <blockNumLast>",
	Flags: []cli.Flag{
		research.WorkersFlag,
		research.SkipTransferTxsFlag,
		research.SkipCallTxsFlag,
		research.SkipCreateTxsFlag,
		research.SubstateDirFlag,
	},
	Description: `
The substate-cli replay command requires two arguments:
<blockNumFirst> <blockNumLast>

<blockNumFirst> and <blockNumLast> are the first and
last block of the inclusive range of blocks to replay transactions.`,
}

// record-replay: func replayAction for replay command
func SeedGenerationAction(ctx *cli.Context) error {
	var err error

	if len(ctx.Args()) != 2 {
		return fmt.Errorf("substate-cli replay command requires exactly 2 arguments")
	}

	first, ferr := strconv.ParseInt(ctx.Args().Get(0), 10, 64)
	last, lerr := strconv.ParseInt(ctx.Args().Get(1), 10, 64)

	if ferr != nil || lerr != nil {
		return fmt.Errorf("substate-cli replay: error in parsing parameters: block number not an integer")
	}
	if first < 0 || last < 0 {
		return fmt.Errorf("substate-cli replay: error: block number must be greater than 0")
	}
	if first > last {
		return fmt.Errorf("substate-cli replay: error: first block has larger number than last block")
	}

	fmt.Println("start seed generation")
	// print or not
	// load the defi info
	fmt.Println("load defi info")
	proxyInfoMap := defi.LoadDefi("hunter/defi.csv", false, true)
	defi.SetCallVisibility(false)

	envCollector.ProxyMap = make(map[string]string)
	envCollector.StakeMap = make(map[string]string)
	msgCollector.ProxyMap = make(map[string]string)
	msgCollector.StakeMap = make(map[string]string)
	for dapp := range proxyInfoMap{
		for _, proxy := range proxyInfoMap[dapp].ProxyContracts{
			envCollector.ProxyMap[strings.ToLower(proxy.String())] = dapp
		}
		for _, token := range proxyInfoMap[dapp].LPTokens{
			envCollector.StakeMap[strings.ToLower(token.String())] = dapp
		}
		// msgCollector.ProxyMap[strings.ToLower(proxyInfoMap[dapp].ProxyContract.String())] = dapp
		// msgCollector.StakeMap[strings.ToLower(proxyInfoMap[dapp].StakeContract.String())] = dapp
	}

	research.SetSubstateFlags(ctx)
	research.OpenSubstateDBReadOnly()
	defer research.CloseSubstateDB()

	taskPool := research.NewSubstateTaskPool("substate-cli seed-generation", SeedGenerationTask, uint64(first), uint64(last), ctx)
	err = taskPool.Execute()
	return err
}

func SeedGenerationTask(block uint64, tx int, substate *research.Substate, taskPool *research.SubstateTaskPool) error {
	inputAlloc := substate.InputAlloc
	inputEnv := substate.Env
	inputMessage := substate.Message

	outputAlloc := substate.OutputAlloc
	outputResult := substate.Result

	var (
		vmConfig    vm.Config
		chainConfig *params.ChainConfig
		getTracerFn func(txIndex int, txHash common.Hash) (tracer vm.EVMLogger, err error)
	)

	vmConfig = vm.Config{}

	chainConfig = &params.ChainConfig{}
	*chainConfig = *params.MainnetChainConfig
	// disable DAOForkSupport, otherwise account states will be overwritten
	chainConfig.DAOForkSupport = false

	getTracerFn = func(txIndex int, txHash common.Hash) (tracer vm.EVMLogger, err error) {
		return nil, nil
	}

	var hashError error
	getHash := func(num uint64) common.Hash {
		if inputEnv.BlockHashes == nil {
			hashError = fmt.Errorf("getHash(%d) invoked, no blockhashes provided", num)
			return common.Hash{}
		}
		h, ok := inputEnv.BlockHashes[num]
		if !ok {
			hashError = fmt.Errorf("getHash(%d) invoked, blockhash for that block not provided", num)
		}
		return h
	}

	// Apply Message
	var (
		statedb   = MakeOffTheChainStateDB(inputAlloc)
		gaspool   = new(core.GasPool)
		blockHash = common.Hash{0x01}
		txHash    = common.Hash{0x02}
		txIndex   = tx
	)

	gaspool.AddGas(inputEnv.GasLimit)
	blockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		Coinbase:    inputEnv.Coinbase,
		BlockNumber: new(big.Int).SetUint64(inputEnv.Number),
		Time:        new(big.Int).SetUint64(inputEnv.Timestamp),
		Difficulty:  inputEnv.Difficulty,
		GasLimit:    inputEnv.GasLimit,
		GetHash:     getHash,
	}
	// If currentBaseFee is defined, add it to the vmContext.
	if inputEnv.BaseFee != nil {
		blockCtx.BaseFee = new(big.Int).Set(inputEnv.BaseFee)
	}

	msg := inputMessage.AsMessage()

	// msgCollector.CollectMsg(msg, new(big.Int).SetUint64(inputEnv.Number), txIndex)

	tracer, err := getTracerFn(txIndex, txHash)
	if err != nil {
		return err
	}
	vmConfig.Tracer = tracer
	vmConfig.Debug = (tracer != nil)
	statedb.Prepare(txHash, txIndex)

	txCtx := vm.TxContext{
		GasPrice: msg.GasPrice(),
		Origin:   msg.From(),
	}
	
	evm := vm.NewEVM(blockCtx, txCtx, statedb, chainConfig, vmConfig)
	evm.ExTx.BlockNumber = new(big.Int).SetUint64(inputEnv.Number)
	evm.ExTx.TxIndex = txIndex
	evm.ExTx.Timestamp = new(big.Int).SetUint64(inputEnv.Timestamp)

	snapshot := statedb.Snapshot()
	msgResult, err := core.ApplyMessage(evm, msg, gaspool)
	envCollector.CollectEnv(evm.ExTx, inputAlloc, outputAlloc, true)

	if err != nil {
		statedb.RevertToSnapshot(snapshot)
		return err
	}

	if hashError != nil {
		return hashError
	}

	if chainConfig.IsByzantium(blockCtx.BlockNumber) {
		statedb.Finalise(true)
	} else {
		statedb.IntermediateRoot(chainConfig.IsEIP158(blockCtx.BlockNumber))
	}

	evmResult := &research.SubstateResult{}
	if msgResult.Failed() {
		evmResult.Status = types.ReceiptStatusFailed
	} else {
		evmResult.Status = types.ReceiptStatusSuccessful
	}
	evmResult.Logs = statedb.GetLogs(txHash, blockHash)
	evmResult.Bloom = types.BytesToBloom(types.LogsBloom(evmResult.Logs))
	if to := msg.To(); to == nil {
		evmResult.ContractAddress = crypto.CreateAddress(evm.TxContext.Origin, msg.Nonce())
	}
	evmResult.GasUsed = msgResult.UsedGas

	evmAlloc := statedb.ResearchPostAlloc

	r := outputResult.Equal(evmResult)
	a := outputAlloc.Equal(evmAlloc)
	if !(r && a) {
		if !r {
			fmt.Printf("inconsistent output: result\n")
		}
		if !a {
			fmt.Printf("inconsistent output: alloc\n")
		}
		var jbytes []byte
		jbytes, _ = json.MarshalIndent(inputAlloc, "", " ")
		fmt.Printf("inputAlloc:\n%s\n", jbytes)
		jbytes, _ = json.MarshalIndent(inputEnv, "", " ")
		fmt.Printf("inputEnv:\n%s\n", jbytes)
		jbytes, _ = json.MarshalIndent(inputMessage, "", " ")
		fmt.Printf("inputMessage:\n%s\n", jbytes)
		jbytes, _ = json.MarshalIndent(outputAlloc, "", " ")
		fmt.Printf("outputAlloc:\n%s\n", jbytes)
		jbytes, _ = json.MarshalIndent(evmAlloc, "", " ")
		fmt.Printf("evmAlloc:\n%s\n", jbytes)
		jbytes, _ = json.MarshalIndent(outputResult, "", " ")
		fmt.Printf("outputResult:\n%s\n", jbytes)
		jbytes, _ = json.MarshalIndent(evmResult, "", " ")
		fmt.Printf("evmResult:\n%s\n", jbytes)
		return fmt.Errorf("inconsistent output")
	}

	return nil
}