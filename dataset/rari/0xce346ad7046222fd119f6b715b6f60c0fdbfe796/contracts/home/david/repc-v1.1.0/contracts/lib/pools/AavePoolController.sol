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

import "../../external/aave/LendingPool.sol";
import "../../external/aave/AToken.sol";

/**
 * @title AavePoolController
 * @author David Lucid <david@rari.capital> (https://github.com/davidlucid)
 * @author Richter Brzeski <richter@rari.capital> (https://github.com/richtermb)
 * @dev This library handles deposits to and withdrawals from Aave liquidity pools.
 */
library AavePoolController {
    using SafeMath for uint256;
    using SafeERC20 for IERC20;

    /**
     * @dev Aave LendingPool contract address.
     */
    address constant private LENDING_POOL_CONTRACT = 0x398eC7346DcD622eDc5ae82352F02bE94C62d119;

    /**
     * @dev Aave LendingPool contract object.
     */
    LendingPool constant private _lendingPool = LendingPool(LENDING_POOL_CONTRACT);

    /**
     * @dev Aave LendingPoolCore contract address.
     */
    address constant private LENDING_POOL_CORE_CONTRACT = 0x3dfd23A6c5E8BbcFc9581d2E864a68feb6a076d3;

    /**
     * @dev AETH contract address.
     */
    address constant private AETH_CONTRACT = 0x3a3A65aAb0dd2A17E3F1947bA16138cd37d08c04;

    /**
     * @dev AETH contract.
     */
    AToken constant private aETH = AToken(AETH_CONTRACT);


    /**
     * @dev Ethereum address abstraction
     */
     address constant private ETHEREUM_ADDRESS = address(0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE);

     
    /**
     * @dev Returns the fund's balance of the specified currency in the Aave pool.
     */
    function getBalance() external view returns (uint256) {
        return aETH.balanceOf(address(this));
    }

    /**
     * @dev Deposits funds to the Aave pool. Assumes that you have already approved >= the amount to Aave.
     * @param amount The amount of tokens to be deposited.
     * @param referralCode Referral code.
     */
    function deposit(uint256 amount, uint16 referralCode) external {
        require(amount > 0, "Amount must be greater than 0.");
        _lendingPool.deposit.value(amount)(ETHEREUM_ADDRESS, amount, referralCode);
    }

    /**
     * @dev Withdraws funds from the Aave pool.
     * @param amount The amount of tokens to be withdrawn.
     */
    function withdraw(uint256 amount) external {
        require(amount > 0, "Amount must be greater than 0.");
        aETH.redeem(amount);
    }

    /**
     * @dev Withdraws all funds from the Aave pool.
     * @return Boolean indicating success.
     */
    function withdrawAll() external returns (bool) {
        uint256 balance = aETH.balanceOf(address(this));
        if (balance <= 0) return false;
        aETH.redeem(balance);
        return true;
    }
}