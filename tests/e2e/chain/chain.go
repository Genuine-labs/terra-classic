package chain

import "fmt"

const (
	keyringPassphrase = "testpassphrase"
	keyringAppName    = "testnet"
)

type Chain struct {
	ChainMeta ChainMeta `json:"chainMeta"`
	Nodes     []*Node	`json:"nodes"`
}

func new(id, dataDir string) (*Chain, error) {
	chainMeta := ChainMeta{
		Id:      id,
		DataDir: dataDir,
	}
	return &Chain{
		ChainMeta: chainMeta,
	}, nil
}

func (c *Chain) createAndInitValidators(count int) error {
	for i := 0; i < count; i++ {
		node := c.createValidator(i)

		// generate genesis files
		if err := node.init(); err != nil {
			return err
		}

		c.Nodes = append(c.Nodes, node)

		// create keys
		if err := node.createKey("val"); err != nil {
			return err
		}
		if err := node.createNodeKey(); err != nil {
			return err
		}
		if err := node.createConsensusKey(); err != nil {
			return err
		}
	}

	return nil
}

func (c *Chain) createAndInitValidatorsWithMnemonics(count int, mnemonics []string) error {
	for i := 0; i < count; i++ {
		// create node
		node := c.createValidator(i)

		// generate genesis files
		if err := node.init(); err != nil {
			return err
		}

		c.Nodes = append(c.Nodes, node)

		// create keys
		if err := node.createKeyFromMnemonic("val", mnemonics[i]); err != nil {
			return err
		}
		if err := node.createNodeKey(); err != nil {
			return err
		}
		if err := node.createConsensusKey(); err != nil {
			return err
		}
	}

	return nil
}

func (c *Chain) createValidator(index int) *Node {
	return &Node{
		Chain:       c,
		Moniker:     fmt.Sprintf("%s-node-%d", c.ChainMeta.Id, index),
		IsValidator: true,
	}
}
