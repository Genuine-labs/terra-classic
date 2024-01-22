package chain

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cosmossdk.io/math"
	"github.com/spf13/viper"
	tmconfig "github.com/tendermint/tendermint/config"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/cosmos/cosmos-sdk/codec/unknownproto"
	"github.com/cosmos/cosmos-sdk/crypto"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"

	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	txsigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"

	"github.com/cosmos/cosmos-sdk/x/genutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	app "github.com/classic-terra/core/v2/app"
	"github.com/classic-terra/core/v2/tests/e2e/util"
)

type Node struct {
	Chain        *Chain
	Moniker      string
	Mnemonic     string
	KeyInfo      keyring.Record
	PrivateKey   cryptotypes.PrivKey
	ConsensusKey privval.FilePVKey
	NodeKey      p2p.NodeKey
	PeerId       string
	IsValidator  bool
}

func (n *Node) init() error {
	if err := n.createConfig(); err != nil {
		return err
	}

	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config

	config.SetRoot(n.ConfigDir())
	config.Moniker = n.Moniker

	genDoc, err := n.getGenesisDoc()
	if err != nil {
		return err
	}

	appState, err := json.MarshalIndent(app.ModuleBasics.DefaultGenesis(util.EncodingConfig.Marshaler), "", " ")
	if err != nil {
		return fmt.Errorf("failed to JSON encode app genesis state: %w", err)
	}

	genDoc.ChainID = n.Chain.ChainMeta.Id
	genDoc.Validators = nil
	genDoc.AppState = appState

	if err = genutil.ExportGenesisFile(genDoc, config.GenesisFile()); err != nil {
		return fmt.Errorf("failed to export app genesis state: %w", err)
	}

	tmconfig.WriteConfigFile(filepath.Join(config.RootDir, "config", "config.toml"), config)
	return nil
}

func (n *Node) getGenesisDoc() (*tmtypes.GenesisDoc, error) {
	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config
	config.SetRoot(n.ConfigDir())

	genFile := config.GenesisFile()
	doc := &tmtypes.GenesisDoc{}

	if _, err := os.Stat(genFile); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		var err error

		doc, err = tmtypes.GenesisDocFromFile(genFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read genesis doc from file: %w", err)
		}
	}

	return doc, nil
}

func (n *Node) createKey(name string) error {
	mnemonic, err := util.CreateMnemonic()
	if err != nil {
		return err
	}

	return n.createKeyFromMnemonic(name, mnemonic)
}

func (n *Node) createNodeKey() error {
	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config

	config.SetRoot(n.ConfigDir())
	config.Moniker = n.Moniker

	nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())
	if err != nil {
		return err
	}

	n.NodeKey = *nodeKey
	return nil
}

func (n *Node) createConsensusKey() error {
	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config

	config.SetRoot(n.ConfigDir())
	config.Moniker = n.Moniker

	pvKeyFile := config.PrivValidatorKeyFile()
	if err := tmos.EnsureDir(filepath.Dir(pvKeyFile), 0o777); err != nil {
		return err
	}

	pvStateFile := config.PrivValidatorStateFile()
	if err := tmos.EnsureDir(filepath.Dir(pvStateFile), 0o777); err != nil {
		return err
	}

	filePV := privval.LoadOrGenFilePV(pvKeyFile, pvStateFile)
	n.ConsensusKey = filePV.Key

	return nil
}

func (n *Node) createKeyFromMnemonic(name, mnemonic string) error {
	kb, err := keyring.New(keyringAppName, keyring.BackendTest, n.ConfigDir(), nil, util.EncodingConfig.Marshaler)
	if err != nil {
		return err
	}

	keyringAlgos, _ := kb.SupportedAlgorithms()
	algo, err := keyring.NewSigningAlgoFromString(string(hd.Secp256k1Type), keyringAlgos)
	if err != nil {
		return err
	}

	info, err := kb.NewAccount(name, mnemonic, "", sdk.FullFundraiserPath, algo)
	if err != nil {
		return err
	}

	privKeyArmor, err := kb.ExportPrivKeyArmor(name, keyringPassphrase)
	if err != nil {
		return err
	}

	privKey, _, err := crypto.UnarmorDecryptPrivKey(privKeyArmor, keyringPassphrase)
	if err != nil {
		return err
	}

	n.KeyInfo = *info
	n.Mnemonic = mnemonic
	n.PrivateKey = privKey

	return nil
}

func (n *Node) createConfig() error {
	p := path.Join(n.ConfigDir(), "config")
	return os.MkdirAll(p, 0o755)
}

func (n *Node) ConfigDir() string {
	return fmt.Sprintf("%s/%s", n.Chain.ChainMeta.configDir(), n.Moniker)
}

func (n *Node) getNodeKey() *p2p.NodeKey {
	return &n.NodeKey
}

func (n *Node) initNodeConfigs(persistentPeers []string) error {
	tmCfgPath := filepath.Join(n.ConfigDir(), "config", "config.toml")

	vpr := viper.New()
	vpr.SetConfigFile(tmCfgPath)
	if err := vpr.ReadInConfig(); err != nil {
		return err
	}

	valConfig := tmconfig.DefaultConfig()
	if err := vpr.Unmarshal(valConfig); err != nil {
		return err
	}

	valConfig.P2P.ListenAddress = "tcp://0.0.0.0:26656"
	valConfig.P2P.AddrBookStrict = false
	valConfig.P2P.ExternalAddress = fmt.Sprintf("%s:%d", n.Moniker, 26656)
	valConfig.RPC.ListenAddress = "tcp://0.0.0.0:26657"
	valConfig.StateSync.Enable = false
	valConfig.LogLevel = "info"
	valConfig.P2P.PersistentPeers = strings.Join(persistentPeers, ",")
	valConfig.Storage.DiscardABCIResponses = true

	valConfig.Consensus.TimeoutPropose = time.Millisecond * 300
	valConfig.Consensus.TimeoutProposeDelta = 0
	valConfig.Consensus.TimeoutPrevote = 0
	valConfig.Consensus.TimeoutPrevoteDelta = 0
	valConfig.Consensus.TimeoutPrecommit = 0
	valConfig.Consensus.TimeoutPrecommitDelta = 0
	valConfig.Consensus.TimeoutCommit = 0

	tmconfig.WriteConfigFile(tmCfgPath, valConfig)
	return nil
}

