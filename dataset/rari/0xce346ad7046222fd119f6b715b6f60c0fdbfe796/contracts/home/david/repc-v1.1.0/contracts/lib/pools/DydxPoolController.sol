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
import "@openzeppelin/contracts-ethereum-package/contracts/token/ERC20/SafeERC20.sol";

import "../../external/dydx/SoloMargin.sol";
import "../../external/dydx/lib/Account.sol";
import "../../external/dydx/lib/Actions.sol";
import "../../external/dydx/lib/Types.sol";

import "@0x/contracts-erc20/contracts/src/interfaces/IEtherToken.sol";

/**
 * @title DydxPoolController
 * @author David Lucid <david@rari.capital> (https://github.com/davidlucid)
 * @author Richter Brzeski <richter@rari.capital> (https://github.com/richtermb)
 * @dev This library handles deposits to and withdrawals from dYdX liquidity pools.
 */
library DydxPoolController {
    using SafeMath for uint256;
    using SafeERC20 for IERC20;

    address constant private SOLO_MARGIN_CONTRACT = 0x1E0447b19BB6EcFdAe1e4AE1694b0C3659614e4e;
    SoloMargin constant private _soloMargin = SoloMargin(SOLO_MARGIN_CONTRACT);
    uint256 constant private WETH_MARKET_ID = 0;

    address constant private WETH_CONTRACT = 0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2;
    IEtherToken constant private _weth = IEtherToken(WETH_CONTRACT);

    /**
     * @dev Returns the fund's balance of the specified currency in the dYdX pool.
     */
    function getBalance() external view returns (uint256) {
        Account.Info memory account = Account.Info(address(this), 0);
        (, , Types.Wei[] memory weis) = _soloMargin.getAccountBalances(account);
        return weis[WETH_MARKET_ID].sign ? weis[WETH_MARKET_ID].value : 0;
    }

    /**
     * @dev Approves tokens to dYdX without spending gas on every deposit.
     * @param amount Amount of the specified token to approve to dYdX.
     * @return Boolean indicating success.
     */
    function approve(uint256 amount) external returns (bool) {
        uint256 allowance = _weth.allowance(address(this), SOLO_MARGIN_CONTRACT);
        if (allowance == amount) return true;
        if (amount > 0 && allowance > 0) _weth.approve(SOLO_MARGIN_CONTRACT, 0);
        _weth.approve(SOLO_MARGIN_CONTRACT, amount);
        return true;
    }

    /**
     * @dev Deposits funds to the dYdX pool. Assumes that you have already approved >= the amount to dYdX.
     * @param amount The amount of tokens to be deposited.
     * @return Boolean indicating success.
     */
    function deposit(uint256 amount) external returns (bool) {
        require(amount > 0, "Amount must be greater than 0.");

        _weth.deposit.value(amount)();

        Account.Info memory account = Account.Info(address(this), 0);
        Account.Info[] memory accounts = new Account.Info[](1);
        accounts[0] = account;

        Types.AssetAmount memory assetAmount = Types.AssetAmount(true, Types.AssetDenomination.Wei, Types.AssetReference.Delta, amount);
        bytes memory emptyData;

        Actions.ActionArgs memory action = Actions.ActionArgs(
            Actions.ActionType.Deposit,
            0,
            assetAmount,
            WETH_MARKET_ID,
            0,
            address(this),
            0,
            emptyData
        );

        Actions.ActionArgs[] memory actions = new Actions.ActionArgs[](1);
        actions[0] = action;

        _soloMargin.operate(accounts, actions);

        return true;
    }

    /**
     * @dev Withdraws funds from the dYdX pool.
     * @param amount The amount of tokens to be withdrawn.
     * @return Boolean indicating success.
     */
    function withdraw(uint256 amount) external returns (bool) {
        require(amount > 0, "Amount must be greater than 0.");

        Account.Info memory account = Account.Info(address(this), 0);
        Account.Info[] memory accounts = new Account.Info[](1);
        accounts[0] = account;

        Types.AssetAmount memory assetAmount = Types.AssetAmount(false, Types.AssetDenomination.Wei, Types.AssetReference.Delta, amount);
        bytes memory emptyData;

        Actions.ActionArgs memory action = Actions.ActionArgs(
            Actions.ActionType.Withdraw,
            0,
            assetAmount,
            WETH_MARKET_ID,
            0,
            address(this),
            0,
            emptyData
        );

        Actions.ActionArgs[] memory actions = new Actions.ActionArgs[](1);
        actions[0] = action;

        _soloMargin.operate(accounts, actions);

        _weth.withdraw(amount); // Convert to ETH

        return true;
    }

    /**
     * @dev Withdraws all funds from the dYdX pool.
     * @return Boolean indicating success.
     */
    function withdrawAll() external returns (bool) {

        Account.Info memory account = Account.Info(address(this), 0);
        Account.Info[] memory accounts = new Account.Info[](1);
        accounts[0] = account;

        Types.AssetAmount memory assetAmount = Types.AssetAmount(true, Types.AssetDenomination.Par, Types.AssetReference.Target, 0);
        bytes memory emptyData;

        Actions.ActionArgs memory action = Actions.ActionArgs(
            Actions.ActionType.Withdraw,
            0,
            assetAmount,
            WETH_MARKET_ID,
            0,
            address(this),
            0,
            emptyData
        );

        Actions.ActionArgs[] memory actions = new Actions.ActionArgs[](1);
        actions[0] = action;

        _soloMargin.operate(accounts, actions);

        _weth.withdraw(_weth.balanceOf(address(this))); // Convert to ETH

        return true;
    }
}
