package defi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

const ABNORMAL_RATE = 3
const SINGLE_ABNORMAL_RATE = 1.0001

type TokenFlowList []TokenFlow

func (a TokenFlowList) Len() int {
	return len(a)
}

func (a TokenFlowList) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a TokenFlowList) Less(i, j int) bool {
	// from big to small in slice
	// return a[j].BlockNumber.Cmp(a[i].BlockNumber) == -1
	// from small to big
	// return a[i].BlockNumber.Cmp(a[j].BlockNumber) == -1
	if a[i].BlockNumber.Cmp(a[j].BlockNumber) == -1 {
		return true
	} else if a[i].BlockNumber.Cmp(a[j].BlockNumber) == 0 {
		if a[i].TxIndex < a[j].TxIndex {
			return true
		} else if a[i].TxIndex == a[j].TxIndex {
			if a[i].TransferIndex < a[j].TransferIndex {
				return true
			}
		}
	}
	return false
}

type TokenTxList []TokenTx

func (a TokenTxList) Len() int {
	return len(a)
}

func (a TokenTxList) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

// from small to big in slice
func (a TokenTxList) Less(i, j int) bool {
	if a[i].BlockNumber.Cmp(a[j].BlockNumber) == -1 {
		return true
	} else if a[i].BlockNumber.Cmp(a[j].BlockNumber) == 0 {
		if a[i].TxIndex < a[j].TxIndex {
			return true
		} else if a[i].TxIndex == a[j].TxIndex {
			if a[i].TransferIndex < a[j].TransferIndex {
				return true
			}
		}
	}
	return false
}

type UserTokenAttackMap map[common.Address]TokenAttackMap
type TokenAttackMap map[common.Address]AttackInfo

type AttackInfo struct {
	BlockNumber         *big.Int
	TotalDeposit        *big.Float
	TotalWithdraw       *big.Float
	TokenFlowList       []TokenFlow
	RelatedUserMap      map[common.Address]*big.Int
	RelatedTokenFlowMap map[common.Address][]TokenFlow
}

// 0x0000000000000000000000000000000000000000 eth
// 0x0000000000000000000000000000000000000001 stable for eth
// 0x0000000000000000000000000000000000000002 stable for USD
// 0x3 stable for btc

type UserTokenFlowMap map[common.Address]TokenFlowMap
type TokenFlowMap map[common.Address][]TokenFlow

type TokenFlow struct {
	Action        string
	BlockNumber   *big.Int
	TxIndex       int
	TransferIndex uint
	Amount        *big.Float
	TotalDeposit  *big.Float
	TotalWithdraw *big.Float
	Balance       *big.Float
	// TotalDepositMap map[common.Address]*big.Float
	// TotalWithdrawMap map[common.Address]*big.Float
}

type RateRecord struct {
	// userAddress, tokenAddress, totalDeposit, totalWithdraw, rate,
	UserAddress   common.Address
	TokenAddress  common.Address
	TotalDeposit  *big.Float
	TotalWithdraw *big.Float
	IsSingleTx    bool
	BlockNumber   *big.Int
	TxIndex       int
}

type LeakingChecker struct {
	UserTokenFlowMap map[common.Address]TokenFlowMap
	TokenRateMap     map[common.Address][]RateRecord
}

// merge deposit and withdraw in a Tx
type UserTokenFlowTxMap map[common.Address]TokenFlowTxMap
type TokenFlowTxMap map[common.Address]FlowTxMap
type FlowTxMap map[string]TokenFlowTx

type TokenFlowTx struct {
	BlockNumber   *big.Int
	TxIndex       int
	TotalDeposit  *big.Float
	TotalWithdraw *big.Float
}

