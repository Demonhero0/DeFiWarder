pragma solidity 0.5.16;

import "@openzeppelin/contracts-ethereum-package/contracts/math/SafeMath.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/token/ERC20/SafeERC20.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/token/ERC20/ERC20Detailed.sol";
import "@openzeppelin/contracts-ethereum-package/contracts/utils/ReentrancyGuard.sol";

contract ForceProfitSharing is ERC20, ERC20Detailed, ReentrancyGuard {
    using SafeMath for uint256;
    IERC20 public force;

    event Withdraw(address indexed beneficiary, uint256 amount);
    event Deposit(address indexed beneficiary, uint256 amount);

    // Define the Force token contract
    constructor(address _underlying) public {
        force = IERC20(_underlying);
        ERC20Detailed.initialize(
            "xFORCE",
            "xFORCE",
            ERC20Detailed(_underlying).decimals()
        );
        ReentrancyGuard.initialize();
    }

    function deposit(uint256 amount) external nonReentrant {
        // Gets the amount of Force locked in the contract
        uint256 totalForce = force.balanceOf(address(this));
        // Gets the amount of xForce in existence
        uint256 totalShares = totalSupply();
        // If no xForce exists, mint it 1:1 to the amount put in
        if (totalShares == 0 || totalForce == 0) {
            _mint(msg.sender, amount);
        }
        // Calculate and mint the amount of xForce the Force is worth. The ratio will change overtime, as xForce is burned/minted and Force deposited + gained from fees / withdrawn.
        else {
            uint256 what = amount.mul(totalShares).div(totalForce);
            _mint(msg.sender, what);
        }
        // Lock the Force in the contract
        force.transferFrom(msg.sender, address(this), amount);

        emit Deposit(msg.sender, amount);
    }

    function withdraw(uint256 numberOfShares) external nonReentrant {
        // Gets the amount of xForce in existence
        uint256 totalShares = totalSupply();
        // Calculates the amount of Force the xForce is worth
        uint256 what =
            numberOfShares.mul(force.balanceOf(address(this))).div(totalShares);
        _burn(msg.sender, numberOfShares);
        force.transfer(msg.sender, what);

        emit Withdraw(msg.sender, what);
    }
}
