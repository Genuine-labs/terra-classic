package chain

import (
	"fmt"
)

func InitChain(id, dataDir string) (*Chain, error) {
	chain, err := new(id, dataDir)
	if err != nil {
		return nil, err
	}
	if err := chain.createAndInitValidators(3); err != nil {
		return nil, err
	}

	if err := initGenesis(chain); err != nil {
		return nil, err
	}
	var peers []string
	for _, peer := range chain.Nodes {
		peerID := fmt.Sprintf("%s@%s:26656", peer.getNodeKey().ID(), peer.Moniker)
		peer.PeerId = peerID
		peers = append(peers, peerID)
	}

	for _, node := range chain.Nodes {
		if node.IsValidator {
			if err := node.initNodeConfigs(peers); err != nil {
				return nil, err
			}
		}
	}
	return chain, nil
}