func GenerateUserFlow(txExtractor *TxExtractor) (UserTokenFlowMap, map[common.Address]int) {

	unSupportToken := make(map[common.Address]int)
	userTokenFlowMap := make(UserTokenFlowMap)
	stableTokenMap := txExtractor.StableTokenMap
	LPTokenMap := txExtractor.LPTokenMap
	tokenSwapMap := txExtractor.TokenSwapMap
	zero, _ := new(big.Int).SetString("0", 10)
	for userAddress, tokenTxMap := range txExtractor.UserTokenTxMap {
		stableTokenListMap := make(map[common.Address][]TokenTx)
		existUnSupportToken := false
		var relatedTokenList []common.Address
		var mergedTokenTxList []TokenTx
		tokenFlowMap := make(TokenFlowMap)
		for tokenAddress := range tokenTxMap {
			// collect stable token for USD and ignore the stake token
			mergedTokenTxList = append(mergedTokenTxList, tokenTxMap[tokenAddress]...)
			relatedTokenList = append(relatedTokenList, tokenAddress)
			if _, ok := stableTokenMap[tokenAddress]; ok {
				xToken := stableTokenMap[tokenAddress].XToken
				stableTokenListMap[xToken] = append(stableTokenListMap[xToken], tokenTxMap[tokenAddress]...)
			} else {
				// ignore the stake token
				if LPTokenMap[tokenAddress] {
					continue
				}
			}

			// generate flow of each token
			var tokenFlowList []TokenFlow
			totalDeposit := new(big.Float)
			totalWithdraw := new(big.Float)
			tokenFlowList = append(tokenFlowList, TokenFlow{
				BlockNumber:   zero,
				TxIndex:       0,
				TransferIndex: 0,
				Amount:        new(big.Float),
				TotalDeposit:  totalDeposit,
				TotalWithdraw: totalWithdraw,
				Balance:       big.NewFloat(float64(0)),
				Action:        "deposit",
			})
			tokenTxList := tokenTxMap[tokenAddress]

			sort.Sort(TokenTxList(tokenTxList))
			for _, tokenTx := range tokenTxList {
				if tokenTx.Amount.Cmp(zero) == 1 {
					amount := new(big.Float).SetInt(tokenTx.Amount)
					if tokenTx.Action == "deposit" || tokenTx.Action == "borrow-in" || tokenTx.Action == "swap-in" {
						totalDeposit = new(big.Float).Add(totalDeposit, amount)
					} else if tokenTx.Action == "withdraw" || tokenTx.Action == "borrow-out" || tokenTx.Action == "swap-out" {
						totalWithdraw = new(big.Float).Add(totalWithdraw, amount)
					}
					var balance *big.Float
					if totalDeposit.Cmp(totalWithdraw) == 1 {
						balance = new(big.Float).Sub(totalDeposit, totalWithdraw)
					} else {
						balance = big.NewFloat(float64(0))
					}
					tokenFlowList = append(tokenFlowList, TokenFlow{
						BlockNumber:   tokenTx.BlockNumber,
						TxIndex:       tokenTx.TxIndex,
						TransferIndex: tokenTx.TransferIndex,
						Amount:        new(big.Float).Add(amount, new(big.Float)),
						Action:        tokenTx.Action,
						TotalDeposit:  totalDeposit,
						TotalWithdraw: totalWithdraw,
						Balance:       balance,
					})
				}
			}
			tokenFlowMap[tokenAddress] = tokenFlowList
		}
		// generate flow of stableToken
		for xToken := range stableTokenListMap {
			stableTokenList := stableTokenListMap[xToken]
			sort.Sort(TokenTxList(stableTokenList))
			var tokenFlowList []TokenFlow
			totalDeposit := new(big.Float)
			totalWithdraw := new(big.Float)
			tokenFlowList = append(tokenFlowList, TokenFlow{
				BlockNumber:   zero,
				TxIndex:       0,
				TransferIndex: 0,
				Amount:        new(big.Float),
				TotalDeposit:  totalDeposit,
				TotalWithdraw: totalWithdraw,
				Balance:       big.NewFloat(float64(0)),
				Action:        "deposit",
			})
			for _, tokenTx := range stableTokenList {
				if tokenTx.Amount.Cmp(zero) == 1 {
					tokenAddress := tokenTx.TokenAddress
					amount := new(big.Float).SetInt(tokenTx.Amount)
					rate := new(big.Float).SetInt64(stableTokenMap[tokenAddress].RateToXToken * int64(math.Pow(float64(10), float64(18-stableTokenMap[tokenAddress].Decimals))))
					amountForXToken := new(big.Float).Mul(amount, rate)
					if tokenTx.Action == "deposit" {
						totalDeposit = new(big.Float).Add(totalDeposit, amountForXToken)
					} else if tokenTx.Action == "withdraw" {
						totalWithdraw = new(big.Float).Add(totalWithdraw, amountForXToken)
					}
					var balance *big.Float
					if totalDeposit.Cmp(totalWithdraw) == 1 {
						balance = new(big.Float).Sub(totalDeposit, totalWithdraw)
					} else {
						balance = big.NewFloat(float64(0))
					}
					tokenFlowList = append(tokenFlowList, TokenFlow{
						BlockNumber:   tokenTx.BlockNumber,
						TxIndex:       tokenTx.TxIndex,
						TransferIndex: tokenTx.TransferIndex,
						Amount:        amountForXToken,
						Action:        tokenTx.Action,
						TotalDeposit:  totalDeposit,
						TotalWithdraw: totalWithdraw,
						Balance:       balance,
					})
				}
			}
			tokenFlowMap[xToken] = tokenFlowList
		}

		// deal with mergedFlow
		sort.Sort(TokenTxList(mergedTokenTxList))
		var mergedFlowList []TokenFlow
		totalDeposit := new(big.Float)
		totalWithdraw := new(big.Float)
		mergedFlowList = append(mergedFlowList, TokenFlow{
			BlockNumber:   zero,
			TxIndex:       0,
			TransferIndex: 0,
			Amount:        new(big.Float),
			TotalDeposit:  totalDeposit,
			TotalWithdraw: totalWithdraw,
			Balance:       big.NewFloat(float64(0)),
			Action:        "deposit",
		})
		var mergedFlowTxList []TokenFlow
		mergedFlowTx := TokenFlow{
			BlockNumber:   zero,
			TxIndex:       0,
			Amount:        new(big.Float),
			TotalDeposit:  totalDeposit,
			TotalWithdraw: totalWithdraw,
			Balance:       big.NewFloat(float64(0)),
			Action:        "deposit",
		}
		for _, tokenTx := range mergedTokenTxList {
			blockNumber := tokenTx.BlockNumber
			if blockNumber.Cmp(mergedFlowTx.BlockNumber) == 1 {
				mergedFlowTxList = append(mergedFlowTxList, mergedFlowTx)
				mergedFlowTx = TokenFlow{
					BlockNumber:   blockNumber,
					TxIndex:       tokenTx.TxIndex,
					Amount:        new(big.Float),
					TotalDeposit:  totalDeposit,
					TotalWithdraw: totalWithdraw,
					Balance:       big.NewFloat(float64(0)),
				}
			}
			isDeposit := false
			isWithdraw := false
			tokenAddress := tokenTx.TokenAddress
			if tokenTx.Amount.Cmp(zero) == 1 {
				amount := new(big.Float).SetInt(tokenTx.Amount)
				var rate *big.Float
				inTokenSwapMap := false
				isETH := tokenAddress == common.HexToAddress("0x0000000000000000000000000000000000000000") || tokenAddress == common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
				if !isETH {
					inTokenSwapMap = ExistToken(tokenAddress, tokenSwapMap, txExtractor.RateMapPath)
				}
				if inTokenSwapMap || isETH {
					if isETH {
						rate = big.NewFloat(float64(1))
					} else {
						var xToken common.Address
						var decimalsRate *big.Float
						// 0x1 ETH 0x2 USD 0x3 BTC
						if _, ok := stableTokenMap[tokenAddress]; ok {
							if stableTokenMap[tokenAddress].XToken == common.HexToAddress("0x2"){
								xToken = common.HexToAddress("0x6b175474e89094c44da98b954eedeac495271d0f")
								ExistToken(xToken, tokenSwapMap, txExtractor.RateMapPath)
								decimalsRate = new(big.Float).SetInt64(int64(math.Pow(float64(10), float64(18-stableTokenMap[tokenAddress].Decimals))))
							} else if stableTokenMap[tokenAddress].XToken == common.HexToAddress("0x3"){
								xToken = common.HexToAddress("0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599")
								ExistToken(xToken, tokenSwapMap, txExtractor.RateMapPath)
								decimalsRate = new(big.Float).SetInt64(int64(math.Pow(float64(10), float64(8-stableTokenMap[tokenAddress].Decimals))))
							}
							amount = new(big.Float).Mul(amount, decimalsRate)
						} else {
							xToken = tokenAddress
						}
						blockNumber := tokenTx.BlockNumber
						block := blockNumber.Uint64()
						block = (block / 500) * 500
						rate = new(big.Float).SetFloat64(tokenSwapMap[xToken].RateMap[strconv.FormatUint(block, 10)])
						// if tokenTx.BlockNumber.Cmp(big.NewInt(int64(12000165))) == 0{
						// 	fmt.Println(tokenAddress, xToken, rate, block)
						// }
					}
					if rate.Cmp(big.NewFloat(float64(0))) == 1 {
						// if blockNumber.Cmp(big.NewInt(int64(10889737))) == 0{
						// 	fmt.Println(blockNumber, tokenAddress, rate)
						// }
						amountForXToken := new(big.Float).Mul(amount, rate)
						if tokenTx.Action == "deposit" {
							isDeposit = true
							totalDeposit = new(big.Float).Add(totalDeposit, amountForXToken)
							mergedFlowTx.Amount = new(big.Float).Add(mergedFlowTx.Amount, amountForXToken)
						} else if tokenTx.Action == "withdraw" {
							isWithdraw = true
							totalWithdraw = new(big.Float).Add(totalWithdraw, amountForXToken)
							mergedFlowTx.Amount = new(big.Float).Sub(mergedFlowTx.Amount, amountForXToken)
						}
						var balance *big.Float
						if totalDeposit.Cmp(totalWithdraw) == 1 {
							balance = new(big.Float).Sub(totalDeposit, totalWithdraw)
						} else {
							balance = big.NewFloat(float64(0))
						}
						mergedFlowList = append(mergedFlowList, TokenFlow{
							BlockNumber:   tokenTx.BlockNumber,
							TxIndex:       tokenTx.TxIndex,
							TransferIndex: tokenTx.TransferIndex,
							Amount:        amountForXToken,
							Action:        tokenTx.Action,
							TotalDeposit:  totalDeposit,
							TotalWithdraw: totalWithdraw,
							Balance:       balance,
						})
						if isDeposit && !strings.Contains(mergedFlowTx.Action, "deposit") {
							mergedFlowTx.Action = mergedFlowTx.Action + "deposit"
						}
						if isWithdraw && !strings.Contains(mergedFlowTx.Action, "withdraw") {
							mergedFlowTx.Action = mergedFlowTx.Action + "withdraw"
						}
						// if mergedFlowTx.Amount.Cmp(big.NewFloat(0)) == 1 {
						// 	mergedFlowTx.Action = "deposit"
						// } else {
						// 	mergedFlowTx.Action = "withdraw"
						// }
						mergedFlowTx.TotalDeposit = new(big.Float).Add(totalDeposit, big.NewFloat(float64(0)))
						mergedFlowTx.TotalWithdraw = new(big.Float).Add(totalWithdraw, big.NewFloat(float64(0)))
						mergedFlowTx.Balance = new(big.Float).Add(balance, big.NewFloat(float64(0)))
					} else {
						unSupportToken[tokenAddress] += 1
						existUnSupportToken = true
					}
				} else {
					unSupportToken[tokenAddress] += 1
					existUnSupportToken = true
				}
			}
		}
		mergedFlowTxList = append(mergedFlowTxList, mergedFlowTx)
		tokenFlowMap[common.HexToAddress("0x4")] = mergedFlowList   // mix
		tokenFlowMap[common.HexToAddress("0x5")] = mergedFlowTxList // mix Tx
		// single token
		// if existUnSupportToken && len(relatedTokenList) == 1 {
		if len(relatedTokenList) == 1 {
			// if userAddress == common.HexToAddress("0xe2307837524Db8961C4541f943598654240bd62f") {
			// 	fmt.Println("check", relatedTokenList, len(tokenTxMap), existUnSupportToken)
			// }
			tokenFlowMap[common.HexToAddress("0x6")] = tokenFlowMap[relatedTokenList[0]]
		} else if existUnSupportToken {
			var tempFlowTxList []TokenFlow
			tokenFlowMap[common.HexToAddress("0x7")] = tempFlowTxList
		}
		userTokenFlowMap[userAddress] = tokenFlowMap
	}
	return userTokenFlowMap, unSupportToken
}

