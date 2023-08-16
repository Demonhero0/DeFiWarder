package fuzz

import (
	"math/big"
	"fmt"
	"strconv"
	cryptoRand "crypto/rand"
	mathRand "math/rand"

	types "github.com/ethereum/go-ethereum/core/types"
	common "github.com/ethereum/go-ethereum/common"
	abi "github.com/ethereum/go-ethereum/accounts/abi"
)

type SeedPool struct {
	
	AddressSeedMap map[common.Address]uint
	AddressSeedList []common.Address
	UintSeedMap map[uint64]uint
	UintSeedList []uint64
	IntSeedMap map[int64]uint
	IntSeedList []int64
	BytesSeedMap map[string]uint
	EtherAmountSeedMap map[string]uint
	EtherAmountSeedList []string
}

func (seedPool *SeedPool) initSeedPool(){

	seedPool.AddressSeedMap = make(map[common.Address]uint)
	seedPool.IntSeedMap = make(map[int64]uint)
	seedPool.UintSeedMap = make(map[uint64]uint)
	seedPool.BytesSeedMap = make(map[string]uint)
	seedPool.EtherAmountSeedMap = make(map[string]uint)

	for _ , i := range IntPool{
		ii, _ := strconv.ParseInt(i, 10, 64)
		seedPool.IntSeedMap[ii] += 1
	}
	for _ , i := range UintPool{
		u, _ := strconv.ParseUint(i, 10, 64)
		seedPool.UintSeedMap[u] += 1
	}
	for _ , address := range AddressPool{
		seedPool.AddressSeedMap[common.HexToAddress(address)] += 1
	}
	for _ , EtherAmt := range EtherPool{
		seedPool.EtherAmountSeedMap[EtherAmt] += 1
	}
}

func (seedPool *SeedPool) updateMapToList() SeedPool{
	
	var newSeedPool SeedPool
	for key, _ := range(seedPool.IntSeedMap){
		newSeedPool.IntSeedList = append(newSeedPool.IntSeedList, key)
	}
	for key, _ := range(seedPool.UintSeedMap){
		newSeedPool.UintSeedList = append(newSeedPool.UintSeedList, key)
	}
	for key, _ := range(seedPool.AddressSeedMap){
		newSeedPool.AddressSeedList = append(newSeedPool.AddressSeedList, key)
	}
	for key, _ := range(seedPool.EtherAmountSeedMap){
		newSeedPool.EtherAmountSeedList = append(newSeedPool.EtherAmountSeedList, key)
	}
	return newSeedPool
}

func (seedPool *SeedPool) parseMethodArgs(contract *Contract, data []byte) {
	// for ether amount
	// seed.EtherAmountSeedMap[msg.Value().String()] += 1

	method, err := contract.Abi.MethodById(data)
	if err != nil{
		// fmt.Println(err, msg.Data())
	} else {
		contract.MethodCallMap[method.Name] = append(contract.MethodCallMap[method.Name], data)
		// fmt.Println(method.Name, method.Inputs.NonIndexed(), hex.EncodeToString(msg.Data()[4:]))
		// parse the input
		args, e := method.Inputs.Unpack(data[4:])
		if e != nil {
			fmt.Println(e)
		}
		for index, arg := range method.Inputs.NonIndexed(){
			// fmt.Println(arg.Type.T, args[index])
			// seed.TypeMap[arg.Type.T] += 1
			switch arg.Type.T {
				case abi.IntTy:
					// intString := args[index].(*big.Int).String()
					i := args[index].(*big.Int).Int64()
					// fmt.Println(args[index], intString)
					seedPool.IntSeedMap[i] += 1
				case abi.UintTy:
					// uintString := args[index].(*big.Int).String()
					u := args[index].(*big.Int).Uint64()
					// fmt.Println("UintTy", reflect.TypeOf(args[index]), args[index])
					seedPool.UintSeedMap[u] += 1
				case abi.AddressTy:
					// fmt.Println("AddressTy", reflect.TypeOf(args[index]), args[index])
					seedPool.AddressSeedMap[args[index].(common.Address)] += 1
				// case abi.BytesTy:
				// 	fmt.Println("BytesTy", arg.Type, arg.Type.Size, reflect.TypeOf(args[index]), args[index])
				// 	byteString := string(args[index].([32]byte))
				// 	seed.BytesSeedMap[byteString] += 1
				// case abi.FixedBytesTy:
				// 	fmt.Println("FixedBytesTy", arg.Type, arg.Type.Size, args[index])
					// byteString := string(args[index].([32]byte))
				// 	seed.BytesSeedMap[byteString] += 1
			}
		}
	}
}

