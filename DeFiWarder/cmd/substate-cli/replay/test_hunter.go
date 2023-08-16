package replay

import (
	"fmt"
	"math/big"
	"strconv"
	"time"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/research"
	cli "gopkg.in/urfave/cli.v1"

	// for su
	defi "github.com/ethereum/go-ethereum/hunter/defi"
	fuzz "github.com/ethereum/go-ethereum/hunter/fuzz"
	types "github.com/ethereum/go-ethereum/core/types"
	// "crypto/rand"
)

var (
	SomeEther    *big.Int
)

// record-replay: substate-cli replay command
var TestCommandSu = cli.Command{
	Action:    TestActionSu,
	Name:      "test-hunter",
	Usage:     "test starting on selected block number",
	ArgsUsage: "<blockNumFirst> <blockNumLast>",
	Flags: []cli.Flag{
		research.WorkersFlag,
		research.SkipCallTxsFlag,
		research.SubstateDirFlag,
	},
	Description: `
The substate-cli replay command requires two arguments:
<blockNumFirst> <blockNumLast>

<blockNumFirst> and <blockNumLast> are the first and
last block of the inclusive range of blocks to replay transactions.`,
}

var stableTokenMap map[common.Address]defi.StableToken
var proxyInfoMap map[string]defi.ProxyInfo

// record-replay: func replayAction for replay command
func TestActionSu(ctx *cli.Context) error {
	var err error

	if len(ctx.Args()) != 3 {
		return fmt.Errorf("substate-cli test-hunter command requires exactly 3 arguments")
	}

	dapp := strings.TrimSpace(ctx.Args().Get(0))
	first, ferr := strconv.ParseInt(ctx.Args().Get(1), 10, 64)
	round, rerr := strconv.ParseInt(ctx.Args().Get(2), 10, 64)
	if ferr != nil || rerr != nil {
		return fmt.Errorf("substate-cli test-hunter: error in parsing parameters: test block number and round not an integer")
	}
	if first < 0 {
		return fmt.Errorf("substate-cli test-hunter: error: test block number must be greater than 0")
	}
	if round <= 0 {
		return fmt.Errorf("substate-cli test-hunter: error: test round of test must be greater than 0")
	}

	fmt.Println("start test-hunter")
	// print or not
	// load the defi info
	fmt.Println("load defi info")
	proxyInfoMap = defi.LoadDefi("hunter/defi.csv", false, true)
	if _, ok := proxyInfoMap[dapp]; !ok {
		return fmt.Errorf("substate-cli test-hunter: error in parsing parameters: dapp (" + dapp + ") not in proxyInfoMap")
	}
	
	stableTokenMap = defi.LoadStableTokenInfo("hunter/stableToken")
	SomeEther, _ = new(big.Int).SetString("1000000000000000000000", 10)
	// fmt.Println(proxyMap, stakeMap)

	research.SetSubstateFlags(ctx)
	research.OpenSubstateDBReadOnly()
	defer research.CloseSubstateDB()

	taskPool := research.NewSubstateTaskPoolForTest("substate-cli test-hunter", TestTask, uint64(first), uint(round), dapp, ctx)
	
	// create result dir
	os.Mkdir("hunter/result", os.ModePerm)
	y,m,d := taskPool.StartTime.Date()
	hour,min,sec := taskPool.StartTime.Clock()
	timeString := fmt.Sprintf("%d-%d-%d_%d-%d-%d",y,m,d,hour,min,sec)
	taskPool.ResultPath = "hunter/result/" + taskPool.Dapp + "_" + timeString
	os.Mkdir(taskPool.ResultPath , os.ModePerm)

	err = taskPool.ExecuteTest()
	return err
}