func MergeFlowInTx(userTokenFlowMap UserTokenFlowMap) UserTokenFlowTxMap {
	userTokenFlowTxMap := make(UserTokenFlowTxMap)
	for userAddress := range userTokenFlowMap {
		userTokenFlowTxMap[userAddress] = make(TokenFlowTxMap)
		for tokenAddress := range userTokenFlowMap[userAddress] {
			userTokenFlowTxMap[userAddress][tokenAddress] = make(FlowTxMap)
			for _, tokenFlow := range userTokenFlowMap[userAddress][tokenAddress] {
				block_tx := tokenFlow.BlockNumber.String() + "_" + strconv.Itoa(tokenFlow.TxIndex)
				if _, ok := userTokenFlowTxMap[userAddress][tokenAddress][block_tx]; !ok {
					userTokenFlowTxMap[userAddress][tokenAddress][block_tx] = TokenFlowTx{
						BlockNumber:   tokenFlow.BlockNumber,
						TxIndex:       tokenFlow.TxIndex,
						TotalDeposit:  new(big.Float),
						TotalWithdraw: new(big.Float),
					}
				}
				tokenFlowTx := userTokenFlowTxMap[userAddress][tokenAddress][block_tx]
				if tokenFlow.Action == "deposit" {
					tokenFlowTx.TotalDeposit = tokenFlowTx.TotalDeposit.Add(tokenFlowTx.TotalDeposit, tokenFlow.Amount)
				} else if tokenFlow.Action == "withdraw" {
					tokenFlowTx.TotalWithdraw = tokenFlowTx.TotalWithdraw.Add(tokenFlowTx.TotalWithdraw, tokenFlow.Amount)
				}
			}
		}
	}
	return userTokenFlowTxMap
}

