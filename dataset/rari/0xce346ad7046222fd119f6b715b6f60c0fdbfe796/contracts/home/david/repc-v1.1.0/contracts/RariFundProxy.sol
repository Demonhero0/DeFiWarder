/**
 * COPYRIGHT © 2020 RARI CAPITAL, INC. ALL RIGHTS RESERVED.
 * Anyone is free to integrate the public (i.e., non-administrative) application programming interfaces (APIs) of the official Ethereum smart contract instances deployed by Rari Capital, Inc. in any application (commercial or noncommercial and under any license), provided that the application does not abuse the APIs or act against the interests of Rari Capital, Inc.
 * Anyone is free to study, review, and analyze the source code contained in this package.
 * Reuse (including deployment of smart contracts other than private testing on a private network), modification, redistribution, or sublicensing of any source code contained in this package is not permitted without the explicit permission of David Lucid of Rari Capital, Inc.
 * No one is permitted to use the software for any purpose other than those allowed by this license.
 * This license is liable to change at any time at the sole discretion of David Lucid of Rari Capital, Inc.
 */

pragma solidity 0.5.17;
pragma experimental ABIEncoderV2;

import "@openzeppelin/contracts-ethereum-package/contracts/math/SafeMath.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/drafts/SignedSafeMath.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/ownership/Ownable.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/token/ERC20/SafeERC20.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/token/ERC20/ERC20Detailed.sol";

import "@0x/contracts-exchange-libs/contracts/src/LibOrder.sol";
import "@0x/contracts-erc20/contracts/src/interfaces/IEtherToken.sol";

import "./lib/exchanges/ZeroExExchangeController.sol";
import "./RariFundManager.sol";

/**
 * @title RariFundProxy
 * @author David Lucid <david@rari.capital> (https://github.com/davidlucid)
 * @author Richter Brzeski <richter@rari.capital> (https://github.com/richtermb)
 * @dev This contract faciliates deposits to RariFundManager from exchanges and withdrawals from RariFundManager for exchanges.
 */
