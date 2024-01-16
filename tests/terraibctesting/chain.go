package terraibctesting

// import (
// 	"encoding/json"
// 	"math/rand"
// 	"time"

// 	simtestutil "github.com/cosmos/cosmos-sdk/simapp/helpers"

// 	"github.com/cosmos/cosmos-sdk/baseapp"
// 	"github.com/cosmos/cosmos-sdk/client"
// 	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
// 	sdk "github.com/cosmos/cosmos-sdk/types"
// 	ibctesting "github.com/cosmos/ibc-go/v6/testing"
// 	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

// 	"github.com/classic-terra/core/v2/app"
// )

// const SimAppChainID = "simulation-app"

// type TestChain struct {
// 	*ibctesting.TestChain
// }

// func SetupTestingApp() (ibctesting.TestingApp, map[string]json.RawMessage) {
// 	terraApp := app.Setup(false)
// 	return terraApp, app.NewDefaultGenesisState()
// }

// // SendMsgsNoCheck is an alternative to ibctesting.TestChain.SendMsgs so that it doesn't check for errors. That should be handled by the caller
// func (chain *TestChain) SendMsgsNoCheck(msgs ...sdk.Msg) (*sdk.Result, error) {
// 	// ensure the chain has the latest time
// 	chain.Coordinator.UpdateTimeForChain(chain.TestChain)

// 	_, r, err := SignAndDeliver(
// 		chain.TxConfig,
// 		chain.App.GetBaseApp(),
// 		chain.GetContext().BlockHeader(),
// 		msgs,
// 		chain.ChainID,
// 		[]uint64{chain.SenderAccount.GetAccountNumber()},
// 		[]uint64{chain.SenderAccount.GetSequence()},
// 		chain.SenderPrivKey,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// SignAndDeliver calls app.Commit()
// 	chain.NextBlock()

// 	// increment sequence for successful transaction execution
// 	err = chain.SenderAccount.SetSequence(chain.SenderAccount.GetSequence() + 1)
// 	if err != nil {
// 		return nil, err
// 	}

// 	chain.Coordinator.IncrementTime()

// 	return r, nil
// }

// // SignAndDeliver signs and delivers a transaction without asserting the results. This overrides the function
// // from ibctesting
// func SignAndDeliver(
// 	txCfg client.TxConfig, app *baseapp.BaseApp, header tmproto.Header, msgs []sdk.Msg,
// 	chainID string, accNums, accSeqs []uint64, priv ...cryptotypes.PrivKey,
// ) (sdk.GasInfo, *sdk.Result, error) {
// 	tx, err := simtestutil.GenSignedMockTx(
// 		rand.New(rand.NewSource(time.Now().UnixNano())),
// 		txCfg,
// 		msgs,
// 		sdk.Coins{sdk.NewInt64Coin(sdk.DefaultBondDenom, 0)},
// 		simtestutil.DefaultGenTxGas,
// 		chainID,
// 		accNums,
// 		accSeqs,
// 		priv...,
// 	)
// 	if err != nil {
// 		return sdk.GasInfo{}, nil, err
// 	}

// 	// Simulate a sending a transaction
// 	gInfo, res, err := app.SimDeliver(txCfg.TxEncoder(), tx)

// 	return gInfo, res, err
// }

// // GetTerraApp returns the current chain's app as an TerraApp
// func (chain *TestChain) GetTerraApp() *app.TerraApp {
// 	v, _ := chain.App.(*app.TerraApp)
// 	return v
// }