func CheckAttackInTx(userTokenFlowMap UserTokenFlowMap, txExtractor *TxExtractor) {
	zero := new(big.Float)
	userTokenFlowTxMap := MergeFlowInTx(userTokenFlowMap)
	// stableTokenMap := txExtractor.StableTokenMap
	commonAddressMap := txExtractor.CommonAddressMap
	userRelatedMap := txExtractor.UserRelatedMap

	for userAddress := range userTokenFlowTxMap {
		if _, ok := commonAddressMap[userAddress]; ok {
			continue
		}
		if txExtractor.RelatedToken[userAddress] > 0 {
			continue
		}
		for tokenAddress := range userTokenFlowTxMap[userAddress] {
			// if _, ok := stableTokenMap[tokenAddress]; ok {
			// 	continue
			// }
			for block_tx, tokenFlowTx := range userTokenFlowTxMap[userAddress][tokenAddress] {
				if tokenFlowTx.TotalDeposit.Cmp(zero) == 1 {
					usedAddress := make(map[common.Address]bool)
					blockNumber := tokenFlowTx.BlockNumber
					totalDeposit := new(big.Float).Add(tokenFlowTx.TotalDeposit, new(big.Float))
					totalWithdraw := new(big.Float).Add(tokenFlowTx.TotalWithdraw, new(big.Float))
					if _, ok := userRelatedMap[userAddress]; ok {
						for relatedUser := range userRelatedMap[userAddress] {
							if _, ok := commonAddressMap[relatedUser]; ok {
								continue
							}
							if _, ok := usedAddress[relatedUser]; ok {
								continue
							}
							if userRelatedMap[userAddress][relatedUser].Cmp(blockNumber) <= 0 {
								usedAddress[relatedUser] = true
								relatedTotalDeposit, relatedTotalWithdraw := GetRelatedTokenFlowInTx(userTokenFlowTxMap, relatedUser, tokenAddress, userRelatedMap, usedAddress, commonAddressMap, block_tx, blockNumber)
								totalDeposit = totalDeposit.Add(totalDeposit, relatedTotalDeposit)
								totalWithdraw = totalWithdraw.Add(totalWithdraw, relatedTotalWithdraw)
							}
						}
					}
					temp_rate := new(big.Float).Quo(totalWithdraw, totalDeposit)
					rate, _ := temp_rate.Float64()
					if rate > float64(SINGLE_ABNORMAL_RATE) {
						fmt.Println(userAddress, tokenAddress, totalDeposit, totalWithdraw, rate, "within one tx", tokenFlowTx.BlockNumber, tokenFlowTx.TxIndex)
					}
				}
			}
		}
	}
}

func GetRelatedTokenFlowInTx(userTokenFlowTxMap UserTokenFlowTxMap, userAddress, tokenAddress common.Address, userRelatedMap UserRelatedMap, usedAddress, commonAddressMap map[common.Address]bool, block_tx string, blockNumber *big.Int) (*big.Float, *big.Float) {
	totalDeposit := new(big.Float)
	totalWithdraw := new(big.Float)
	if _, ok := userTokenFlowTxMap[userAddress]; ok {
		if _, ok1 := userTokenFlowTxMap[userAddress][tokenAddress]; ok1 {
			if _, ok2 := userTokenFlowTxMap[userAddress][tokenAddress][block_tx]; ok2 {
				totalDeposit = new(big.Float).Add(userTokenFlowTxMap[userAddress][tokenAddress][block_tx].TotalDeposit, new(big.Float))
				totalWithdraw = new(big.Float).Add(userTokenFlowTxMap[userAddress][tokenAddress][block_tx].TotalWithdraw, new(big.Float))
			}
		}
	}
	if _, ok := userRelatedMap[userAddress]; ok {
		for relatedUser := range userRelatedMap[userAddress] {
			if _, ok := commonAddressMap[relatedUser]; ok {
				continue
			}
			if _, ok := usedAddress[relatedUser]; ok {
				continue
			}
			if userRelatedMap[userAddress][relatedUser].Cmp(blockNumber) <= 0 {
				usedAddress[relatedUser] = true
				relatedTotalDeposit, relatedTotalWithdraw := GetRelatedTokenFlowInTx(userTokenFlowTxMap, relatedUser, tokenAddress, userRelatedMap, usedAddress, commonAddressMap, block_tx, blockNumber)
				totalDeposit = totalDeposit.Add(totalDeposit, relatedTotalDeposit)
				totalWithdraw = totalWithdraw.Add(totalWithdraw, relatedTotalWithdraw)
			}
		}
	}
	return totalDeposit, totalWithdraw
}

