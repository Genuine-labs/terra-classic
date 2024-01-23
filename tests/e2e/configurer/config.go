package configurer

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/types/bech32"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"

	"github.com/classic-terra/core/v2/tests/e2e/chain"
)

type ChainConfig struct {
	chain.ChainMeta

	NodeConfigs      []*NodeConfig
	NodeTempConfigs  []*NodeConfig
	containerManager *Manager

	t *testing.T
}

func NewChainConfig(t *testing.T, chain *chain.Chain, manager *Manager) (config *ChainConfig, err error) {
	var nodeConfigs []*NodeConfig
	for _, node := range chain.Nodes {
		addr, err := node.KeyInfo.GetAddress()
		if err != nil {
			return nil, err
		}
		bech32Addr, err := bech32.ConvertAndEncode("terra", addr)
		if err != nil {
			return nil, err
		}
		nodeConfigs = append(nodeConfigs, &NodeConfig{
			Node:             node,
			OperatorAddress:  bech32Addr,
			ChainId:          chain.ChainMeta.Id,
			containerManager: manager,
			t:                t,
		})
	}

	return &ChainConfig{
		ChainMeta:        chain.ChainMeta,
		NodeConfigs:      nodeConfigs,
		NodeTempConfigs:  nodeConfigs,
		containerManager: manager,
		t:                t,
	}, nil

}

type NodeConfig struct {
	*chain.Node

	OperatorAddress  string
	ChainId          string
	rpcClient        *rpchttp.HTTP
	containerManager *Manager

	t *testing.T
}

// Run runs a node container for the given nodeIndex.
// The node configuration must be already added to the chain config prior to calling this
// method.
func (n *NodeConfig) Run() error {
	maxRetries := 3
	currentRetry := 0

	fmt.Println("run node: ", n)
	fmt.Println("node: ", n.Node)

	for currentRetry < maxRetries {
		n.t.Logf("starting node container: %s", n.Moniker)
		resource, err := n.containerManager.RunNodeResource(n.ChainId, n.Moniker, n.ConfigDir())
		if err != nil {
			return err
		}

		hostPort := resource.GetHostPort("26657/tcp")
		rpcClient, err := rpchttp.New("tcp://"+hostPort, "/websocket")
		if err != nil {
			return err
		}

		n.rpcClient = rpcClient

		success := false
		timeout := time.After(time.Second * 10)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				n.t.Logf("Terra node failed to produce blocks")
				// break out of the for loop, not just the select statement
				goto Retry
			case <-ticker.C:
				_, err := n.rpcClient.Status(context.Background())
				if err != nil {
					return err
				}
				if err == nil {
					n.t.Logf("started node container: %s", n.Moniker)
					success = true
					break
				}
			}

			if success {
				break
			}
		}

		if success {
			break
		}

	Retry:
		n.t.Logf("failed to start node container, retrying... (%d/%d)", currentRetry+1, maxRetries)
		// Do not remove the node resource on the last retry
		if currentRetry < maxRetries-1 {
			err := n.containerManager.RemoveNodeResource(n.Moniker)
			if err != nil {
				return err
			}
		}
		currentRetry++
	}

	if currentRetry >= maxRetries {
		return fmt.Errorf("failed to start node container after %d retries", maxRetries)
	}

	cmd := []string{"terrad", "debug", "addr", n.PrivateKey.PubKey().String()}
	n.t.Logf("extracting validator operator addresses for validator: %s", n.Moniker)
	_, errBuf, err := n.containerManager.ExecCmd(n.t, n.Moniker, cmd, "", false, false)
	if err != nil {
		return err
	}
	re := regexp.MustCompile("terravaloper(.{39})")
	operAddr := fmt.Sprintf("%s\n", re.FindString(errBuf.String()))
	n.OperatorAddress = strings.TrimSuffix(operAddr, "\n")

	return nil
}
