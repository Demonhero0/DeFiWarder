# DeFiWarder

DeFiWarder is a tool for protecting DeFi Apps from Token Leaking vulnerabilities.

DeFiWarder is implemented on the top of Ethereum substate recorder/replayer (see [README_Ethereum_Substate.md](README_Ethereum_Substate.md)).

## Building the source

For prerequisites and detailed build instructions please read the [Installation Instructions](https://geth.ethereum.org/docs/install-and-build/installing-geth).

Building `geth` requires both a Go (version 1.14 or later) and a C compiler. You can install
them using your favourite package manager. Once the dependencies are installed, run

```shell
make geth
```

or, to build the full suite of utilities:

```shell
make all
```

## Usage

The data of 32 DeFi apps is recorded in "hunter/defi_warder.csv".

### Replay transactions with DeFiWarder
Using DeFiWarder to detect Token Leaking vulnerbaility from historical transactions， which will load the related transactions from "hunter/defiTxMap.json" and replay them with DeFiWarder.
```
go run cmd/substate-cli/main.go replay-txs <dapp> <strat_block> <end_block>  --substatedir <substate> --skip-transfer-txs --workers 1
```

example.
```
go run cmd/substate-cli/main.go replay-txs opyn 10000000 15500000  --substatedir <path of substatedir> --workers 1
```

### Analyze prepared transactions with DeFiWarder
We also provide the approach to run DeFiWarder with prepared transactions, which will load the transactions from "hunter/defi_apps/dapp/historyTx" and perform detection. Note that the transaction data is large, we only provide some examples, including opyn, warp_finance and umbrella.
```
go run hunter/main.go
```