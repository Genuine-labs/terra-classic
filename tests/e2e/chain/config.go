package chain

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	tmjson "github.com/tendermint/tendermint/libs/json"

	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1types "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	staketypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	util "github.com/classic-terra/core/v2/tests/e2e/util"
)

const (
	TerraDenom        = "uluna"
	StakeDenom        = "stake"
	ChainAID          = "terra-test-a"
	TerraBalanceA     = 100000000000
	StakeBalanceA     = 110000000000
	ChainBID          = "terra-test-b"
	TerraBalanceB     = 200000000000
	StakeBalanceB     = 220000000000
	E2EFeeToken       = "e2e-default-feetoken"
	GenesisFeeBalance = 100000000000
)

var (
	InitBalanceStrA = fmt.Sprintf("%d%s,%d%s", TerraBalanceA, TerraDenom, StakeBalanceA, StakeDenom)
	InitBalanceStrB = fmt.Sprintf("%d%s,%d%s", TerraBalanceB, TerraDenom, StakeBalanceB, StakeDenom)
)

func initGenesis(chain *Chain, votingPeriod time.Duration) error {
	// initialize a genesis file
	configDir := chain.Nodes[0].ConfigDir()
	for _, val := range chain.Nodes {
		addr, err := val.KeyInfo.GetAddress()
		if err != nil {
			return err
		}
		if chain.ChainMeta.Id == ChainAID {
			if err := modifyGenesis(configDir, "", InitBalanceStrA, addr); err != nil {
				return err
			}
		} else if chain.ChainMeta.Id == ChainBID {
			if err := modifyGenesis(configDir, "", InitBalanceStrB, addr); err != nil {
				return err
			}
		}
	}

	// copy the genesis file to the remaining validators
	for _, val := range chain.Nodes[1:] {
		_, err := util.CopyFile(
			filepath.Join(configDir, "config", "genesis.json"),
			filepath.Join(val.ConfigDir(), "config", "genesis.json"),
		)
		if err != nil {
			return err
		}
	}

	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config

	config.SetRoot(chain.Nodes[0].ConfigDir())
	config.Moniker = chain.Nodes[0].Moniker

	genFilePath := config.GenesisFile()
	appGenState, genDoc, err := genutiltypes.GenesisStateFromGenFile(genFilePath)
	if err != nil {
		return err
	}

	err = updateModuleGenesis(appGenState, staketypes.ModuleName, &staketypes.GenesisState{}, updateStakeGenesis)
	if err != nil {
		return err
	}

	err = updateModuleGenesis(appGenState, minttypes.ModuleName, &minttypes.GenesisState{}, updateMintGenesis)
	if err != nil {
		return err
	}

	err = updateModuleGenesis(appGenState, crisistypes.ModuleName, &crisistypes.GenesisState{}, updateCrisisGenesis)
	if err != nil {
		return err
	}

	err = updateModuleGenesis(appGenState, govtypes.ModuleName, &govv1types.GenesisState{}, updateGovGenesis(votingPeriod))
	if err != nil {
		return err
	}

	err = updateModuleGenesis(appGenState, genutiltypes.ModuleName, &genutiltypes.GenesisState{}, updateGenUtilGenesis(chain))
	if err != nil {
		return err
	}

	bz, err := json.MarshalIndent(appGenState, "", "  ")
	if err != nil {
		return err
	}

	genDoc.AppState = bz

	genesisJson, err := tmjson.MarshalIndent(genDoc, "", "  ")
	if err != nil {
		return err
	}

	// write the updated genesis file to each validator
	for _, val := range chain.Nodes {
		if err := util.WriteFile(filepath.Join(val.ConfigDir(), "config", "genesis.json"), genesisJson); err != nil {
			return err
		}
	}
	return nil
}

func updateStakeGenesis(stakeGenState *staketypes.GenesisState) {
	stakeGenState.Params = staketypes.Params{
		BondDenom:         TerraDenom,
		MaxValidators:     100,
		MaxEntries:        7,
		HistoricalEntries: 10000,
		UnbondingTime:     240000000000,
		MinCommissionRate: sdk.ZeroDec(),
	}
}

func updateMintGenesis(mintGenState *minttypes.GenesisState) {
	mintGenState.Params.MintDenom = TerraDenom
}

func updateCrisisGenesis(crisisGenState *crisistypes.GenesisState) {
	crisisGenState.ConstantFee.Denom = TerraDenom
}

func updateGovGenesis(votingPeriod time.Duration) func(*govv1types.GenesisState) {
	return func(govGenState *govv1types.GenesisState) {
		maxDepositPeriod := 10 * time.Minute

		govGenState.VotingParams.VotingPeriod = &votingPeriod
		govGenState.DepositParams.MinDeposit = sdk.Coins{sdk.NewInt64Coin(TerraDenom, 10_000_000)}
		govGenState.DepositParams.MaxDepositPeriod = &maxDepositPeriod
		govGenState.TallyParams.Quorum = "0.000000000000000001"
		govGenState.TallyParams.Threshold = "0.000000000000000001"
	}
}

func updateGenUtilGenesis(c *Chain) func(*genutiltypes.GenesisState) {
	StakeAmountCoinA := sdk.NewCoin(StakeDenom, sdk.NewInt(StakeBalanceA))
	StakeAmountCoinB := sdk.NewCoin(StakeDenom, sdk.NewInt(StakeBalanceB))
	return func(genUtilGenState *genutiltypes.GenesisState) {
		// generate genesis txs
		genTxs := make([]json.RawMessage, 0, len(c.Nodes))
		for _, node := range c.Nodes {
			if !node.IsValidator {
				continue
			}

			stakeAmountCoin := StakeAmountCoinA
			if c.ChainMeta.Id != ChainAID {
				stakeAmountCoin = StakeAmountCoinB
			}
			createValmsg, err := node.buildCreateValidatorMsg(stakeAmountCoin)

			const genesisSetupFailed = "genutil genesis setup failed: "
			if err != nil {
				panic(genesisSetupFailed + err.Error())
			}

			signedTx, err := node.signMsg(createValmsg)
			if err != nil {
				panic(genesisSetupFailed + err.Error())
			}

			txRaw, err := util.EncodingConfig.Marshaler.MarshalJSON(signedTx)
			if err != nil {
				panic(genesisSetupFailed + err.Error())
			}
			genTxs = append(genTxs, txRaw)
		}
		genUtilGenState.GenTxs = genTxs
	}
}
