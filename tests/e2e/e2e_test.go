package e2e

import "fmt"

// import (
// 	"fmt"
// 	"os"
// 	"path/filepath"
// 	"strconv"
// 	"strings"
// 	"time"

// 	ibchookskeeper "github.com/terra-money/core/v2/x/ibc-hooks/keeper"

// 	"github.com/classic-terra/core/v2/tests/e2e/initialization"
// 	"github.com/classic-terra/core/v2/tests/e2e/util"
// )

// func (s *IntegrationTestSuite) TestIBCWasmHooks() {
// 	chainA := s.chains[0]
// 	chainB := s.chains[1]

// 	nodeA := chainA.Nodes[0]
// 	nodeB := chainB.Nodes[0]

// 	// copy the contract from x/rate-limit/testdata/
// 	wd, err := os.Getwd()
// 	s.NoError(err)
// 	// co up two levels
// 	projectDir := filepath.Dir(filepath.Dir(wd))
// 	_, err = util.CopyFile(projectDir+"/tests/ibc-hooks/bytecode/counter.wasm", wd+"/scripts/counter.wasm")
// 	s.NoError(err)

// 	nodeA.StoreWasmCode("counter.wasm", initialization.ValidatorWalletName)
// 	chainA.LatestCodeId = int(nodeA.QueryLatestWasmCodeID())
// 	nodeA.InstantiateWasmContract(
// 		strconv.Itoa(chainA.LatestCodeId),
// 		`{"count": 0}`,
// 		initialization.ValidatorWalletName)

// 	contracts, err := nodeA.QueryContractsFromId(chainA.LatestCodeId)
// 	s.NoError(err)
// 	s.Require().Len(contracts, 1, "Wrong number of contracts for the counter")
// 	contractAddr := contracts[0]

// 	transferAmount := int64(10)
// 	validatorAddr := nodeB.GetWallet(initialization.ValidatorWalletName)
// 	nodeB.SendIBCTransfer(validatorAddr, contractAddr, fmt.Sprintf("%dluna", transferAmount),
// 		fmt.Sprintf(`{"wasm":{"contract":"%s","msg": {"increment": {}} }}`, contractAddr))

// 	// check the balance of the contract
// 	s.Eventually(func() bool {
// 		balance, err := nodeA.QueryBalances(contractAddr)
// 		s.Require().NoError(err)
// 		if len(balance) == 0 {
// 			return false
// 		}
// 		return balance[0].Amount.Int64() == transferAmount
// 	},
// 		1*time.Minute,
// 		10*time.Millisecond,
// 	)

// 	// sender wasm addr
// 	senderBech32, err := ibchookskeeper.DeriveIntermediateSender("channel-0", validatorAddr, "terra")

// 	var response map[string]interface{}
// 	s.Eventually(func() bool {
// 		response, err = nodeA.QueryWasmSmart(contractAddr, fmt.Sprintf(`{"get_total_funds": {"addr": "%s"}}`, senderBech32))
// 		totalFunds := response["total_funds"].([]interface{})[0]
// 		amount := totalFunds.(map[string]interface{})["amount"].(string)
// 		denom := totalFunds.(map[string]interface{})["denom"].(string)
// 		// check if denom contains "luna"
// 		return err == nil && amount == strconv.FormatInt(transferAmount, 10) && strings.Contains(denom, "ibc")
// 	},
// 		15*time.Second,
// 		10*time.Millisecond,
// 	)
// }


func (s *IntegrationTestSuite) TestBankTokenTransfer() {
	s.Run("send_photon_between_accounts", func() {
		var err error
		senderAddress, err := s.chainConfigs[0].NodeConfigs[0].KeyInfo.GetAddress()
		s.Require().NoError(err)
		sender := senderAddress.String()

		recipientAddress, err := s.chainConfigs[0].NodeConfigs[0].KeyInfo.GetAddress()
		s.Require().NoError(err)

		recipient := recipientAddress.String()
		fmt.Println("sender: ", sender)
		fmt.Println("recipient: ", recipient)	
		// chainAAPIEndpoint := fmt.Sprintf("http://%s", s.valResources[s.chainA.id][0].GetHostPort("1317/tcp"))

		// var (
		// 	beforeSenderUAtomBalance    sdk.Coin
		// 	beforeRecipientUAtomBalance sdk.Coin
		// )

		// s.Require().Eventually(
		// 	func() bool {
		// 		beforeSenderUAtomBalance, err = getSpecificBalance(chainAAPIEndpoint, sender, uatomDenom)
		// 		s.Require().NoError(err)

		// 		beforeRecipientUAtomBalance, err = getSpecificBalance(chainAAPIEndpoint, recipient, uatomDenom)
		// 		s.Require().NoError(err)

		// 		return beforeSenderUAtomBalance.IsValid() && beforeRecipientUAtomBalance.IsValid()
		// 	},
		// 	10*time.Second,
		// 	5*time.Second,
		// )

		// s.execBankSend(s.chainA, 0, sender, recipient, tokenAmount.String(), standardFees.String(), false)

		// s.Require().Eventually(
		// 	func() bool {
		// 		afterSenderUAtomBalance, err := getSpecificBalance(chainAAPIEndpoint, sender, uatomDenom)
		// 		s.Require().NoError(err)

		// 		afterRecipientUAtomBalance, err := getSpecificBalance(chainAAPIEndpoint, recipient, uatomDenom)
		// 		s.Require().NoError(err)

		// 		decremented := beforeSenderUAtomBalance.Sub(tokenAmount).Sub(standardFees).IsEqual(afterSenderUAtomBalance)
		// 		incremented := beforeRecipientUAtomBalance.Add(tokenAmount).IsEqual(afterRecipientUAtomBalance)

		// 		return decremented && incremented
		// 	},
		// 	time.Minute,
		// 	5*time.Second,
		// )
	})
}
