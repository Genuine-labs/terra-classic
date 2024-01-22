package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/classic-terra/core/v2/tests/e2e/chain"
)

func main() {
	var (
		dataDir               string
		chainId               string
		votingPeriod          time.Duration
	)

	flag.StringVar(&dataDir, "data-dir", "", "chain data directory")
	flag.StringVar(&chainId, "chain-id", "", "chain ID")
	flag.DurationVar(&votingPeriod, "voting-period", 30000000000, "voting period")
	flag.Parse()

	if len(dataDir) == 0 {
		panic("data-dir is required")
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		panic(err)
	}

	createdChain, err := chain.InitChain(chainId, dataDir, votingPeriod)
	if err != nil {
		panic(err)
	}

	b, _ := json.Marshal(createdChain)
	fileName := fmt.Sprintf("%v/%v-encode", dataDir, chainId)
	if err = os.WriteFile(fileName, b, 0777); err != nil {
		panic(err)
	}
}