// -1 means infinity
func calRate(totalDeposit, totalWithdraw *big.Float) (rate float64) {
	zero := new(big.Float)
	if totalDeposit.Cmp(zero) == 1 {
		temp_rate := new(big.Float).Quo(totalWithdraw, totalDeposit)
		rate, _ = temp_rate.Float64()
	} else if totalDeposit.Cmp(zero) == 0 && totalWithdraw.Cmp(zero) == 1 {
		rate = float64(-1)
	}
	return rate
}

func RateLimit(num []float64) float64 {
	var sum, mean, std, limit float64
	sum = float64(0)
	length := len(num)
	for i := 0; i < length; i++ {
		sum += num[i]
	}
	mean = sum / float64(length)
	fmt.Println("The mean of above array is:", mean)
	for j := 0; j < length; j++ {
		std += math.Pow(num[j]-mean, 2)
	}
	std = math.Sqrt(std / float64(length))
	limit = mean + 5*std
	fmt.Println("The std of above array is:", std)
	return limit
}

func (checker *LeakingChecker) AbnormalDetection() {
	// zhiyuan todo
	var allRate []float64
	for tokenAddress := range checker.TokenRateMap {
		for _, rateRecord := range checker.TokenRateMap[tokenAddress] {
			rate_tmp := calRate(rateRecord.TotalDeposit, rateRecord.TotalWithdraw)
			if rate_tmp < 5 && rate_tmp > 1 {
				allRate = append(allRate, rate_tmp)
			}
		}
	}
	rateLimit := RateLimit(allRate)
	fmt.Println("The length of allRate is:", len(allRate))
	fmt.Println("The rateLimit is:", rateLimit)

	for tokenAddress := range checker.TokenRateMap {
		for _, rateRecord := range checker.TokenRateMap[tokenAddress] {
			userAddress := rateRecord.UserAddress
			tokenAddress := rateRecord.TokenAddress
			totalDeposit := rateRecord.TotalDeposit
			totalWithdraw := rateRecord.TotalWithdraw

			rate := calRate(rateRecord.TotalDeposit, rateRecord.TotalWithdraw)
			if rate > float64(1) {
				if rateRecord.IsSingleTx {
					if tokenAddress == common.HexToAddress("0x0000000000000000000000000000000000000005") || tokenAddress == common.HexToAddress("0x0000000000000000000000000000000000000006") {
						fmt.Println(userAddress, tokenAddress, totalDeposit, totalWithdraw, rate, rateRecord.BlockNumber, rateRecord.TxIndex, "within one tx")
					}
				} else if rate > rateLimit {
					if tokenAddress == common.HexToAddress("0x0000000000000000000000000000000000000005") || tokenAddress == common.HexToAddress("0x0000000000000000000000000000000000000006") {
						fmt.Println(userAddress, tokenAddress, totalDeposit, totalWithdraw, rate, rateRecord.BlockNumber, rateRecord.TxIndex, "multiple tx")
					}
				}
			} else if rate == -1 {
				if rateRecord.IsSingleTx {
					if tokenAddress == common.HexToAddress("0x0000000000000000000000000000000000000005") || tokenAddress == common.HexToAddress("0x0000000000000000000000000000000000000006") {
						fmt.Println(userAddress, tokenAddress, totalDeposit, totalWithdraw, rate, rateRecord.BlockNumber, rateRecord.TxIndex, "within one tx")
					}
				} else {
					if tokenAddress == common.HexToAddress("0x0000000000000000000000000000000000000005") || tokenAddress == common.HexToAddress("0x0000000000000000000000000000000000000006") {
						fmt.Println(userAddress, tokenAddress, totalDeposit, totalWithdraw, rate, rateRecord.BlockNumber, rateRecord.TxIndex, "multiple tx")
					}
				}
			}
		}
	}
}

