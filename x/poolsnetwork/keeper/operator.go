package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/bloxapp/pools-network/shared/types"
	poolTypes "github.com/bloxapp/pools-network/x/poolsnetwork/types"
)

func (k Keeper) CreateOperator(ctx sdk.Context, operator poolTypes.Operator) error {
	if err := k.setOperator(ctx, operator); err != nil {
		return sdkerrors.Wrap(err, "could not set operator")
	}

	// get operator with ref
	operRef, found, err := k.GetOperator(ctx, operator.ConsensusAddress)
	if err != nil {
		return sdkerrors.Wrap(err, "could not get operator with ref")
	}
	if !found {
		return sdkerrors.Wrap(poolTypes.ErrOperatorNotFound, "")
	}

	// mint
	coin := sdk.NewInt64Coin("stake", int64(operator.EthStake))
	_, err = k.StakingKeeper.Delegate(
		ctx,
		sdk.AccAddress(operator.ConsensusAddress),
		coin.Amount,
		sdk.Unbonded,
		*operRef.CosmosValidatorRef,
		true,
	)
	if err != nil {
		return sdkerrors.Wrap(err, "Could not self delegate to new operator")
	}

	return nil
}

func (k Keeper) UpdateOperator(ctx sdk.Context, operator poolTypes.Operator) {

}

func (k Keeper) DeleteOperator(ctx sdk.Context, address types.ConsensusAddress) {
	// delete from pools module
	store := ctx.KVStore(k.storeKey)
	store.Delete(address)

	shares, err := k.StakingKeeper.ValidateUnbondAmount(ctx, sdk.AccAddress(address), sdk.ValAddress(address), sdk.TokensFromConsensusPower(10))
	if err != nil {

	}
	_, err = k.StakingKeeper.Undelegate(ctx, sdk.AccAddress(address), sdk.ValAddress(address), shares)
	if err != nil {

	}

	// delete from cosmos
	k.StakingKeeper.RemoveValidator(ctx, sdk.ValAddress(address))
}

func (k Keeper) GetOperator(ctx sdk.Context, address types.ConsensusAddress) (operator poolTypes.Operator, found bool, err error) {
	store := ctx.KVStore(k.storeKey)
	byts := store.Get(address)

	if byts == nil || len(byts) == 0 {
		return poolTypes.Operator{}, false, nil
	}

	// unmarshal
	ret := poolTypes.Operator{}
	err = ret.Unmarshal(byts)
	if err != nil {
		return poolTypes.Operator{}, false, sdkerrors.Wrap(err, "Could not unmarshal operator")
	}

	// attach cosmos validator ref
	val, found := k.StakingKeeper.GetValidator(ctx, sdk.ValAddress(ret.ConsensusAddress))
	if !found {
		return poolTypes.Operator{}, false, sdkerrors.Wrap(poolTypes.ErrNoStakingValidatorForOperator, "")
	}
	ret.CosmosValidatorRef = &val

	return ret, true, nil
}

// SetOperator is responsible for saving the pools operator and it's reference cosmos validator.
// This is an important relationship as an operator should be identified i a one-to-one relationship with a
// cosmos validator for the consensus to work.
func (k Keeper) setOperator(ctx sdk.Context, operator poolTypes.Operator) error {
	store := ctx.KVStore(k.storeKey)

	revert := func() {
		k.DeleteOperator(ctx, operator.ConsensusAddress)
	}

	cpy := operator.CopyWithoutValidatorRef()
	byts, err := cpy.Marshal()
	if err != nil {
		revert()
		return sdkerrors.Wrap(err, "Could not set operator")
	}
	store.Set(cpy.ConsensusAddress, byts)

	// An operator is a wrapper around the native staking validator found in the staking module
	// https://github.com/cosmos/cosmos-sdk/tree/master/x/staking
	// When setting an operator we should also be setting a dedicated validator
	pk, err := sdk.GetPubKeyFromBech32(sdk.Bech32PubKeyTypeConsPub, operator.ConsensusPk)
	if err != nil {
		revert()
		return sdkerrors.Wrap(err, "Could not set validator for staking module")
	}
	val := stakingTypes.NewValidator(sdk.ValAddress(operator.ConsensusAddress), pk, stakingTypes.Description{})

	k.StakingKeeper.SetValidator(ctx, val)
	k.StakingKeeper.SetValidatorByConsAddr(ctx, val)
	k.StakingKeeper.SetValidatorByPowerIndex(ctx, val)
	k.StakingKeeper.AfterValidatorCreated(ctx, val.GetOperator())

	return nil
}
