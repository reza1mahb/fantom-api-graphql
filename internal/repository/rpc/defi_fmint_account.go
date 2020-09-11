/*
Package rpc implements bridge to Lachesis full node API interface.

We recommend using local IPC for fast and the most efficient inter-process communication between the API server
and an Opera/Lachesis node. Any remote RPC connection will work, but the performance may be significantly degraded
by extra networking overhead of remote RPC calls.

You should also consider security implications of opening Lachesis RPC interface for a remote access.
If you considering it as your deployment strategy, you should establish encrypted channel between the API server
and Lachesis RPC interface with connection limited to specified endpoints.

We strongly discourage opening Lachesis RPC interface for unrestricted Internet access.
*/
package rpc

//go:generate abigen --abi ./contracts/price-oracle-proxy-interface.abi --pkg rpc --type PriceOracleProxyInterface --out ./smc_oracle_proxy.go

import (
	"fantom-api-graphql/internal/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
)

// FMintAccount loads details of a DeFi/fMint protocol account identified by the owner address.
func (ftm *FtmBridge) FMintAccount(owner *common.Address) (*types.FMintAccount, error) {
	// make the container
	var err error
	da := types.FMintAccount{Address: *owner}

	// load list of collateral tokens
	da.CollateralList, err = ftm.DefiTokenList()
	if err != nil {
		ftm.log.Errorf("collateral tokens list loader failed; %s", err.Error())
		return nil, err
	}

	// debt tokens are the same
	da.DebtList = da.CollateralList

	// get the current values of the account tokens on both collateral and debt
	da.CollateralValue, da.DebtValue, err = ftm.fMintAccountValue(*owner)
	if err != nil {
		ftm.log.Errorf("can not pull account tokens value; %s", err.Error())
		return nil, err
	}

	// return the account detail
	return &da, nil
}

// FMintTokenBalance loads balance of a single DeFi token in fMint contract by it's address.
func (ftm *FtmBridge) FMintTokenBalance(owner *common.Address, token *common.Address, tp types.DefiTokenType) (hexutil.Big, error) {
	// connect the contract
	contract, err := ftm.fMintCfg.fMintMinterContract()
	if err != nil {
		return hexutil.Big{}, err
	}

	// get the collateral token balance
	var val *big.Int

	// pull the right value based to token type
	switch tp {
	case types.DefiTokenTypeCollateral:
		val, err = contract.CollateralBalance(nil, *owner, *token)
	case types.DefiTokenTypeDebt:
		val, err = contract.DebtBalance(nil, *owner, *token)
	}

	// do we have the value?
	if val == nil {
		ftm.log.Debugf("token %s balance not available for owner %s", token.String(), owner.String())
		return hexutil.Big{}, err
	}

	return hexutil.Big(*val), err
}

// FMintTokenValue loads value of a single DeFi token by it's address in fUSD.
func (ftm *FtmBridge) FMintTokenValue(owner *common.Address, token *common.Address, tp types.DefiTokenType) (hexutil.Big, error) {
	// get the balance
	balance, err := ftm.FMintTokenBalance(owner, token, tp)
	if err != nil {
		ftm.log.Errorf("token %s balance unknown; %s", token.String(), err.Error())
		return hexutil.Big{}, err
	}

	// get the price for the given token from oracle
	val, err := ftm.FMintTokenPrice(token)
	if err != nil {
		ftm.log.Errorf("token %s price not available; %s", token.String(), err.Error())
		return hexutil.Big{}, err
	}

	// calculate the target value
	value := new(big.Int).Mul(val.ToInt(), balance.ToInt())
	return hexutil.Big(*value), nil
}

// DefiTokenPrice loads the current price of the given token from on-chain price oracle.
func (ftm *FtmBridge) FMintTokenPrice(token *common.Address) (hexutil.Big, error) {
	// get the price oracle address
	oracle, err := ftm.fMintCfg.priceOracleProxyContract()
	if err != nil {
		return hexutil.Big{}, err
	}

	// get the price for the given token from oracle
	val, err := oracle.GetPrice(nil, *token)
	if err != nil {
		ftm.log.Errorf("price not available for token %s; %s", token.String(), err.Error())
		return hexutil.Big{}, err
	}

	// do we have the value?
	if val == nil {
		ftm.log.Debugf("token %s has no value", token.String())
		return hexutil.Big{}, nil
	}

	return hexutil.Big(*val), nil
}

// fMintAccountTokensValue loads total value status of a given fMint account.
func (ftm *FtmBridge) fMintAccountValue(owner common.Address) (hexutil.Big, hexutil.Big, error) {
	// connect the contract
	contract, err := ftm.fMintCfg.fMintMinterContract()
	if err != nil {
		return hexutil.Big{}, hexutil.Big{}, err
	}

	// get joined collateral value
	cValue, err := contract.CollateralValueOf(nil, owner)
	if err != nil {
		ftm.log.Errorf("joined collateral value loader failed")
		return hexutil.Big{}, hexutil.Big{}, err
	}

	// get joined debt value
	dValue, err := contract.DebtValueOf(nil, owner)
	if err != nil {
		ftm.log.Errorf("joined debt value loader failed")
		return hexutil.Big{}, hexutil.Big{}, err
	}

	// return the value we got
	return hexutil.Big(*cValue), hexutil.Big(*dValue), nil
}