func (checker *LeakingChecker) RecordRate(txExtractor *TxExtractor) {

	userTokenFlowMap := checker.UserTokenFlowMap
	for userAddress := range userTokenFlowMap {

		if txExtractor.RoleMap[userAddress] == "manager"{
			continue
		}
		if txExtractor.CommonAddressMap[userAddress] || txExtractor.RelatedToken[userAddress] != 0 {
			continue
		}

		// remove the case of unSupportToken
		tokenListToCheck := make(map[common.Address]bool)
		if _, ok := userTokenFlowMap[userAddress][common.HexToAddress("0x7")]; ok {
			continue
		} else if _, ok := userTokenFlowMap[userAddress][common.HexToAddress("0x6")]; ok {
			tokenListToCheck[common.HexToAddress("0x6")] = true // single token
		} else {
			tokenListToCheck[common.HexToAddress("0x5")] = true // mix token
		}
		var existUnSupportToken bool

		userRelatedMap := txExtractor.UserRelatedMap
		commonAddressMap := txExtractor.CommonAddressMap
		stableTokenMap := txExtractor.StableTokenMap

		firstDeposit := TokenFlow{
			BlockNumber: new(big.Int),
			TxIndex:     0,
		}

		if _, ok := commonAddressMap[userAddress]; ok {
			return
		}

		for tokenAddress := range tokenListToCheck{
			if _, ok := userTokenFlowMap[userAddress]; !ok {
				continue
			}
			if _, ok := stableTokenMap[tokenAddress]; ok {
				continue
			}
			tokenFlowList := userTokenFlowMap[userAddress][tokenAddress]
			sort.Sort(TokenFlowList(tokenFlowList))
			lastBalance := big.NewFloat(float64(0))
			for index := range tokenFlowList {
				if index == 0{
					continue
				}
				tokenFlow := tokenFlowList[index]
				if (firstDeposit.BlockNumber.Cmp(big.NewInt(int64(0))) == 0) || (lastBalance.Cmp(big.NewFloat(float64(0))) == 0 && strings.Contains(tokenFlow.Action, "deposit")) {
					firstDeposit = tokenFlow
				}
				if !(strings.Contains(tokenFlow.Action, "withdraw")) {
					continue
				}
				totalDeposit := new(big.Float).Add(tokenFlow.TotalDeposit, new(big.Float))
				totalWithdraw := new(big.Float).Add(tokenFlow.TotalWithdraw, new(big.Float))
				blockNumber := tokenFlow.BlockNumber
				usedAddress := make(map[common.Address]bool)

				// add related users' token flow
				if _, ok := userRelatedMap[userAddress]; ok {
					for relatedUser := range userRelatedMap[userAddress] {
						if _, ok := commonAddressMap[relatedUser]; ok {
							continue
						}
						if _, ok := usedAddress[relatedUser]; ok {
							continue
						}
						if userRelatedMap[userAddress][relatedUser].Cmp(blockNumber) <= 0 {
							// fmt.Println("relatedUser", relatedUser)
							// if _, ok := userTokenFlowMap[relatedUser]; ok {
							usedAddress[relatedUser] = true
							relatedTotalDeposit, relatedTotalWithdraw, relatedFirstDeposit, existUnSupportTokenFlag := GetRelatedTokenFlow(userTokenFlowMap, relatedUser, tokenAddress, userRelatedMap, usedAddress, commonAddressMap, tokenFlow)
							if relatedFirstDeposit.BlockNumber.Cmp(firstDeposit.BlockNumber) == -1 || (relatedFirstDeposit.BlockNumber.Cmp(firstDeposit.BlockNumber) == 0 && relatedFirstDeposit.TxIndex < firstDeposit.TxIndex){
								firstDeposit.BlockNumber = new(big.Int).Add(relatedFirstDeposit.BlockNumber, new(big.Int))
								firstDeposit.TxIndex = relatedFirstDeposit.TxIndex
							}
							if existUnSupportTokenFlag {
								existUnSupportToken = true
								break
							}
							totalDeposit = totalDeposit.Add(totalDeposit, relatedTotalDeposit)
							totalWithdraw = totalWithdraw.Add(totalWithdraw, relatedTotalWithdraw)
						}
					}
				}
				isSingleTx := false
				if tokenFlow.BlockNumber.Cmp(firstDeposit.BlockNumber) == 0 && tokenFlow.TxIndex == firstDeposit.TxIndex {
					isSingleTx = true
				}
				if !existUnSupportToken {
					checker.TokenRateMap[tokenAddress] = append(checker.TokenRateMap[tokenAddress], RateRecord{
						UserAddress:   userAddress,
						TokenAddress:  tokenAddress,
						TotalDeposit:  new(big.Float).Add(totalDeposit, big.NewFloat(float64(0))),
						TotalWithdraw: new(big.Float).Add(totalWithdraw, big.NewFloat(float64(0))),
						IsSingleTx:    isSingleTx,
						BlockNumber:   new(big.Int).Add(tokenFlow.BlockNumber, big.NewInt(int64(0))),
						TxIndex:       tokenFlow.TxIndex,
					})
				}
				lastBalance = new(big.Float).Add(tokenFlow.Balance, big.NewFloat(float64(0)))
			}
		}
	}
}

func abnormalDetection(totalDeposit, totalWithdraw *big.Float, userTokenFlowMap UserTokenFlowMap, userAddress, tokenAddress common.Address, usedAddress map[common.Address]bool, userRelatedMap UserRelatedMap, isSingleTx bool, thisTokenFlow TokenFlow) (AttackInfo, bool) {

	attackInfo := AttackInfo{}
	flag := false
	zero := new(big.Float)
	if totalDeposit.Cmp(zero) == 1 {
		temp_rate := new(big.Float).Quo(totalWithdraw, totalDeposit)
		rate, _ := temp_rate.Float64()
		if isSingleTx {
			if rate >= float64(SINGLE_ABNORMAL_RATE) {
				fmt.Println(userAddress, tokenAddress, totalDeposit, totalWithdraw, rate, thisTokenFlow.BlockNumber, thisTokenFlow.TxIndex, "within one tx")
				flag = true
			}
		} else if rate >= float64(ABNORMAL_RATE) {
			fmt.Println(userAddress, tokenAddress, totalDeposit, totalWithdraw, rate, thisTokenFlow.BlockNumber, thisTokenFlow.TxIndex, "multiple tx")
			flag = true
		}
	} else if totalDeposit.Cmp(zero) == 0 && totalWithdraw.Cmp(zero) == 1 {
		fmt.Println(userAddress, tokenAddress, totalDeposit, totalWithdraw)
		flag = true
	}
	if flag {
		attackInfo = AttackInfo{
			BlockNumber:         thisTokenFlow.BlockNumber,
			TotalDeposit:        totalDeposit,
			TotalWithdraw:       totalWithdraw,
			TokenFlowList:       userTokenFlowMap[userAddress][tokenAddress],
			RelatedUserMap:      userRelatedMap[userAddress],
			RelatedTokenFlowMap: make(map[common.Address][]TokenFlow),
		}
		for relatedUser := range usedAddress {
			attackInfo.RelatedTokenFlowMap[relatedUser] = userTokenFlowMap[relatedUser][tokenAddress]
		}
	}
	return attackInfo, flag
}

