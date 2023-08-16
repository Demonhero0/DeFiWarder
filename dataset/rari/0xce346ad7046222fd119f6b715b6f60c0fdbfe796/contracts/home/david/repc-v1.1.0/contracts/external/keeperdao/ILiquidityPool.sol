pragma solidity 0.5.17;

import "./IKToken.sol";


interface ILiquidityPool {
    function () external payable;
    function kToken(address _token) external view returns (IKToken);
    function register(IKToken _kToken) external;
    function renounceOperator() external;
    function deposit(address _token, uint256 _amount) external payable returns (uint256);
    function withdraw(address payable _to, IKToken _kToken, uint256 _kTokenAmount) external;
    function borrowableBalance(address _token) external view returns (uint256);
    function underlyingBalance(address _token, address _owner) external view returns (uint256);
}