func (seedPool *SeedPool) generateTypeInput(t abi.Type, msg *types.Message) interface{} {
	switch t.T {
		case abi.IntTy:
			return seedPool.selectInt(t.Size)
		case abi.UintTy:
			return seedPool.selectUint(t.Size)
		case abi.BoolTy:
			boolList := [2]bool{true, false}
			n, _ := cryptoRand.Int(cryptoRand.Reader, big.NewInt(2))
			return boolList[n.Int64()]
		// case abi.StringTy:
		// 	panic("Invalid type")
		// case abi.SliceTy:
		// 	panic("Invalid type")
		// case abi.ArrayTy:
		// 	panic("Invalid type")
		// case abi.TupleTy:
		// 	panic("Invalid type")
		case abi.AddressTy:
			// return common.HexToAddress("0x66441289c5185637b35bcD3df89D3d200EE5c76c")
			return seedPool.selectAddress(50, msg)
		case abi.FixedBytesTy:
			// fmt.Println("FixedBytesTy", reflect.ArrayOf(t.Size, reflect.TypeOf(byte(0))))
			return generateFixBytes(t.Size)
		case abi.BytesTy:
			return generateBytes()
		case abi.StringTy:
			return generateString()
		// case abi.HashTy:
		// 	// hashtype currently not used
		// 	panic("Invalid type")
		// case abi.FixedPointTy:
		// 	// fixedpoint type currently not used
		// 	panic("Invalid type")
		// case abi.FunctionTy:
		// 	panic("Invalid type")
		default:
			fmt.Println(t)
			return nil
	}	
}

func (seedPool *SeedPool) selectInt(size int) interface{} {

	n, _ := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(len(seedPool.IntSeedList))))
	// temp_value, _ := new(big.Int).SetString(seed.IntSeedList[n.Int64()],10)
	temp_value := seedPool.IntSeedList[n.Int64()]

	switch size {
	case 8:
		return int8(temp_value)
	case 16:
		return int16(temp_value)
	case 32:
		return int32(temp_value)
	case 64:
		return int64(temp_value)
	}
	return big.NewInt(temp_value)
}

func (seedPool *SeedPool) selectUint(size int) interface{} {
	n, _ := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(len(seedPool.UintSeedList))))
	// temp_value, _ := new(big.Int).SetString(seed.UintSeedList[n.Int64()],10)
	temp_value := seedPool.UintSeedList[n.Int64()]
	switch size {
	case 8:
		return uint8(temp_value)
	case 16:
		return uint16(temp_value)
	case 32:
		return uint32(temp_value)
	case 64:
		return uint64(temp_value)
	}
	return new(big.Int).SetUint64(temp_value)
}

func (seedPool *SeedPool) selectAddress(rateForFrom int64, msg *types.Message) common.Address {
	// fmt.Println("address", len(seedPool.AddressSeedList))
	r, _ := cryptoRand.Int(cryptoRand.Reader, big.NewInt(100))
	if r.Int64() <= rateForFrom{
		return msg.From()
	} else {
		r1, _ := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(len(seedPool.AddressSeedList))))
		return seedPool.AddressSeedList[r1.Int64()]
	}
}

func (seedPool *SeedPool) SelectEtherAmount() *big.Int {
	n, _ := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(len(seedPool.EtherAmountSeedList))))
	temp_value, _ := new(big.Int).SetString(seedPool.EtherAmountSeedList[n.Int64()],10)
	return temp_value
}

func generateString() string {

	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	n, _ := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(100)))
	b := make([]rune, n.Int64())
	for i := range b {
		b[i] = letters[mathRand.Intn(len(letters))]
	}
	return string(b)
}

func generateFixBytes(length int) interface{} {

	switch length{
	case 4:
		var bytes [4]byte
		b := make([]byte, 4)
		mathRand.Read(b)
		copy(bytes[:], b)
		return bytes
	case 8:
		var bytes [8]byte
		b := make([]byte, 8)
		mathRand.Read(b)
		copy(bytes[:], b)
		return bytes
	case 32:
		var bytes [32]byte
		b := make([]byte, 32)
		mathRand.Read(b)
		copy(bytes[:], b)
		return bytes
	case 64:
		var bytes [64]byte
		b := make([]byte, 64)
		mathRand.Read(b)
		copy(bytes[:], b)
		return bytes
	case 96:
		var bytes [96]byte
		b := make([]byte, 96)
		mathRand.Read(b)
		copy(bytes[:], b)
		return bytes
	case 128:
		var bytes [128]byte
		b := make([]byte, 128)
		mathRand.Read(b)
		copy(bytes[:], b)
		return bytes
	case 160:
		var bytes [160]byte
		b := make([]byte, 160)
		mathRand.Read(b)
		copy(bytes[:], b)
		return bytes
	case 192:
		var bytes [192]byte
		b := make([]byte, 192)
		mathRand.Read(b)
		copy(bytes[:], b)
		return bytes
	case 224:
		var bytes [224]byte
		b := make([]byte, 224)
		mathRand.Read(b)
		copy(bytes[:], b)
		return bytes
	case 256:
		var bytes [256]byte
		b := make([]byte, 256)
		mathRand.Read(b)
		copy(bytes[:], b)
		return bytes
	default:
		return [0]byte{}
	}
}

func generateBytes() []byte {
	lengthList := [9]int{0,32,64,96,128,160,192,224,256}
	n, _ := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(9)))
	bytes := make([]byte, lengthList[n.Int64()])
	mathRand.Read(bytes)
	return bytes
}

func PrintSeedPool(seedPool SeedPool){
	fmt.Println("Int", len(seedPool.IntSeedMap))
	fmt.Println("Uint", len(seedPool.UintSeedMap))
	fmt.Println("Address", len(seedPool.AddressSeedMap))
	fmt.Println("Bytes", len(seedPool.BytesSeedMap))
	fmt.Println("EtherAmountMap", len(seedPool.EtherAmountSeedMap))
}