func CheckAttack(userAddress common.Address, userTokenFlowMap UserTokenFlowMap, txExtractor *TxExtractor) (map[common.Address]AttackInfo, bool) {

	userRelatedMap := txExtractor.UserRelatedMap
	commonAddressMap := txExtractor.CommonAddressMap
	stableTokenMap := txExtractor.StableTokenMap
	attackInfoMap := make(map[common.Address]AttackInfo)

	flag := false
	firstDeposit := TokenFlow{
		BlockNumber: new(big.Int),
		TxIndex:     0,
	}
	if _, ok := commonAddressMap[userAddress]; ok {
		return attackInfoMap, flag
	}

	// mode selection
	// remove the cases with unSupportToken
	tokenListToCheck := make(map[common.Address]bool)
	if _, ok := userTokenFlowMap[userAddress][common.HexToAddress("0x7")]; ok {
		return attackInfoMap, flag
	} else if _, ok := userTokenFlowMap[userAddress][common.HexToAddress("0x6")]; ok {
		tokenListToCheck[common.HexToAddress("0x6")] = true // single token
	} else {
		tokenListToCheck[common.HexToAddress("0x5")] = true // mix token
	}
	var existUnSupportToken bool

	for tokenAddress := range tokenListToCheck {
		if _, ok := userTokenFlowMap[userAddress]; !ok {
			continue
		}
		if _, ok := stableTokenMap[tokenAddress]; ok {
			continue
		}
		tokenFlowList := userTokenFlowMap[userAddress][tokenAddress]
		sort.Sort(TokenFlowList(tokenFlowList))
		lastBalance := big.NewFloat(float64(0))
		for index := range tokenFlowList {
			if index == 0{
				// jump out {deposit 0 0 0 0 0 0 0}
				continue
			}
			tokenFlow := tokenFlowList[index]
			totalDeposit := new(big.Float).Add(tokenFlow.TotalDeposit, new(big.Float))
			totalWithdraw := new(big.Float).Add(tokenFlow.TotalWithdraw, new(big.Float))
			blockNumber := tokenFlow.BlockNumber
			usedAddress := make(map[common.Address]bool)
			if (firstDeposit.BlockNumber.Cmp(big.NewInt(int64(0))) == 0) || (lastBalance.Cmp(big.NewFloat(float64(1000000000000000000))) == -1 && strings.Contains(tokenFlow.Action, "deposit")) {
				firstDeposit = tokenFlow
			}
			if !(strings.Contains(tokenFlow.Action, "withdraw")) {
				continue
			}
			// add related users' token flow
			if _, ok := userRelatedMap[userAddress]; ok {
				for relatedUser := range userRelatedMap[userAddress] {
					if _, ok := commonAddressMap[relatedUser]; ok {
						continue
					}
					if _, ok := usedAddress[relatedUser]; ok {
						continue
					}
					if userRelatedMap[userAddress][relatedUser].Cmp(blockNumber) <= 0 {
						// if _, ok := userTokenFlowMap[relatedUser]; ok {
						usedAddress[relatedUser] = true
						relatedTotalDeposit, relatedTotalWithdraw, relatedFirstDeposit, existUnSupportTokenFlag := GetRelatedTokenFlow(userTokenFlowMap, relatedUser, tokenAddress, userRelatedMap, usedAddress, commonAddressMap, tokenFlow)
						if relatedFirstDeposit.BlockNumber.Cmp(firstDeposit.BlockNumber) == -1 || (relatedFirstDeposit.BlockNumber.Cmp(firstDeposit.BlockNumber) == 0 && relatedFirstDeposit.TxIndex < firstDeposit.TxIndex){
							firstDeposit.BlockNumber = new(big.Int).Add(relatedFirstDeposit.BlockNumber, new(big.Int))
							firstDeposit.TxIndex = relatedFirstDeposit.TxIndex
						}
						if existUnSupportTokenFlag {
							existUnSupportToken = true
							break
						}
						totalDeposit = totalDeposit.Add(totalDeposit, relatedTotalDeposit)
						totalWithdraw = totalWithdraw.Add(totalWithdraw, relatedTotalWithdraw)
					}
				}
			}
			// if userAddress == common.HexToAddress("0xdfb6fab7f4bc9512d5620e679e90d1c91c4eade6"){
			// 	fmt.Println("check Attack", totalDeposit, totalWithdraw, firstDeposit, existUnSupportToken)
			// }
			
			var isSingleTx bool
			if tokenFlow.BlockNumber.Cmp(firstDeposit.BlockNumber) == 0 && tokenFlow.TxIndex == firstDeposit.TxIndex {
				isSingleTx = true
			}
			if !existUnSupportToken {
				// fmt.Println(userAddress, tokenAddress, totalDeposit, totalWithdraw, isSingleTx, tokenFlow.BlockNumber, firstDeposit.BlockNumber, existUnSupportToken)
				attackInfo, isAttack := abnormalDetection(totalDeposit, totalWithdraw, userTokenFlowMap, userAddress, tokenAddress, usedAddress, userRelatedMap, isSingleTx, tokenFlow)
				if isAttack {
					attackInfoMap[tokenAddress] = attackInfo
					flag = true
					break
				}
			}
			lastBalance = new(big.Float).Add(tokenFlow.Balance, big.NewFloat(float64(0)))
		}
	}
	return attackInfoMap, flag
}

