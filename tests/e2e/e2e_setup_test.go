package e2e

import (
	// "bytes"
	// "context"
	// "encoding/json"
	"fmt"
	// "io"
	// "net/http"
	"os"
	// "path"
	// "path/filepath"
	"testing"
	// "time"

	// "github.com/ory/dockertest/v3"
	// "github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/suite"

	"github.com/classic-terra/core/v2/tests/e2e/chain"
	"github.com/classic-terra/core/v2/tests/e2e/configurer"
	// "github.com/classic-terra/core/v2/tests/e2e/util"
)

type IntegrationTestSuite struct {
	suite.Suite

	chainConfigs     []*configurer.ChainConfig
	containerManager *configurer.Manager
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up e2e integration test suite...")

	// The e2e test flow is as follows:
	//
	// 1. Configure two chains - chan A and chain B.
	//   * For each chain, set up two validators
	//   * Initialize configs and genesis for all validators.
	// 2. Start both networks.
	// 3. Run IBC relayer betweeen the two chains.
	// 4. Execute various e2e tests, including IBC.
	manager, err := configurer.NewManager()
	s.Require().NoError(err)
	s.containerManager = manager

	chainA := s.createChainConfig(chain.ChainAID)
	chainB := s.createChainConfig(chain.ChainBID)
	s.chainConfigs = []*configurer.ChainConfig{chainA, chainB}

	s.runValidators()
	// s.runIBCRelayer()
	fmt.Println("pass setup")
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down e2e integration test suite...")

	err := s.containerManager.ClearResources()
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) runValidators() {
	for _, chainConfig := range s.chainConfigs {
		s.T().Logf("starting Terra Classic %s validator containers...", chainConfig.ChainMeta.Id)
		for _, node := range chainConfig.NodeConfigs {
			node.Run()
		}
	}
}

// func (s *IntegrationTestSuite) runIBCRelayer() {
// 	s.T().Log("starting Hermes relayer container...")

// 	tmpDir, err := os.MkdirTemp("", "terra-e2e-testnet-hermes-")
// 	s.Require().NoError(err)
// 	s.tmpDirs = append(s.tmpDirs, tmpDir)

// 	terraAVal := s.chains[0].Nodes[0]
// 	terraBVal := s.chains[1].Nodes[0]
// 	hermesCfgPath := path.Join(tmpDir, "hermes")

// 	s.Require().NoError(os.MkdirAll(hermesCfgPath, 0o755))
// 	_, err = util.CopyFile(
// 		filepath.Join("./scripts/", "hermes_bootstrap.sh"),
// 		filepath.Join(hermesCfgPath, "hermes_bootstrap.sh"),
// 	)
// 	s.Require().NoError(err)

// 	s.hermesResource, err = s.dkrPool.RunWithOptions(
// 		&dockertest.RunOptions{
// 			Name:       fmt.Sprintf("%s-%s-relayer", s.chains[0].ChainMeta.Id, s.chains[1].ChainMeta.Id),
// 			Repository: "informalsystems/hermes",
// 			Tag:        "1.7.4",
// 			NetworkID:  s.dkrNet.Network.ID,
// 			Cmd: []string{
// 				"start",
// 			},
// 			User: "root:root",
// 			Mounts: []string{
// 				fmt.Sprintf("%s/:/root/hermes", hermesCfgPath),
// 			},
// 			ExposedPorts: []string{
// 				"3031",
// 			},
// 			PortBindings: map[docker.Port][]docker.PortBinding{
// 				"3031/tcp": {{HostIP: "", HostPort: "3031"}},
// 			},
// 			Env: []string{
// 				fmt.Sprintf("TERRA_A_E2E_CHAIN_ID=%s", s.chains[0].ChainMeta.Id),
// 				fmt.Sprintf("TERRA_B_E2E_CHAIN_ID=%s", s.chains[1].ChainMeta.Id),
// 				fmt.Sprintf("TERRA_A_E2E_VAL_MNEMONIC=%s", terraAVal.Mnemonic),
// 				fmt.Sprintf("TERRA_B_E2E_VAL_MNEMONIC=%s", terraBVal.Mnemonic),
// 				fmt.Sprintf("TERRA_A_E2E_VAL_HOST=%s", s.valResources[s.chains[0].ChainMeta.Id][0].Container.Name[1:]),
// 				fmt.Sprintf("TERRA_B_E2E_VAL_HOST=%s", s.valResources[s.chains[1].ChainMeta.Id][0].Container.Name[1:]),
// 			},
// 			Entrypoint: []string{
// 				"sh",
// 				"-c",
// 				"chmod +x /root/hermes/hermes_bootstrap.sh && /root/hermes/hermes_bootstrap.sh",
// 			},
// 		},
// 		noRestart,
// 	)
// 	s.Require().NoError(err)