func TestTask(round uint, substate *research.Substate, taskPool *research.SubstateTaskPoolForTest) (bool, error) {

	// inputAlloc := substate.InputAlloc
	inputEnv := substate.Env
	inputMessage := substate.Message

	startTime := time.Now()
	block := taskPool.StartBlock
	dapp := taskPool.Dapp
	var testResult defi.TestResult
	testResult.TestStartBlock = block

	txExtractor := defi.TxExtractor{
		Dapp: dapp,
		ActionMap: defi.LoadActionMap("hunter/defi/ActionMap.json"),
		ProxyInfo : proxyInfoMap[dapp],
		UserTokenTxMap : make(defi.UserTokenTxMap),
		UserRelatedMap : make(defi.UserRelatedMap),
		AddressCountMap : make(map[common.Address]int),
		ActionInfoList : make(map[int]defi.ActionInfo),
		MethodCountMap : make(map[string]uint),
	}
	txExtractor.Init("hunter/defi_apps")
	// fmt.Println("Test starts at block", block)

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
			// hashError = fmt.Errorf("getHash(%d) invoked, blockhash for that block not provided", num)
		}
		return h
	}

	// load env
	epoch := 1000000
	limit := 50
	timeLimit := float64(1800)
	existAttack := false

	var allocs []research.SubstateAlloc
	allocs = fuzz.LoadAllocs("hunter/seed_and_env/" + dapp + "/statedb", block) 
	// fmt.Println(len(allocs))

	// Apply Message
	var (
		// statedb   = MakeOffTheChainStateDB(inputAlloc)
		statedb = MakeOffTheChainStateDBWithTxs(allocs)
		gaspool   = new(core.GasPool)
		txHash    = common.Hash{0x02}
		txIndex   = 0
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
	tracer, err := getTracerFn(txIndex, txHash)
	if err != nil {
		return existAttack ,err
	}
	vmConfig.Tracer = tracer
	vmConfig.Debug = (tracer != nil)

	statedb.Prepare(txHash, txIndex)

	txCtx := vm.TxContext{
		GasPrice: msg.GasPrice(),
		Origin:   msg.From(),
	}

	// load front msgs
	// fmt.Println("load front msgs")
	// oldMsgList := fuzz.LoadMsgs("hunter/seed_and_env/" + dapp + "/msg", block)
	oldTxs := fuzz.LoadTxs("hunter/defi_apps/" + dapp, block)

	// init seed
	seed := new(fuzz.Seed)
	proxyInfo := txExtractor.ProxyInfo
	seed.InitSeed(&proxyInfo)
	// seed.InitSeedFromFrontMsg(oldMsgList)
	seed.InitSeedFromFrontTxs(oldTxs)

	// for m := range seed.Contracts[proxyInfo.ProxyContract].MethodMsgMap{
	// 	fmt.Println(m)
	// }

	// init tokenUserTxMap and userRelatedMap
	// fmt.Println("init tokenUserTxMap and userRelatedMap ")
	defi.SetCallVisibility(false)
	// defi.InitTokenUserTxMap(block, txExtractor, "hunter/defi_apps/")
	// defi.InitUserRelatedMapWithStake(block, txExtractor, "hunter/defi_apps/")
	source := "hunter"
	startBlock := uint64(10000000)
	endBlock := block
	defi.InitTokenUserTxMap(startBlock, endBlock, txExtractor, source + "/defi_apps/")
	defi.InitUserRelatedMapWithStake(startBlock, endBlock, txExtractor, source + "/defi_apps/")
	txExtractor.UpdateCommonAddressMap()
	txExtractor.ExtractActionInfoList()

	// init token flow
	init_userTokenTxMap := defi.DeepCpUserTokenTxMap(txExtractor.UserTokenTxMap) 
	init_userRelatedMap := defi.DeepCpUserRelatedMap(txExtractor.UserRelatedMap)
	// fmt.Println(commonAddressMap)

	// init check
	userTokenFlowMap, _ := defi.GenerateUserFlow(&txExtractor)
	for userAddress := range(userTokenFlowMap){
		_, existAttack = defi.CheckAttack(userAddress, userTokenFlowMap, &txExtractor)
	}

	if existAttack{
		return existAttack, nil
	}
	
	// set the test round and steps for each round
	defi.SetCallVisibility(false)
	defi.SetEventVisibility(false)
	testStartTime := time.Now()

	fmt.Println("Finish init, start fuzzing")
	tokenAddress := common.HexToAddress("0x6B175474E89094C44Da98b954EedeAC495271d0F")
	statedb = MakeOffTheChainStateDBWithTxs(allocs)
	for i := 0; i < epoch; i++ {
		if time.Since(testStartTime).Seconds() > timeLimit{
			break
		}
		// fmt.Printf("Epoch %d\n",i)
		// select attacker 
		attacker, attacker_index := seed.SelectSender()

		// reset tokenTxMap
		txExtractor.ActionInfoList = make(map[int]defi.ActionInfo)
		txExtractor.UserTokenTxMap = defi.DeepCpUserTokenTxMap(init_userTokenTxMap)
		txExtractor.UserRelatedMap = defi.DeepCpUserRelatedMap(init_userRelatedMap)
		attacker = common.HexToAddress("0x66441289c5185637b35bcd3df89d3d200ee5c76c")
		fmt.Println("Attacker" , attacker)
		fmt.Println("before", len(txExtractor.UserTokenTxMap[attacker][tokenAddress]), len(txExtractor.ActionInfoList))

		// var msgList []types.Message
		// for i := 0; i < 10; i++{
		// 	transfer_msg := seed.GenerateMsgWithMethod(attacker, proxyInfo.ProxyContract, "transfer")
		// 	msgList = append(msgList, transfer_msg)
		// }
		// for i := 0; i < 10; i++{
		// 	burn_msg := seed.GenerateMsgWithMethod(attacker, proxyInfo.ProxyContract, "burn")
		// 	msgList = append(msgList, burn_msg)
		// }

		// reset EVM
		evm := vm.NewEVM(blockCtx, txCtx, statedb, chainConfig, vmConfig)
	
		var recordMsgList []types.Message
		// for step, msg := range(msgList[:]) {
		for step := 0; step < limit; step++ {

			toContract := seed.SelectTo()
			msg := seed.GenerateMsg(attacker, toContract)

			// add some ethers to the From of msg, ensuring enough balnace to buy gas
			evm.StateDB.AddBalance(msg.From(), SomeEther)
			
			evm.ExTx.BlockNumber = new(big.Int).SetUint64(inputEnv.Number + uint64(step))
			evm.ExTx.TxIndex = 0
			gaspool.AddGas(inputEnv.GasLimit)
			msgResult, err := core.ApplyMessageTest(evm, msg, gaspool)
			// fmt.Println(msgResult)
			if err != nil {
				fmt.Println(msgResult, err)
			} else if msgResult.Err == nil {
				recordMsgList = append(recordMsgList, msg)
				// defi.SetCallVisibility(true)
				// defi.SetEventVisibility(true)
				// defi.ParseTxAndDump(evm.ExTx, false)
				// evm.ExTx.ParseTxTree()
				txExtractor.ExtractTokenTx(evm.ExTx)
				txExtractor.ExtractStakeTokenTx(evm.ExTx)
			}
			
			// fmt.Println(step, msg.From(), *msg.To(), hex.EncodeToString(msg.Data()))

			evm.ExTx = new(vm.ExternalTx)
		}
		if len(recordMsgList) == 0{
			seed.RemoveSender(attacker_index)
		}

		txExtractor.ExtractActionInfoList()
		fmt.Println("after", len(txExtractor.UserTokenTxMap[attacker][tokenAddress]),len(txExtractor.ActionInfoList), len(recordMsgList))
		userTokenFlowMap, _ = defi.GenerateUserFlow(&txExtractor)

		userTokenAttackMap := make(defi.UserTokenAttackMap)
		for userAddress := range(userTokenFlowMap){
			tokenAttackMap, isAttack := defi.CheckAttack(userAddress, userTokenFlowMap, &txExtractor)
			if isAttack{
				userTokenAttackMap[userAddress] = tokenAttackMap
				existAttack = true
			}
		}
		
		if existAttack{
			fmt.Printf("Find attack at round %d\n", round)
			testResult.UserTokenAttackMap = userTokenAttackMap
			testResult.ExistAttack = true
			fuzz.DumpMsgList(recordMsgList, taskPool.ResultPath + "/" + strconv.Itoa(int(round)) + "_msgs")
			break
		}
		// fmt.Println("--------------------------------------------------------------")
	}

	testResult.TotalDuration = (time.Since(startTime) + 1*time.Nanosecond).String()
	testResult.TestDuration = (time.Since(testStartTime) + 1*time.Nanosecond).String()
	defi.DumpTestOutput(testResult, taskPool.ResultPath +  "/" + strconv.Itoa(int(round)) + "_result")

	if hashError != nil {
		return existAttack, hashError
	}

	return existAttack, nil
}