func GetRelatedTokenFlow(userTokenFlowMap UserTokenFlowMap, userAddress, tokenAddress common.Address, userRelatedMap UserRelatedMap, usedAddress, commonAddressMap map[common.Address]bool, thisTokenFlow TokenFlow) (totalDeposit *big.Float, totalWithdraw *big.Float, firstDeposit TokenFlow, existUnSupportToken bool) {
	// remove the cases with unSupportToken

	totalDeposit = new(big.Float)
	totalWithdraw = new(big.Float)
	firstDeposit = TokenFlow{
		BlockNumber: new(big.Int),
		TxIndex:     0,
	}
	if _, ok := userTokenFlowMap[userAddress][common.HexToAddress("0x7")]; ok {
		return totalDeposit, totalWithdraw, firstDeposit, true
	}

	if tokenAddress == common.HexToAddress("0x6"){
		if _, ok := userTokenFlowMap[userAddress][common.HexToAddress("0x6")]; !ok && len(userTokenFlowMap[userAddress]) > 0 {
			// fmt.Println("Test", userTokenFlowMap[userAddress][common.HexToAddress("0x6")])
			return totalDeposit, totalWithdraw, firstDeposit, true
		}
	}

	tokenFlowList := userTokenFlowMap[userAddress][tokenAddress]
	sort.Sort(TokenFlowList(tokenFlowList))

	lastBalance := big.NewFloat(float64(0))
	for index := range tokenFlowList {
		if index == 0{
			// jump out {deposit 0 0 0 0 0 0 0}
			continue
		}
		tokenFlow := tokenFlowList[index]
		if (firstDeposit.BlockNumber.Cmp(big.NewInt(int64(0))) == 0) || (lastBalance.Cmp(big.NewFloat(float64(1000000000000000000))) == -1 && strings.Contains(tokenFlow.Action, "deposit")) {
			firstDeposit = tokenFlow
		}
		if tokenFlow.BlockNumber.Cmp(thisTokenFlow.BlockNumber) == 1 || (tokenFlow.BlockNumber.Cmp(thisTokenFlow.BlockNumber) == 0 && tokenFlow.TxIndex > thisTokenFlow.TxIndex) {
			break
		}
		// if tokenFlow.Action == "deposit"{
		// 	lastDeposit = new(big.Int).Add(tokenFlow.BlockNumber,new(big.Int))
		// }
		totalDeposit = new(big.Float).Add(tokenFlow.TotalDeposit, new(big.Float))
		totalWithdraw = new(big.Float).Add(tokenFlow.TotalWithdraw, new(big.Float))
		lastBalance = new(big.Float).Add(tokenFlow.Balance, big.NewFloat(float64(0)))
	}
	if _, ok := userRelatedMap[userAddress]; ok {
		for relatedUser := range userRelatedMap[userAddress] {
			if _, ok := commonAddressMap[relatedUser]; ok {
				continue
			}
			if _, ok := usedAddress[relatedUser]; ok {
				continue
			}
			if userRelatedMap[userAddress][relatedUser].Cmp(thisTokenFlow.BlockNumber) <= 0 {
				// if _, ok := userTokenFlowMap[relatedUser]; ok {
				usedAddress[relatedUser] = true
				relatedTotalDeposit, relatedTotalWithdraw, relatedFirstDeposit, existUnSupportTokenFlag := GetRelatedTokenFlow(userTokenFlowMap, relatedUser, tokenAddress, userRelatedMap, usedAddress, commonAddressMap, thisTokenFlow)
				// update related user's firstDeposit
				if relatedFirstDeposit.BlockNumber.Cmp(firstDeposit.BlockNumber) == -1 || (relatedFirstDeposit.BlockNumber.Cmp(firstDeposit.BlockNumber) == 0 && relatedFirstDeposit.TxIndex < firstDeposit.TxIndex){
					firstDeposit.BlockNumber = new(big.Int).Add(relatedFirstDeposit.BlockNumber, new(big.Int))
					firstDeposit.TxIndex = relatedFirstDeposit.TxIndex
				}
				if existUnSupportToken {
					return totalDeposit, totalWithdraw, firstDeposit, existUnSupportTokenFlag
				}
				totalDeposit = totalDeposit.Add(totalDeposit, relatedTotalDeposit)
				totalWithdraw = totalWithdraw.Add(totalWithdraw, relatedTotalWithdraw)
			}
		}
	}
	return totalDeposit, totalWithdraw, firstDeposit, existUnSupportToken
}

type TestResult struct {
	UserTokenAttackMap UserTokenAttackMap
	ExistAttack        bool
	TotalDuration      string
	TestDuration       string
	TestStartBlock     uint64
}

func DumpTestOutput(testResult TestResult, path string) {
	os.Mkdir(path, os.ModePerm)
	var err error
	b, _ := json.Marshal(testResult)
	err = ioutil.WriteFile(path+"/"+"result.json", b, 0644)
	if err != nil {
		fmt.Println(err)
	}
}
