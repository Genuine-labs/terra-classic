package chain

import "fmt"

type ChainMeta struct {
	DataDir string `json:"dataDir"`
	Id      string `json:"id"`
}

func (c *ChainMeta) configDir() string {
	return fmt.Sprintf("%s/%s", c.DataDir, c.Id)
}
