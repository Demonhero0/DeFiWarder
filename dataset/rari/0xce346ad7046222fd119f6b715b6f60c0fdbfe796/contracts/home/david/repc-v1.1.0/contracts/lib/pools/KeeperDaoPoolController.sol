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

import "@0x/contracts-erc20/contracts/src/interfaces/IEtherToken.sol";

import "../../external/keeperdao/ILiquidityPool.sol";
import "../../external/keeperdao/IKToken.sol";

/**
 * @title KeeperDaoPoolController
 * @author David Lucid <david@rari.capital> (https://github.com/davidlucid)
 * @author Richter Brzeski <richter@rari.capital> (https://github.com/richtermb)
 * @dev This library handles deposits to and withdrawals from KeeperDAO liquidity pools.
 */
library KeeperDaoPoolController {
    using SafeMath for uint256;
    using SafeERC20 for IERC20;

    address payable constant private KEEPERDAO_CONTRACT = 0xEB7e15B4E38CbEE57a98204D05999C3230d36348;
    ILiquidityPool constant private _liquidityPool = ILiquidityPool(KEEPERDAO_CONTRACT);

    // KeeperDAO's representation of ETH
    address constant private ETHEREUM_ADDRESS = address(0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE);

    /**
     * @dev Returns the fund's balance in the KeeperDAO pool.
     */
    function getBalance() external view returns (uint256) {
        return _liquidityPool.underlyingBalance(ETHEREUM_ADDRESS, address(this));
    }

    /**
     * @dev Approves tokens to KeeperDAO to burn without spending gas on every deposit.
     * @param amount Amount of the specified token to approve to KeeperDAO.
     * @return Boolean indicating success.
     */
    function approve(uint256 amount) external returns (bool) {
        IKToken kEther = _liquidityPool.kToken(ETHEREUM_ADDRESS);
        uint256 allowance = kEther.allowance(address(this), KEEPERDAO_CONTRACT);
        if (allowance == amount) return true;
        if (amount > 0 && allowance > 0) kEther.approve(KEEPERDAO_CONTRACT, 0);
        kEther.approve(KEEPERDAO_CONTRACT, amount);
        return true;
    }

    /**
     * @dev Deposits funds to the KeeperDAO pool..
     * @param amount The amount of ETH to be deposited.
     * @return Boolean indicating success.
     */
    function deposit(uint256 amount) external returns (bool) {
        require(amount > 0, "Amount must be greater than 0.");

        _liquidityPool.deposit.value(amount)(ETHEREUM_ADDRESS, amount);

        return true;
    }

    /**
     * @dev Withdraws funds from the KeeperDAO pool.
     * @param amount The amount of ETH to be withdrawn.
     * @return Boolean indicating success.
     */
    function withdraw(uint256 amount) external returns (bool) {
        require(amount > 0, "Amount must be greater than 0.");

        _liquidityPool.withdraw(address(uint160(address(this))), 
                                _liquidityPool.kToken(ETHEREUM_ADDRESS), 
                                calculatekEtherWithdrawAmount(amount));

        return true;
    }

    /**
     * @dev Withdraws all funds from the KeeperDAO pool.
     * @return Boolean indicating success.
     */
    function withdrawAll() external returns (bool) {
        IKToken kEther = _liquidityPool.kToken(ETHEREUM_ADDRESS);
        uint256 entireBalance = kEther.balanceOf(address(this));

        _liquidityPool.withdraw(address(uint160(address(this))), kEther, entireBalance);

        return true;
    }

    /**
     * @dev Calculates an amount of kEther to withdraw equivalent to amount parameter in ETH.
     * @return amount to withdraw in kEther.
     */
    function calculatekEtherWithdrawAmount(uint256 amount) internal view returns (uint256) {
        IKToken kEther = _liquidityPool.kToken(ETHEREUM_ADDRESS);
        uint256 totalSupply = kEther.totalSupply();
        uint256 borrowableBalance = _liquidityPool.borrowableBalance(ETHEREUM_ADDRESS);
        uint256 kEtherAmount = amount.mul(totalSupply).div(borrowableBalance); 
        if (kEtherAmount.mul(borrowableBalance).div(totalSupply) < amount) kEtherAmount++;
        return kEtherAmount;
    }
}