// var msgList []types.Message
// transfer_msg := seed.GenerateMsg(seed.StakeABI, "transfer", common.HexToAddress("0x66441289c5185637b35bcd3df89d3d200ee5c76c"), proxyInfo.ProxyContract)
// msgList = append(msgList, transfer_msg)
// msgList = append(msgList, transfer_msg)
// msgList = append(msgList, transfer_msg)
// burn_msg := seed.GenerateMsg(seed.StakeABI, "burn", common.HexToAddress("0x66441289c5185637b35bcd3df89d3d200ee5c76c"), proxyInfo.StakeContract)
// msgList = append(msgList, burn_msg)
// msgList = append(msgList, burn_msg)
// msgList = append(msgList, burn_msg)
// msgList = append(msgList, burn_msg)
// msgList = append(msgList, burn_msg)
// msgList = append(msgList, burn_msg)
// msgList = append(msgList, burn_msg)
// msgList = append(msgList, burn_msg)
// msgList = append(msgList, burn_msg)
// msgList = append(msgList, burn_msg)
// msgList = append(msgList, burn_msg)
// msgList = append(msgList, burn_msg)
// msgList = append(msgList, burn_msg)

// go run cmd/substate-cli/main.go test-su 10847339 1  --substatedir /home/nvme2/substate.ethereum8-15/substate.ethereum/ --skip-create-txs --skip-transfer-txs --workers 1