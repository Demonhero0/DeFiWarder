/**
 * COPYRIGHT © 2020 RARI CAPITAL, INC. ALL RIGHTS RESERVED.
 * Anyone is free to integrate the public (i.e., non-administrative) application programming interfaces (APIs) of the official Ethereum smart contract instances deployed by Rari Capital, Inc. in any application (commercial or noncommercial and under any license), provided that the application does not abuse the APIs or act against the interests of Rari Capital, Inc.
 * Anyone is free to study, review, and analyze the source code contained in this package.
 * Reuse (including deployment of smart contracts other than private testing on a private network), modification, redistribution, or sublicensing of any source code contained in this package is not permitted without the explicit permission of David Lucid of Rari Capital, Inc.
 * No one is permitted to use the software for any purpose other than those allowed by this license.
 * This license is liable to change at any time at the sole discretion of David Lucid of Rari Capital, Inc.
 */

pragma solidity 0.5.17;

import "@openzeppelin/contracts-ethereum-package/contracts/math/SafeMath.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/token/ERC20/SafeERC20.sol";

import "../../external/compound/CEther.sol";

/**
 * @title CompoundPoolController
 * @author David Lucid <david@rari.capital> (https://github.com/davidlucid)
 * @author Richter Brzeski <richter@rari.capital> (https://github.com/richtermb)
 * @dev This library handles deposits to and withdrawals from Compound liquidity pools.
 */
library CompoundPoolController {
    using SafeMath for uint256;
    using SafeERC20 for IERC20;

    address constant private cETH_CONTACT_ADDRESS = 0x4Ddc2D193948926D02f9B1fE9e1daa0718270ED5; 
    CEther constant private _cETHContract = CEther(cETH_CONTACT_ADDRESS);

    /**
     * @dev Returns the fund's balance of the specified currency in the Compound pool.
     */
    function getBalance() external returns (uint256) {
        return _cETHContract.balanceOfUnderlying(address(this));
    }

    /**
     * @dev Deposits funds to the Compound pool. Assumes that you have already approved >= the amount to Compound.
     * @param amount The amount of tokens to be deposited.
     * @return Boolean indicating success.
     */
    function deposit(uint256 amount) external returns (bool) {
        require(amount > 0, "Amount must be greater than 0.");
        _cETHContract.mint.value(amount)();
        // require(mintResult == 0, "Error calling mint on Compound cToken: error code not equal to 0");
        return true;
    }

    /**
     * @dev Withdraws funds from the Compound pool.
     * @param amount The amount of tokens to be withdrawn.
     * @return Boolean indicating success.
     */
    function withdraw(uint256 amount) external returns (bool) {
        require(amount > 0, "Amount must be greater than to 0.");
        uint256 redeemResult = _cETHContract.redeemUnderlying(amount);
        require(redeemResult == 0, "Error calling redeemUnderlying on Compound cToken: error code not equal to 0");
        return true;
    }

    /**
     * @dev Withdraws all funds from the Compound pool.
     * @return Boolean indicating success.
     */
    function withdrawAll() external returns (bool) {
        uint256 balance = _cETHContract.balanceOf(address(this));
        if (balance <= 0) return false; // TODO: Or revert("No funds available to redeem from Compound cToken.")
        uint256 redeemResult = _cETHContract.redeem(balance);
        require(redeemResult == 0, "Error calling redeem on Compound cToken: error code not equal to 0");
        return true;
    }
}