contract RariFundProxy is Ownable {
    using SafeMath for uint256;
    using SignedSafeMath for int256;
    using SafeERC20 for IERC20;

    /**
     * @dev Maps ERC20 token contract addresses to supported currency codes.
     */
    mapping(string => address) private _erc20Contracts;

    /**
     * @dev Constructor that sets supported ERC20 token contract addresses.
     */
    constructor () public {
        Ownable.initialize(msg.sender);
    }

    /**
     * @dev Address of the RariFundManager.
     */
    address payable private _rariFundManagerContract;

    /**
     * @dev Contract of the RariFundManager.
     */
    RariFundManager public rariFundManager;

    /**
     * @dev Emitted when the RariFundManager of the RariFundProxy is set.
     */
    event FundManagerSet(address newContract);

    /**
     * @dev WETH token address.
     */
    address constant private WETH_CONTRACT = 0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2;

    /**
     * @dev WETH token contract.
     */
    IEtherToken constant private _weth = IEtherToken(WETH_CONTRACT);

    /**
     * @dev Sets or upgrades the RariFundManager of the RariFundProxy.
     * @param newContract The address of the new RariFundManager contract.
     */
    function setFundManager(address payable newContract) external onlyOwner {
        // Approve maximum output tokens to RariFundManager for deposit
        // see safeApprove in IERC20
        if (_rariFundManagerContract != address(0)) _weth.approve(_rariFundManagerContract, 0);
        if (newContract != address(0)) _weth.approve(newContract, uint256(-1));

        _rariFundManagerContract = newContract;
        rariFundManager = RariFundManager(_rariFundManagerContract);
        emit FundManagerSet(newContract);
    }

    /**
     * @dev Payable fallback function called by 0x exchange to refund unspent protocol fee.
     */
    function () external payable { }

    /**
     * @dev Emitted when funds have been exchanged before being deposited via RariFundManager.
     * If exchanging from ETH, `inputErc20Contract` = address(0).
     */
    event PreDepositExchange(address indexed inputErc20Contract, address indexed payee, uint256 makerAssetFilledAmount, uint256 depositAmount);

    /**
     * @dev Emitted when funds have been exchanged after being withdrawn via RariFundManager.
     * If exchanging from ETH, `outputErc20Contract` = address(0).
     */
    event PostWithdrawalExchange(address indexed outputErc20Contract, address indexed payee, uint256 withdrawalAmount, uint256 takerAssetFilledAmount);

    /**
     * @notice Exchanges and deposits funds to RariFund in exchange for RFT (via 0x).
     * You can retrieve orders from the 0x swap API (https://0x.org/docs/api#get-swapv0quote). See the web client for implementation.
     * Please note that you must approve RariFundProxy to transfer at least `inputAmount` unless you are inputting ETH.
     * You also must input at least enough ETH to cover the protocol fee (and enough to cover `orders` if you are inputting ETH).
     * @dev We should be able to make this function external and use calldata for all parameters, but Solidity does not support calldata structs (https://github.com/ethereum/solidity/issues/5479).
     * @param inputErc20Contract The ERC20 contract address of the token to be exchanged. Set to address(0) to input ETH.
     * @param inputAmount The amount of tokens to be exchanged (including taker fees).
     * @param orders The limit orders to be filled in ascending order of the price you pay.
     * @param signatures The signatures for the orders.
     * @param takerAssetFillAmount The amount of the taker asset to sell (excluding taker fees).
     */
    function exchangeAndDeposit(address inputErc20Contract, uint256 inputAmount, LibOrder.Order[] memory orders, bytes[] memory signatures, uint256 takerAssetFillAmount) public payable {
        // Input validation
        require(_rariFundManagerContract != address(0), "Fund manager contract not set. This may be due to an upgrade of this proxy contract.");
        require(inputAmount > 0, "Input amount must be greater than 0.");
        require(inputErc20Contract != 0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2, "Input and output currencies cannot be the same.");
        require(orders.length > 0, "Orders array is empty.");
        require(orders.length == signatures.length, "Length of orders and signatures arrays must be equal.");
        require(takerAssetFillAmount > 0, "Taker asset fill amount must be greater than 0.");

        // Transfer input tokens from msg.sender if not inputting ETH
        IERC20(inputErc20Contract).safeTransferFrom(msg.sender, address(this), inputAmount); // The user must approve the transfer of tokens beforehand

        // Approve and exchange tokens
        if (inputAmount > ZeroExExchangeController.allowance(inputErc20Contract)) ZeroExExchangeController.approve(inputErc20Contract, uint256(-1));
        uint256[2] memory filledAmounts = ZeroExExchangeController.marketSellOrdersFillOrKill(orders, signatures, takerAssetFillAmount, msg.value);

        // Unwrap outputted WETH
        uint256 wethBalance = _weth.balanceOf(address(this));
        require(wethBalance > 0, "No WETH outputted.");
        _weth.withdraw(wethBalance);

        // Refund unused input tokens
        IERC20 inputToken = IERC20(inputErc20Contract);
        uint256 inputTokenBalance = inputToken.balanceOf(address(this));
        if (inputTokenBalance > 0) inputToken.safeTransfer(msg.sender, inputTokenBalance);

        // Emit event
        emit PreDepositExchange(inputErc20Contract, msg.sender, filledAmounts[0], filledAmounts[1]);

        // Deposit output tokens
        rariFundManager.depositTo.value(wethBalance)(msg.sender);
    }

    /**
     * @notice Withdraws funds from RariFund in exchange for RFT and exchanges to them to the desired currency (if no 0x orders are supplied, exchanges DAI, USDC, USDT, TUSD, and mUSD via mStable).
     * You can retrieve orders from the 0x swap API (https://0x.org/docs/api#get-swapv0quote). See the web client for implementation.
     * Please note that you must approve RariFundManager to burn of the necessary amount of RFT.
     * You also must input at least enough ETH to cover the protocol fees.
     * @dev We should be able to make this function external and use calldata for all parameters, but Solidity does not support calldata structs (https://github.com/ethereum/solidity/issues/5479).
     * @param inputAmount The amounts of tokens to be withdrawn and exchanged (including taker fees).
     * @param outputErc20Contract The ERC20 contract address of the token to be outputted by the exchange. Set to address(0) to output ETH.
     * @param orders The limit orders to be filled in ascending order of the price you pay.
     * @param signatures The signatures for the orders.
     * @param makerAssetFillAmount The amount of the maker assets to buy.
     */
    function withdrawAndExchange(uint256 inputAmount, address outputErc20Contract, LibOrder.Order[] memory orders, bytes[] memory signatures, uint256 makerAssetFillAmount) public payable {
        // Input validation
        require(inputAmount > 0, "Input amount must be greater than 0.");
        require(outputErc20Contract != 0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2, "Input and output currencies cannot be the same.");
        require(makerAssetFillAmount > 0, "Maker asset amount must be greater than 0.");
        require(orders.length > 0 && signatures.length > 0, "Must supply more than 0 orders and signatures.");
        require(orders.length == signatures.length, "Lengths of all orders and signatures arrays must be equal.");
        require(_rariFundManagerContract != address(0), "Fund manager contract not set. This may be due to an upgrade of this proxy contract.");

        // Withdraw input tokens
        rariFundManager.withdrawFrom(msg.sender, inputAmount);

        // Wrap ETH for exchanging with 0x
        _weth.deposit.value(inputAmount)();

        // Exchange tokens and emit event
        uint256[2] memory filledAmounts = ZeroExExchangeController.marketBuyOrdersFillOrKill(orders, signatures, makerAssetFillAmount, msg.value);
        emit PostWithdrawalExchange(outputErc20Contract, msg.sender, filledAmounts[0], filledAmounts[1]);

        // Unwrap unused WETH
        uint256 wethBalance = _weth.balanceOf(address(this));
        _weth.withdraw(wethBalance);

        // Forward output tokens
        IERC20 outputToken = IERC20(outputErc20Contract);
        uint256 outputTokenBalance = outputToken.balanceOf(address(this));
        if (outputTokenBalance > 0) outputToken.safeTransfer(msg.sender, outputTokenBalance);

        // Forward unused ETH
        uint256 ethBalance = address(this).balance;
        
        if (ethBalance > 0) {
            (bool success, ) = msg.sender.call.value(ethBalance)("");
            require(success, "Failed to transfer ETH to msg.sender after exchange.");
        }
    }
}