// 	endpoint := fmt.Sprintf("http://%s/state", s.hermesResource.GetHostPort("3031/tcp"))
// 	s.Require().Eventually(
// 		func() bool {
// 			resp, err := http.Get(endpoint)
// 			if err != nil {
// 				return false
// 			}

// 			defer resp.Body.Close()

// 			bz, err := io.ReadAll(resp.Body)
// 			if err != nil {
// 				return false
// 			}

// 			var respBody map[string]interface{}
// 			if err := json.Unmarshal(bz, &respBody); err != nil {
// 				return false
// 			}

// 			status := respBody["status"].(string)
// 			result := respBody["result"].(map[string]interface{})

// 			return status == "success" && len(result["chains"].([]interface{})) == 2
// 		},
// 		5*time.Minute,
// 		time.Second,
// 		"hermes relayer not healthy",
// 	)

// 	s.T().Logf("started Hermes relayer container: %s", s.hermesResource.Container.ID)

// 	// XXX: Give time to both networks to start, otherwise we might see gRPC
// 	// transport errors.
// 	time.Sleep(10 * time.Second)

// 	// create the client, connection and channel between the two Osmosis chains
// 	s.connectIBCChains()
// }

// func (s *IntegrationTestSuite) connectIBCChains() {
// 	s.T().Logf("connecting %s and %s chains via IBC", s.chains[0].ChainMeta.Id, s.chains[1].ChainMeta.Id)

// 	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
// 	defer cancel()

// 	exec, err := s.dkrPool.Client.CreateExec(docker.CreateExecOptions{
// 		Context:      ctx,
// 		AttachStdout: true,
// 		AttachStderr: true,
// 		Container:    s.hermesResource.Container.ID,
// 		User:         "root",
// 		Cmd: []string{
// 			"hermes",
// 			"create",
// 			"channel",
// 			s.chains[0].ChainMeta.Id,
// 			s.chains[1].ChainMeta.Id,
// 			"--port-a=transfer",
// 			"--port-b=transfer",
// 		},
// 	})
// 	s.Require().NoError(err)

// 	var (
// 		outBuf bytes.Buffer
// 		errBuf bytes.Buffer
// 	)

// 	err = s.dkrPool.Client.StartExec(exec.ID, docker.StartExecOptions{
// 		Context:      ctx,
// 		Detach:       false,
// 		OutputStream: &outBuf,
// 		ErrorStream:  &errBuf,
// 	})
// 	s.Require().NoErrorf(
// 		err,
// 		"failed connect chains; stdout: %s, stderr: %s", outBuf.String(), errBuf.String(),
// 	)

// 	s.Require().Containsf(
// 		errBuf.String(),
// 		"successfully opened init channel",
// 		"failed to connect chains via IBC: %s", errBuf.String(),
// 	)

// 	s.T().Logf("connected %s and %s chains via IBC", s.chains[0].ChainMeta.Id, s.chains[1].ChainMeta.Id)
// }

func (s *IntegrationTestSuite) createChainConfig(chainId string) *configurer.ChainConfig {
	s.T().Logf("starting e2e infrastructure for chain-id: %s", chainId)
	tmpDir, err := os.MkdirTemp("", "terra-e2e-testnet-")

	s.T().Logf("temp directory for chain-id %v: %v", chainId, tmpDir)
	s.Require().NoError(err)

	initializedChain, err := chain.InitChain(chainId, tmpDir)
	s.Require().NoError(err)

	chainConfig, err := configurer.NewChainConfig(s.T(), initializedChain, s.containerManager)
	s.Require().NoError(err)

	return chainConfig
}