func (n *Node) buildCreateValidatorMsg(amount sdk.Coin) (sdk.Msg, error) {
	description := stakingtypes.NewDescription(n.Moniker, "", "", "", "")
	commissionRates := stakingtypes.CommissionRates{
		Rate:          sdk.MustNewDecFromStr("0.1"),
		MaxRate:       sdk.MustNewDecFromStr("0.2"),
		MaxChangeRate: sdk.MustNewDecFromStr("0.01"),
	}

	// get the initial validator min self delegation
	minSelfDelegation := math.OneInt()

	valPubKey, err := cryptocodec.FromTmPubKeyInterface(n.ConsensusKey.PubKey)
	if err != nil {
		return nil, err
	}

	addr, err := n.KeyInfo.GetAddress()
	if err != nil {
		return nil, err
	}

	return stakingtypes.NewMsgCreateValidator(
		sdk.ValAddress(addr),
		valPubKey,
		amount,
		description,
		commissionRates,
		minSelfDelegation,
	)
}

// signMsg returns a signed tx of the provided messages,
// signed by the validator, using 0 fees, a high gas limit, and a common memo.
func (n *Node) signMsg(msgs ...sdk.Msg) (*sdktx.Tx, error) {
	txBuilder := util.EncodingConfig.TxConfig.NewTxBuilder()

	if err := txBuilder.SetMsgs(msgs...); err != nil {
		return nil, err
	}

	txBuilder.SetMemo(fmt.Sprintf("%s@%s:26656", n.NodeKey.ID(), n.Moniker))
	txBuilder.SetFeeAmount(sdk.NewCoins())
	txBuilder.SetGasLimit(uint64(200000 * len(msgs)))

	// TODO: Find a better way to sign this tx with less code.
	signerData := authsigning.SignerData{
		ChainID:       n.Chain.ChainMeta.Id,
		AccountNumber: 0,
		Sequence:      0,
	}

	// For SIGN_MODE_DIRECT, calling SetSignatures calls setSignerInfos on
	// TxBuilder under the hood, and SignerInfos is needed to generate the sign
	// bytes. This is the reason for setting SetSignatures here, with a nil
	// signature.
	//
	// Note: This line is not needed for SIGN_MODE_LEGACY_AMINO, but putting it
	// also doesn't affect its generated sign bytes, so for code's simplicity
	// sake, we put it here.
	pubkey, err := n.KeyInfo.GetPubKey()
	if err != nil {
		return nil, err
	}

	sig := txsigning.SignatureV2{
		PubKey: pubkey,
		Data: &txsigning.SingleSignatureData{
			SignMode:  txsigning.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
		Sequence: 0,
	}

	if err := txBuilder.SetSignatures(sig); err != nil {
		return nil, err
	}

	bytesToSign, err := util.EncodingConfig.TxConfig.SignModeHandler().GetSignBytes(
		txsigning.SignMode_SIGN_MODE_DIRECT,
		signerData,
		txBuilder.GetTx(),
	)
	if err != nil {
		return nil, err
	}

	sigBytes, err := n.PrivateKey.Sign(bytesToSign)
	if err != nil {
		return nil, err
	}

	sig = txsigning.SignatureV2{
		PubKey: pubkey,
		Data: &txsigning.SingleSignatureData{
			SignMode:  txsigning.SignMode_SIGN_MODE_DIRECT,
			Signature: sigBytes,
		},
		Sequence: 0,
	}
	if err := txBuilder.SetSignatures(sig); err != nil {
		return nil, err
	}

	signedTx := txBuilder.GetTx()
	bz, err := util.EncodingConfig.TxConfig.TxEncoder()(signedTx)
	if err != nil {
		return nil, err
	}

	return decodeTx(bz)
}

func decodeTx(txBytes []byte) (*sdktx.Tx, error) {
	var raw sdktx.TxRaw

	// reject all unknown proto fields in the root TxRaw
	err := unknownproto.RejectUnknownFieldsStrict(txBytes, &raw, util.EncodingConfig.InterfaceRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to reject unknown fields: %w", err)
	}

	if err := util.EncodingConfig.Marshaler.Unmarshal(txBytes, &raw); err != nil {
		return nil, err
	}

	var body sdktx.TxBody
	if err := util.EncodingConfig.Marshaler.Unmarshal(raw.BodyBytes, &body); err != nil {
		return nil, fmt.Errorf("failed to decode tx: %w", err)
	}

	var authInfo sdktx.AuthInfo

	// reject all unknown proto fields in AuthInfo
	err = unknownproto.RejectUnknownFieldsStrict(raw.AuthInfoBytes, &authInfo, util.EncodingConfig.InterfaceRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to reject unknown fields: %w", err)
	}

	if err := util.EncodingConfig.Marshaler.Unmarshal(raw.AuthInfoBytes, &authInfo); err != nil {
		return nil, fmt.Errorf("failed to decode auth info: %w", err)
	}

	return &sdktx.Tx{
		Body:       &body,
		AuthInfo:   &authInfo,
		Signatures: raw.Signatures,
	}, nil
}
