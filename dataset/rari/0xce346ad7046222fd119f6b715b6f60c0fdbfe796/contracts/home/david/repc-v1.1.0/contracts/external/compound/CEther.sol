pragma solidity 0.5.17;

interface CEther {
  function mint() external payable;
  function redeemUnderlying(uint redeemAmount) external returns (uint);
  function redeem(uint redeemTokens) external returns (uint);
  function borrow(uint borrowAmount) external returns (uint);
  function repayBorrow() external payable;
  function borrowBalanceCurrent(address account) external returns (uint);
  function exchangeRateCurrent() external returns (uint);
  function balanceOfUnderlying(address owner) external returns (uint);

  function balanceOf(address account) external view returns (uint);
  function decimals() external view returns (uint);
}