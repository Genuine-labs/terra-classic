#!/bin/bash
set -o errexit -o nounset -o pipefail -o xtrace
shopt -s expand_aliases

alias chainA="terrad --node http://localhost:26657 --chain-id localterra-a"
alias chainB="terrad --node http://localhost:36657 --chain-id localterra-b"

# setup the keys
echo "bottom loan skill merry east cradle onion journey palm apology verb edit desert impose absurd oil bubble sweet glove shallow size build burst effort" | terrad --keyring-backend test keys add validator --recover || echo "key exists"
echo "increase bread alpha rigid glide amused approve oblige print asset idea enact lawn proof unfold jeans rabbit audit return chuckle valve rather cactus great" | terrad --keyring-backend test  keys add faucet --recover || echo "key exists"

VALIDATOR=$(terrad keys show validator --keyring-backend test -a)

args="--keyring-backend test --gas 200000 --gas-prices 0.1uluna --gas-adjustment 1.3 --broadcast-mode block --yes"
TX_FLAGS=($args)

# send money to the validator on both chains
chainA tx bank send faucet "$VALIDATOR" 1000000000uluna "${TX_FLAGS[@]}"
chainB tx bank send faucet "$VALIDATOR" 1000000000uluna "${TX_FLAGS[@]}"


args="--keyring-backend test --gas 2000000 --gas-prices 0.1uluna --gas-adjustment 1.3 --broadcast-mode block --yes"
TX_FLAGS=($args)
# store and instantiate the contract
chainA tx wasm store ./tests/ibc-hooks/bytecode/counter.wasm --from validator  "${TX_FLAGS[@]}"
CONTRACT_ID=$(chainA query wasm list-code -o json | jq -r '.code_infos[-1].code_id')
chainA tx wasm instantiate "$CONTRACT_ID" '{"count": "0"}' --from validator --no-admin --label=counter "${TX_FLAGS[@]}"

# get the contract address
export CONTRACT_ADDRESS=$(chainA query wasm list-contract-by-code "$CONTRACT_ID" -o json | jq -r '.contracts | [last][0]')

denom=$(chainA query bank balances "$CONTRACT_ADDRESS" -o json | jq -r '.balances[0].denom')
balance=$(chainA query bank balances "$CONTRACT_ADDRESS" -o json | jq -r '.balances[0].amount')

# send ibc transaction to execite the contract
MEMO='{"wasm":{"contract":"'"$CONTRACT_ADDRESS"'","msg": {"increment": {}} }}'
chainB tx ibc-transfer transfer transfer channel-0 $CONTRACT_ADDRESS 10uluna \
       --from validator --keyring-backend test -y  \
       --memo "$MEMO"

# wait for the ibc round trip
sleep 16

new_balance=$(chainA query bank balances "$CONTRACT_ADDRESS" -o json | jq -r '.balances[0].amount')
export ADDR_IN_CHAIN_A=$(chainA q ibchooks wasm-sender channel-0 "$VALIDATOR")
QUERY='{"get_total_funds": {}}'
funds=$(chainA query wasm contract-state smart "$CONTRACT_ADDRESS" "$QUERY" -o json | jq -c -r '.data')
# QUERY='{"get_count": {"addr": "'"$ADDR_IN_CHAIN_A"'"}}'
QUERY='{"get_count": {}}'
count=$(chainA query wasm contract-state smart "$CONTRACT_ADDRESS" "$QUERY" -o json |  jq -r '.data')

echo "funds: $funds, count: $count"
echo "denom: $denom, old balance: $balance, new balance: $new_balance"
