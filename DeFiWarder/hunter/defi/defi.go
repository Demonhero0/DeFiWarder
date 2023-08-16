package defi

import (
	"encoding/csv"
	"os"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"math/big"

	common "github.com/ethereum/go-ethereum/common"
	abi "github.com/ethereum/go-ethereum/accounts/abi"
)

var printCall bool
var printEvent bool

type ProxyInfo struct {
	Name string
	CreateAt *big.Int
	Deposit map[string]bool
	Withdraw map[string]bool
	ProxyContracts []common.Address
	LPTokens []common.Address
}

var erc20ABI abi.ABI
var erc20string string

func LoadDefi(filePath string, createDataset bool, createSeed bool) (map[string]ProxyInfo) {

	proxyInfoMap := make(map[string]ProxyInfo)

	csvFile, err := os.Open(filePath)
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
		proxyName := strings.TrimSpace(strings.ToLower(record[0]))

		createAt, _ := new(big.Int).SetString(strings.TrimSpace(record[1]), 10)
		proxyAddresses := strings.Split(strings.ToLower(strings.TrimSpace(record[4])), ";")
		var proxyContracts []common.Address
		for _, address := range proxyAddresses{
			proxyContracts = append(proxyContracts, common.HexToAddress(address))
		}
		temp_proxyInfo := ProxyInfo{
			Name : proxyName,
			CreateAt : createAt,
			Deposit : make(map[string]bool), 
			Withdraw : make(map[string]bool),
			ProxyContracts : proxyContracts,
		}
		for _, siginature := range strings.Split(strings.ToLower(strings.TrimSpace(record[2])), ";"){
			temp_proxyInfo.Deposit[siginature] = true
		}
		for _, siginature := range strings.Split(strings.ToLower(strings.TrimSpace(record[3])), ";"){
			temp_proxyInfo.Withdraw[siginature] = true
		}

		var LPTokens []common.Address
		if len(record[5]) >= 42 {
			stakeAddresses := strings.Split(strings.ToLower(strings.TrimSpace(record[5])), ";")
			for _, address := range stakeAddresses{
				LPTokens = append(LPTokens, common.HexToAddress(address))
			}
			temp_proxyInfo.LPTokens = LPTokens
		}

		proxyInfoMap[proxyName] = temp_proxyInfo

		// create related dir
		if createDataset{
			err = os.Mkdir("hunter/defi_apps/" + proxyName, os.ModePerm)
			err = os.Mkdir("hunter/defi_apps/" + proxyName + "/historyTx/", os.ModePerm)
			for _, proxyContract := range proxyContracts{
				os.Mkdir("hunter/defi_apps/" + proxyName + "/historyTx/" + strings.ToLower(proxyContract.String()), os.ModePerm)
			}
			for _, lpContract := range LPTokens{
				os.Mkdir("hunter/defi_apps/" + proxyName + "/historyTx/" + strings.ToLower(lpContract.String()), os.ModePerm)
			}
		}
		if createSeed{
			err = os.Mkdir("hunter/seed_and_env/" + proxyName, os.ModePerm)
			// err = os.Mkdir("hunter/seed_and_env/" + proxyName + "/msg", os.ModePerm)
			// err = os.Mkdir("hunter/seed_and_env/" + proxyName + "/msg/proxy", os.ModePerm)
			// err = os.Mkdir("hunter/seed_and_env/" + proxyName + "/msg/stake", os.ModePerm)
			err = os.Mkdir("hunter/seed_and_env/" + proxyName + "/statedb", os.ModePerm)
			// err = os.Mkdir("hunter/seed_and_env/" + proxyName + "/statedb/input", os.ModePerm)
			err = os.Mkdir("hunter/seed_and_env/" + proxyName + "/statedb/output", os.ModePerm)
		}
	}

	// load erc20 ABI
	jsonFile, err := os.Open("hunter/abi/erc20.json")
	if err != nil {
		fmt.Println(err)
	}	
	defer jsonFile.Close()
	erc20, _ := ioutil.ReadAll(jsonFile)
	erc20string = string(erc20)

	return proxyInfoMap
}

func SetCallVisibility(flag bool){
	printCall = flag
}

func SetEventVisibility(flag bool){
	printEvent = flag
}

func FindCommonAddress(userRelatedMap UserRelatedMap) map[common.Address]bool {
	commonAddressMap := make(map[common.Address]bool)
	relatedAddressCountMap := make(map[common.Address]int)

	userCount := 0
	for userAddress := range userRelatedMap{
		userCount += 1
		for relatedAddress := range userRelatedMap[userAddress]{
			relatedAddressCountMap[relatedAddress] += 1
		}
	}
	// fmt.Println("All User", userCount)
	for relatedAddress := range relatedAddressCountMap{
		if float64(relatedAddressCountMap[relatedAddress]) > float64(userCount) * float64(0.9){
			// fmt.Println(relatedAddress, relatedAddressCountMap[relatedAddress])
			commonAddressMap[relatedAddress] = true
		}
	}
	return commonAddressMap
}