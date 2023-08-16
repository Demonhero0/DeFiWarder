// SPDX-License-Identifier: UNLICENSED

// This contract locks uniswap v2 liquidity tokens. Used to give investors peace of mind a token team has locked liquidity
// and that the univ2 tokens cannot be removed from uniswap until the specified unlock date has been reached.

pragma solidity ^0.8.0;

interface IUnicryptV2Locker {
    event onDeposit(address lpToken, address user, uint amount, uint lockDate, uint unlockDate);
    event onWithdraw(address lpToken, uint amount);

    /**
     * @notice Creates a new lock
     * @param _lpToken the univ2 token address
     * @param _amount amount of LP tokens to lock
     * @param _unlock_date the unix timestamp (in seconds) until unlock
     * @param _referral the referrer address if any or address(0) for none
     * @param _fee_in_eth fees can be paid in eth or in a secondary token such as UNCX with a discount on univ2 tokens
     * @param _withdrawer the user who can withdraw liquidity once the lock expires.
     */
    function lockLPToken(
        address _lpToken,
        uint _amount,
        uint _unlock_date,
        address payable _referral,
        bool _fee_in_eth,
        address payable _withdrawer
    ) external payable;

    /**
     * @notice extend a lock with a new unlock date, _index and _lockID ensure the correct lock is changed
     * this prevents errors when a user performs multiple tx per block possibly with varying gas prices
     */
    function relock(
        address _lpToken,
        uint _index,
        uint _lockID,
        uint _unlock_date
    ) external;

    /**
     * @notice withdraw a specified amount from a lock. _index and _lockID ensure the correct lock is changed
     * this prevents errors when a user performs multiple tx per block possibly with varying gas prices
     */
    function withdraw(
        address _lpToken,
        uint _index,
        uint _lockID,
        uint _amount
    ) external;

    /**
     * @notice increase the amount of tokens per a specific lock, this is preferable to creating a new lock, less fees, and faster loading on our live block explorer
     */
    function incrementLock(
        address _lpToken,
        uint _index,
        uint _lockID,
        uint _amount
    ) external;

    /**
     * @notice split a lock into two seperate locks, useful when a lock is about to expire and youd like to relock a portion
     * and withdraw a smaller portion
     */
    function splitLock(
        address _lpToken,
        uint _index,
        uint _lockID,
        uint _amount
    ) external payable;

    /**
     * @notice transfer a lock to a new owner, e.g. presale project -> project owner
     * CAN USE TO MIGRATE UNICRYPT LOCKS TO OUR PLATFORM
     * Must be called by the owner of the token
     */
    function transferLockOwnership(
        address _lpToken,
        uint _index,
        uint _lockID,
        address payable _newOwner
    ) external;

    /**
     * @notice migrates liquidity to uniswap v3
     */
    function migrate(
        address _lpToken,
        uint _index,
        uint _lockID,
        uint _amount
    ) external;

    function getNumLocksForToken(address _lpToken) external view returns (uint);

    function getNumLockedTokens() external view returns (uint);

    function getLockedTokenAtIndex(uint _index) external view returns (address);

    // user functions
    function getUserNumLockedTokens(address _user) external view returns (uint);

    function getUserLockedTokenAtIndex(address _user, uint _index) external view returns (address);

    function getUserNumLocksForToken(address _user, address _lpToken) external view returns (uint);

    function getUserLockForTokenAtIndex(
        address _user,
        address _lpToken,
        uint _index
    )
        external
        view
        returns (
            uint,
            uint,
            uint,
            uint,
            uint,
            address
        );

    function tokenLocks(address asset, uint _lockID)
        external
        returns (
            uint lockDate,
            uint amount,
            uint initialAmount,
            uint unlockDate,
            uint lockID,
            address owner
        );
}
