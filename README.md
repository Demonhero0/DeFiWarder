# Artifacts of DeFiWarder

DeFiWarder: Protecting DeFi Apps from Token Leaking Vulnerabilities

## Introduction

### DataSet

The dataset includs 32 Dapps with Token Leaking vulnerability, which are presented in the folder "dataset", whose information is recorded in "dataset.csv".

#### dataset.csv

| Syntax      | Description |
| ----------- | ----------- |
| dapp      | The name of the dapp       |
| proxy_contract   | The address of the proxy contract of the dapp        |
| logic_contract   | The address of the vulnerable logic contract of the dapp |
| main_name        | The name of main contract of the logic contract |
| sol_path         | The path of the dapp in folder "dataset" |
| version          | The compiler version of the logic contract |
| attack_tx        | The example transaction hash of the atack |

#### dataset/*

We present the detialed information of the dapp (logic contract), which are sourced from Etherscan.

| File Name      | Description |
| ----------- | ----------- |
| contracts      | The source code of the contract  |
| abi.json   | The abi of the contract       |
| bytecode.txt   | The bytecode of the contract |
| outputJson.json        | The original data from Etherscan |

### Tool

The code of DeFiWarder is in "DeFiWarder". The detail usage is presented in DeFiWarder/README.md
