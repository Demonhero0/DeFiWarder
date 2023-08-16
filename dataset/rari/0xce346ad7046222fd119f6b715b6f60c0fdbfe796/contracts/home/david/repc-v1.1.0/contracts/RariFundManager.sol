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

import "@openzeppelin/upgrades/contracts/Initializable.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/math/SafeMath.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/drafts/SignedSafeMath.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/ownership/Ownable.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/token/ERC20/SafeERC20.sol";

import "@0x/contracts-exchange-libs/contracts/src/LibOrder.sol";
import "@0x/contracts-erc20/contracts/src/interfaces/IEtherToken.sol";

import "./RariFundController.sol";
import "./RariFundToken.sol";
import "./RariFundProxy.sol";
import "./interfaces/IRariGovernanceTokenDistributor.sol";

/**
 * @title RariFundManager
 * @author David Lucid <david@rari.capital> (https://github.com/davidlucid)
 * @author Richter Brzeski <richter@rari.capital> (https://github.com/richtermb)
 * @dev This contract is the primary contract powering the Rari Ethereum Pool.
 * Anyone can deposit to the fund with deposit(uint256 amount).
 * Anyone can withdraw their funds (with interest) from the fund with withdraw(uint256 amount).
 */
contract RariFundManager is Initializable, Ownable {
    using SafeMath for uint256;
    using SignedSafeMath for int256;
    using SafeERC20 for IERC20;

    /**
     * @dev Boolean that, if true, disables the primary functionality of this RariFundManager.
     */
    bool private _fundDisabled;

    /**
     * @dev Address of the RariFundController.
     */
    address payable private _rariFundControllerContract;

    /**
     * @dev Contract of the RariFundController.
     */
    RariFundController public rariFundController;

    /**
     * @dev Address of the REPT tokem.
     */
    address private _rariFundTokenContract;

    /**
     * @dev Contract for the REPT tokem.
     */
    RariFundToken public rariFundToken;

    /**
     * @dev Address of the RariFundProxy.
     */
    address private _rariFundProxyContract;

    /**
     * @dev Address of the rebalancer.
     */
    address private _rariFundRebalancerAddress;

    /**
     * @dev Array of supported pools.
     */
    uint8[] private _supportedPools;

    /**
     * @dev Initializer that sets supported ETH pools.
     */
    function initialize() public initializer {
        // Initialize base contracts
        Ownable.initialize(msg.sender);

        // Add supported currencies
        addPool(0); // dYdX
        addPool(1); // Compound
        addPool(2); // KeeperDAO
        addPool(3); // Aave

        // Initialize raw fund balance cache (can't set initial values in field declarations with proxy storage)
        _rawFundBalanceCache = -1;
    }

    /**
     * @dev Entry into deposit functionality.
     */
    function () external payable {
        if (msg.sender != _rariFundControllerContract) {
            require(msg.value > 0, "Not enough money deposited.");
            require(_depositTo(msg.sender, msg.value), "Deposit failed.");
        }
    }

    /**
     * @dev Adds a supported pool for eth.
     * @param pool Pool ID to be supported.
     */
    function addPool(uint8 pool) internal {
        _supportedPools.push(pool);
    }

    /**
     * @dev Emitted when RariFundManager is upgraded.
     */
    event FundManagerUpgraded(address newContract);

    /**
     * @dev Upgrades RariFundManager.
     * Sends data to the new contract, sets the new REPT minter, and forwards eth from the old to the new.
     * @param newContract The address of the new RariFundManager contract.
     */
    function upgradeFundManager(address payable newContract) external onlyOwner {
        // Pass data to the new contract
        FundManagerData memory data;

        data = FundManagerData(
            _netDeposits,
            _rawInterestAccruedAtLastFeeRateChange,
            _interestFeesGeneratedAtLastFeeRateChange,
            _interestFeesClaimed
        );

        RariFundManager(newContract).setFundManagerData(data);

        // Update REPT minter
        if (_rariFundTokenContract != address(0)) {
            rariFundToken.addMinter(newContract);
            rariFundToken.renounceMinter();
        }

        emit FundManagerUpgraded(newContract);
    }

    /**
     * @dev Old RariFundManager contract authorized to migrate its data to the new one.
     */
    address payable private _authorizedFundManagerDataSource;

    /**
     * @dev Upgrades RariFundManager.
     * Authorizes the source for fund manager data (i.e., the old fund manager).
     * @param authorizedFundManagerDataSource Authorized source for data (i.e., the old fund manager).
     */
    function authorizeFundManagerDataSource(address payable authorizedFundManagerDataSource) external onlyOwner {
        _authorizedFundManagerDataSource = authorizedFundManagerDataSource;
    }

    /**
     * @dev Struct for data to transfer from the old RariFundManager to the new one.
     */
    struct FundManagerData {
        int256 netDeposits;
        int256 rawInterestAccruedAtLastFeeRateChange;
        int256 interestFeesGeneratedAtLastFeeRateChange;
        uint256 interestFeesClaimed;
    }

    /**
     * @dev Upgrades RariFundManager.
     * Sets data receieved from the old contract.
     * @param data The data from the old contract necessary to initialize the new contract.
     */
    function setFundManagerData(FundManagerData calldata data) external {
        require(_authorizedFundManagerDataSource != address(0) && msg.sender == _authorizedFundManagerDataSource, "Caller is not an authorized source.");
        _netDeposits = data.netDeposits;
        _rawInterestAccruedAtLastFeeRateChange = data.rawInterestAccruedAtLastFeeRateChange;
        _interestFeesGeneratedAtLastFeeRateChange = data.interestFeesGeneratedAtLastFeeRateChange;
        _interestFeesClaimed = data.interestFeesClaimed;
        _interestFeeRate = RariFundManager(_authorizedFundManagerDataSource).getInterestFeeRate();
    }

    /**
     * @dev Emitted when the RariFundController of the RariFundManager is set or upgraded.
     */
    event FundControllerSet(address newContract);


    /**
     * @dev Sets or upgrades the RariFundController of the RariFundManager.
     * @param newContract The address of the new RariFundController contract.
     */
    function setFundController(address payable newContract) external onlyOwner {
        _rariFundControllerContract = newContract;
        rariFundController = RariFundController(_rariFundControllerContract);
        emit FundControllerSet(newContract);
    }

    /**
     * @dev Emitted when the REPT contract of the RariFundManager is set.
     */
    event FundTokenSet(address newContract);

    /**
     * @dev Sets or upgrades the RariFundToken of the RariFundManager.
     * @param newContract The address of the new RariFundToken contract.
     */
    function setFundToken(address newContract) external onlyOwner {
        _rariFundTokenContract = newContract;
        rariFundToken = RariFundToken(_rariFundTokenContract);
        emit FundTokenSet(newContract);
    }

    /**
     * @dev Throws if called by any account other than the RariFundToken.
     */
    modifier onlyToken() {
        require(_rariFundTokenContract == msg.sender, "Caller is not the RariFundToken.");
        _;
    }

    /**
     * @dev Emitted when the RariFundProxy of the RariFundManager is set.
     */
    event FundProxySet(address newContract);

    /**
     * @dev Sets or upgrades the RariFundProxy of the RariFundManager.
     * @param newContract The address of the new RariFundProxy contract.
     */
    function setFundProxy(address newContract) external onlyOwner {
        _rariFundProxyContract = newContract;
        emit FundProxySet(newContract);
    }

    /**
     * @dev Throws if called by any account other than the RariFundProxy.
     */
    modifier onlyProxy() {
        require(_rariFundProxyContract == msg.sender, "Caller is not the RariFundProxy.");
        _;
    }

    /**
     * @dev Emitted when the rebalancer of the RariFundManager is set.
     */
    event FundRebalancerSet(address newAddress);

    /**
     * @dev Sets or upgrades the rebalancer of the RariFundManager.
     * @param newAddress The Ethereum address of the new rebalancer server.
     */
    function setFundRebalancer(address newAddress) external onlyOwner {
        _rariFundRebalancerAddress = newAddress;
        emit FundRebalancerSet(newAddress);
    }

    /**
     * @dev Throws if called by any account other than the rebalancer.
     */
    modifier onlyRebalancer() {
        require(_rariFundRebalancerAddress == msg.sender, "Caller is not the rebalancer.");
        _;
    }

    /**
     * @dev Emitted when the primary functionality of this RariFundManager contract has been disabled.
     */
    event FundDisabled();

    /**
     * @dev Emitted when the primary functionality of this RariFundManager contract has been enabled.
     */
    event FundEnabled();

    /**
     * @dev Disables primary functionality of this RariFundManager so contract(s) can be upgraded.
     */
    function disableFund() external onlyOwner {
        require(!_fundDisabled, "Fund already disabled.");
        _fundDisabled = true;
        emit FundDisabled();
    }

    /**
     * @dev Enables primary functionality of this RariFundManager once contract(s) are upgraded.
     */
    function enableFund() external onlyOwner {
        require(_fundDisabled, "Fund already enabled.");
        _fundDisabled = false;
        emit FundEnabled();
    }

    /**
     * @dev Throws if fund is disabled.
     */
    modifier fundEnabled() {
        require(!_fundDisabled, "This fund manager contract is disabled. This may be due to an upgrade.");
        _;
    }

    /**
     * @dev Boolean indicating if return values of `getPoolBalance` are to be cached.
     */
    bool _cachePoolBalance;

    /**
     * @dev Maps cached pool balances to pool indexes
     */
    mapping(uint8 => uint256) _poolBalanceCache;


    /**
     * @dev Returns the fund controller's balance of the specified currency in the specified pool.
     * @dev Ideally, we can add the view modifier, but Compound's `getUnderlyingBalance` function (called by `CompoundPoolController.getBalance`) potentially modifies the state.
     * @param pool The index of the pool.
     */
    function getPoolBalance(uint8 pool) internal returns (uint256) {
        if (!rariFundController.hasETHInPool(pool)) return 0;

        if (_cachePoolBalance) {
            if (_poolBalanceCache[pool] == 0) _poolBalanceCache[pool] = rariFundController._getPoolBalance(pool);
            return _poolBalanceCache[pool];
        }

        return rariFundController._getPoolBalance(pool);
    }

    /**
     * @dev Caches return value of `getPoolBalance` for the duration of the function.
     */
    modifier cachePoolBalance() {
        bool cacheSetPreviously = _cachePoolBalance;
        _cachePoolBalance = true;
        _;

        if (!cacheSetPreviously) {
            _cachePoolBalance = false;

            for (uint256 i = 0; i < _supportedPools.length; i++) {
                _poolBalanceCache[_supportedPools[i]] = 0;
            }
        }
    }

    /**
     * @notice Returns the fund's raw total balance (all REPT holders' funds + all unclaimed fees).
     * @dev Ideally, we can add the view modifier, but Compound's `getUnderlyingBalance` function (called by `RariFundController.getPoolBalance`) potentially modifies the state.
     */
    function getRawFundBalance() public returns (uint256) {
        uint256 totalBalance = _rariFundControllerContract.balance; // ETH balance in fund controller contract

        for (uint256 i = 0; i < _supportedPools.length; i++)
            totalBalance = totalBalance.add(getPoolBalance(_supportedPools[i]));

        return totalBalance;
    }

    /**
     * @dev Caches the fund's raw total balance (all REPT holders' funds + all unclaimed fees) of ETH.
     */
    int256 private _rawFundBalanceCache;

    /**
     * @dev Caches the value of getRawFundBalance() for the duration of the function.
     */
    modifier cacheRawFundBalance() {
        bool cacheSetPreviously = _rawFundBalanceCache >= 0;
        if (!cacheSetPreviously) _rawFundBalanceCache = int256(getRawFundBalance());
        _;
        if (!cacheSetPreviously) _rawFundBalanceCache = -1;
    }

    /**
     * @notice Returns the fund's total investor balance (all REPT holders' funds but not unclaimed fees) of all currencies in EETH (scaled by 1e18).
     * @dev Ideally, we can add the view modifier, but Compound's `getUnderlyingBalance` function (called by `getRawFundBalance`) potentially modifies the state.
     */
    function getFundBalance() public cacheRawFundBalance returns (uint256) {
        return getRawFundBalance().sub(getInterestFeesUnclaimed());
    }

    /**
     * @notice Returns an account's total balance in ETH.
     * @dev Ideally, we can add the view modifier, but Compound's `getUnderlyingBalance` function (called by `getRawFundBalance`) potentially modifies the state.
     * @param account The account whose balance we are calculating.
     */
    function balanceOf(address account) external returns (uint256) {
        uint256 reptTotalSupply = rariFundToken.totalSupply();
        if (reptTotalSupply == 0) return 0;
        uint256 reptBalance = rariFundToken.balanceOf(account);
        uint256 fundBalance = getFundBalance();
        uint256 accountBalance = reptBalance.mul(fundBalance).div(reptTotalSupply);
        return accountBalance;
    }

    /**
     * @dev Emitted when funds have been deposited to Rari Eth Pool.
     */
    event Deposit(address indexed sender, address indexed payee, uint256 amount, uint256 reptMinted);

    /**
     * @dev Emitted when funds have been withdrawn from Rari Eth Pool.
     */
    event Withdrawal(address indexed sender, address indexed payee, uint256 amount, uint256 reptBurned);

    /**
     * @notice Internal function to deposit funds from `msg.sender` to Rari Eth Pool in exchange for REPT minted to `to`.
     * Please note that you must approve RariFundManager to transfer at least `amount`.
     * @param to The address that will receieve the minted REPT.
     * @param amount The amount of tokens to be deposited.
     * @return Boolean indicating success.
     */
    function _depositTo(address to, uint256 amount) internal fundEnabled returns (bool) {
        // Input validation
        require(amount > 0, "Deposit amount must be greater than 0.");

        // Calculate REPT to mint
        uint256 reptTotalSupply = rariFundToken.totalSupply();
        uint256 fundBalance = reptTotalSupply > 0 ? getFundBalance() : 0; // Only set if used
        uint256 reptAmount = 0;

        if (reptTotalSupply > 0 && fundBalance > 0) reptAmount = amount.mul(reptTotalSupply).div(fundBalance);
        else reptAmount = amount;

        require(reptAmount > 0, "Deposit amount is so small that no REPT would be minted.");

        // Update net deposits, transfer funds from msg.sender, mint REPT, emit event, and return true
        _netDeposits = _netDeposits.add(int256(amount));

        (bool success, ) = _rariFundControllerContract.call.value(amount)(""); // Transfer ETH to RariFundController
        require(success, "Failed to transfer ETH.");

        require(rariFundToken.mint(to, reptAmount), "Failed to mint output tokens.");

        emit Deposit(msg.sender, to, amount, reptAmount);
    
        // Update RGT distribution speeds
        IRariGovernanceTokenDistributor rariGovernanceTokenDistributor = rariFundToken.rariGovernanceTokenDistributor();
        if (address(rariGovernanceTokenDistributor) != address(0) && block.number < rariGovernanceTokenDistributor.distributionEndBlock()) rariGovernanceTokenDistributor.refreshDistributionSpeeds(IRariGovernanceTokenDistributor.RariPool.Ethereum, getFundBalance());

        return true;
    }

    /**
     * @notice Deposits ETH to RariFund in exchange for REPT.
     * @return Boolean indicating success.
     */
    function deposit() payable external returns (bool) {
        require(_depositTo(msg.sender, msg.value), "Deposit failed.");
        return true;
    }

    /**
     * @dev Deposits funds from `msg.sender` (RariFundProxy) to RariFund in exchange for REPT minted to `to`.
     * @param to The address that will receieve the minted REPT.
     * @return Boolean indicating success.
     */
    function depositTo(address to) payable external returns (bool) {
        require(_depositTo(to, msg.value), "Deposit failed.");
        return true;
    }


    /**
     * @dev Returns the amount of REPT to burn for a withdrawal (used by `_withdrawFrom`).
     * @param from The address from which REPT will be burned.
     * @param amount The amount of the withdrawal in ETH
     */
    function getREPTBurnAmount(address from, uint256 amount) internal returns (uint256) {
        uint256 reptTotalSupply = rariFundToken.totalSupply();
        uint256 fundBalance = getFundBalance();
        require(fundBalance > 0, "Fund balance is zero.");
        uint256 reptAmount = amount.mul(reptTotalSupply).div(fundBalance); // check again
        require(reptAmount <= rariFundToken.balanceOf(from), "Your REPT balance is too low for a withdrawal of this amount.");
        require(reptAmount > 0, "Withdrawal amount is so small that no REPT would be burned.");
        return reptAmount;
    }

    /**
     * @dev Internal function to withdraw funds from RariFund to `msg.sender` in exchange for REPT burned from `from`.
     * Please note that you must approve RariFundManager to burn of the necessary amount of REPT.
     * @param from The address from which REPT will be burned.
     * @param amount The amount of tokens to be withdrawn.
     * @return Boolean indicating success.
     */
    function _withdrawFrom(address from, uint256 amount) internal fundEnabled cachePoolBalance returns (bool) {
        // Input validation
        require(amount > 0, "Withdrawal amount must be greater than 0.");

        // Check contract balance of ETH and withdraw from pools if necessary
        uint256 contractBalance = _rariFundControllerContract.balance;

        for (uint256 i = 0; i < _supportedPools.length; i++) {
            if (contractBalance >= amount) break;
            uint8 pool = _supportedPools[i];
            uint256 poolBalance = getPoolBalance(pool);
            if (poolBalance <= 0) continue;
            uint256 amountLeft = amount.sub(contractBalance);
            uint256 poolAmount = amountLeft < poolBalance ? amountLeft : poolBalance;
            require(rariFundController.withdrawFromPoolKnowingBalance(pool, poolAmount, poolBalance), "Pool withdrawal failed.");
            _poolBalanceCache[pool] = poolBalance.sub(poolAmount);
            contractBalance = contractBalance.add(poolAmount);
        }

        require(amount <= contractBalance, "Available balance not enough to cover amount even after withdrawing from pools.");

        // Calculate REPT to burn
        uint256 reptAmount = getREPTBurnAmount(from, amount);
        
        // Update net deposits, burn REPT, transfer ETH to user, and emit event
        _netDeposits = _netDeposits.sub(int256(amount));
        rariFundToken.fundManagerBurnFrom(from, reptAmount);
        rariFundController.withdrawToManager(amount);
        (bool senderSuccess, ) = msg.sender.call.value(amount)(""); // Transfer 'amount' in ETH to the sender
        require(senderSuccess, "Failed to transfer ETH to sender.");
        emit Withdrawal(from, msg.sender, amount, reptAmount);

        // Update RGT distribution speeds
        IRariGovernanceTokenDistributor rariGovernanceTokenDistributor = rariFundToken.rariGovernanceTokenDistributor();
        if (address(rariGovernanceTokenDistributor) != address(0) && block.number < rariGovernanceTokenDistributor.distributionEndBlock()) rariGovernanceTokenDistributor.refreshDistributionSpeeds(IRariGovernanceTokenDistributor.RariPool.Ethereum, getFundBalance());
        
        return true;
    }

    /**
     * @notice Withdraws funds from RariFund in exchange for REPT.
     * You may only withdraw currencies held by the fund (see `getRawFundBalance(string currencyCode)`).
     * Please note that you must approve RariFundManager to burn of the necessary amount of REPY.
     * @param amount The amount of tokens to be withdrawn.
     * @return Boolean indicating success.
     */
    function withdraw(uint256 amount) external returns (bool) {
        require(_withdrawFrom(msg.sender, amount), "Withdrawal failed.");
        return true;
    }

    /**
     * @dev Withdraws funds from RariFund to `msg.sender` (RariFundProxy) in exchange for REPT burned from `from`.
     * Please note that you must approve RariFundManager to burn of the necessary amount of REPT.
     * @param from The address from which REPT will be burned.
     * @param amount The amount of tokens to be withdrawn.
     * @return Boolean indicating success.
     */
    function withdrawFrom(address from, uint256 amount) external onlyProxy returns (bool) {
        require(_withdrawFrom(from, amount), "Withdrawal failed.");
        return true;
    }

    /**
     * @dev Net quantity of deposits to the fund (i.e., deposits - withdrawals).
     * On deposit, amount deposited is added to `_netDeposits`; on withdrawal, amount withdrawn is subtracted from `_netDeposits`.
     */
    int256 private _netDeposits;
    
    /**
     * @notice Returns the raw total amount of interest accrued by the fund as a whole (including the fees paid on interest) in USD (scaled by 1e18).
     * @dev Ideally, we can add the view modifier, but Compound's `getUnderlyingBalance` function (called by `getRawFundBalance`) potentially modifies the state.
     */
    function getRawInterestAccrued() public returns (int256) {
        return int256(getRawFundBalance()).sub(_netDeposits).add(int256(_interestFeesClaimed));
    }
    
    /**
     * @notice Returns the total amount of interest accrued by past and current REPT holders (excluding the fees paid on interest) in USD (scaled by 1e18).
     * @dev Ideally, we can add the view modifier, but Compound's `getUnderlyingBalance` function (called by `getRawFundBalance`) potentially modifies the state.
     */
    function getInterestAccrued() public returns (int256) {
        return int256(getFundBalance()).sub(_netDeposits);
    }

    /**
     * @dev The proportion of interest accrued that is taken as a service fee (scaled by 1e18).
     */
    uint256 private _interestFeeRate;

    /**
     * @dev Returns the fee rate on interest.
     */
    function getInterestFeeRate() public view returns (uint256) {
        return _interestFeeRate;
    }

    /**
     * @dev Sets the fee rate on interest.
     * @param rate The proportion of interest accrued to be taken as a service fee (scaled by 1e18).
     */
    function setInterestFeeRate(uint256 rate) external fundEnabled onlyOwner cacheRawFundBalance {
        require(rate != _interestFeeRate, "This is already the current interest fee rate.");
        _depositFees();
        _interestFeesGeneratedAtLastFeeRateChange = getInterestFeesGenerated(); // MUST update this first before updating _rawInterestAccruedAtLastFeeRateChange since it depends on it 
        _rawInterestAccruedAtLastFeeRateChange = getRawInterestAccrued();
        _interestFeeRate = rate;
    }

    /**
     * @dev The amount of interest accrued at the time of the most recent change to the fee rate.
     */
    int256 private _rawInterestAccruedAtLastFeeRateChange;

    /**
     * @dev The amount of fees generated on interest at the time of the most recent change to the fee rate.
     */
    int256 private _interestFeesGeneratedAtLastFeeRateChange;

    /**
     * @notice Returns the amount of interest fees accrued by beneficiaries in USD (scaled by 1e18).
     * @dev Ideally, we can add the view modifier, but Compound's `getUnderlyingBalance` function (called by `getRawFundBalance`) potentially modifies the state.
     */
    function getInterestFeesGenerated() public returns (int256) {
        int256 rawInterestAccruedSinceLastFeeRateChange = getRawInterestAccrued().sub(_rawInterestAccruedAtLastFeeRateChange);
        int256 interestFeesGeneratedSinceLastFeeRateChange = rawInterestAccruedSinceLastFeeRateChange.mul(int256(_interestFeeRate)).div(1e18);
        int256 interestFeesGenerated = _interestFeesGeneratedAtLastFeeRateChange.add(interestFeesGeneratedSinceLastFeeRateChange);
        return interestFeesGenerated;
    }

    /**
     * @dev The total claimed amount of interest fees.
     */
    uint256 private _interestFeesClaimed;

    /**
     * @dev Returns the total unclaimed amount of interest fees.
     * Ideally, we can add the view modifier, but Compound's `getUnderlyingBalance` function (called by `getRawFundBalance`) potentially modifies the state.
     */
    function getInterestFeesUnclaimed() public returns (uint256) {
        int256 interestFeesUnclaimed = getInterestFeesGenerated().sub(int256(_interestFeesClaimed));
        return interestFeesUnclaimed > 0 ? uint256(interestFeesUnclaimed) : 0;
    }

    /**
     * @dev The master beneficiary of fees on interest; i.e., the recipient of all fees on interest.
     */
    address payable private _interestFeeMasterBeneficiary;

    /**
     * @dev Sets the master beneficiary of interest fees.
     * @param beneficiary The master beneficiary of fees on interest; i.e., the recipient of all fees on interest.
     */
    function setInterestFeeMasterBeneficiary(address payable beneficiary) external fundEnabled onlyOwner {
        require(beneficiary != address(0), "Master beneficiary cannot be the zero address.");
        _interestFeeMasterBeneficiary = beneficiary;
    }

    /**
     * @dev Emitted when fees on interest are deposited back into the fund.
     */
    event InterestFeeDeposit(address beneficiary, uint256 amount);

    /**
     * @dev Emitted when fees on interest are withdrawn.
     */
    event InterestFeeWithdrawal(address beneficiary, uint256 amountEth);

    /**
     * @dev Internal function to deposit all accrued fees on interest back into the fund on behalf of the master beneficiary.
     * @return Integer indicating success (0), no fees to claim (1), or no REPT to mint (2).
     */
    function _depositFees() internal fundEnabled cacheRawFundBalance returns (uint8) {
        require(_interestFeeMasterBeneficiary != address(0), "Master beneficiary cannot be the zero address.");

        uint256 amount = getInterestFeesUnclaimed();
        if (amount <= 0) return 1;

        uint256 reptTotalSupply = rariFundToken.totalSupply();
        uint256 reptAmount = 0;

        if (reptTotalSupply > 0) {
            uint256 fundBalance = getFundBalance();
            if (fundBalance > 0) reptAmount = amount.mul(reptTotalSupply).div(fundBalance);
            else reptAmount = amount;
        } else reptAmount = amount;

        if (reptAmount <= 0) return 2;

        _interestFeesClaimed = _interestFeesClaimed.add(amount);
        _netDeposits = _netDeposits.add(int256(amount));

        require(rariFundToken.mint(_interestFeeMasterBeneficiary, reptAmount), "Failed to mint output tokens.");
        emit Deposit(_interestFeeMasterBeneficiary, _interestFeeMasterBeneficiary, amount, reptAmount);

        emit InterestFeeDeposit(_interestFeeMasterBeneficiary, amount);

        // Update RGT distribution speeds
        IRariGovernanceTokenDistributor rariGovernanceTokenDistributor = rariFundToken.rariGovernanceTokenDistributor();
        if (address(rariGovernanceTokenDistributor) != address(0) && block.number < rariGovernanceTokenDistributor.distributionEndBlock()) rariGovernanceTokenDistributor.refreshDistributionSpeeds(IRariGovernanceTokenDistributor.RariPool.Ethereum, getFundBalance());

        return 0;
    }

    /**
     * @notice Deposits all accrued fees on interest back into the fund on behalf of the master beneficiary.
     * @return Boolean indicating success.
     */
    function depositFees() external onlyRebalancer returns (bool) {
        uint8 result = _depositFees();
        require(result == 0, result == 2 ? "Deposit amount is so small that no REPT would be minted." : "No new fees are available to claim.");
    }

    /**
     * @notice Withdraws all accrued fees on interest to the master beneficiary.
     * @return Boolean indicating success.
     */
    function withdrawFees() external fundEnabled onlyRebalancer returns (bool) {
        require(_interestFeeMasterBeneficiary != address(0), "Master beneficiary cannot be the zero address.");
        uint256 amount = getInterestFeesUnclaimed();
        require(amount > 0, "No new fees are available to claim.");
        _interestFeesClaimed = _interestFeesClaimed.add(amount);
        rariFundController.withdrawToManager(amount);
        (bool success, ) = _interestFeeMasterBeneficiary.call.value(amount)("");
        require(success, "Failed to transfer ETH.");
        emit InterestFeeWithdrawal(_interestFeeMasterBeneficiary, amount);
        return true;
    }
}
