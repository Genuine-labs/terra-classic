package configurer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

type Manager struct {
	pool           *dockertest.Pool
	network        *dockertest.Network
	resources      map[string]*dockertest.Resource
	resourcesMutex sync.RWMutex
}

type TxResponse struct {
	Code      int      `yaml:"code" json:"code"`
	Codespace string   `yaml:"codespace" json:"codespace"`
	Data      string   `yaml:"data" json:"data"`
	GasUsed   string   `yaml:"gas_used" json:"gas_used"`
	GasWanted string   `yaml:"gas_wanted" json:"gas_wanted"`
	Height    string   `yaml:"height" json:"height"`
	Info      string   `yaml:"info" json:"info"`
	Logs      []string `yaml:"logs" json:"logs"`
	Timestamp string   `yaml:"timestamp" json:"timestamp"`
	Tx        string   `yaml:"tx" json:"tx"`
	TxHash    string   `yaml:"txhash" json:"txhash"`
	RawLog    string   `yaml:"raw_log" json:"raw_log"`
	Events    []string `yaml:"events" json:"events"`
}

// NewManager creates a new Manager instance and initializes
// all Docker specific utilities. Returns an error if initialisation fails.
func NewManager() (docker *Manager, err error) {
	docker = &Manager{
		resources: make(map[string]*dockertest.Resource),
	}
	docker.pool, err = dockertest.NewPool("")
	if err != nil {
		return nil, err
	}
	docker.network, err = docker.pool.CreateNetwork("terra-classic-testnet")
	if err != nil {
		return nil, err
	}
	return docker, nil
}

// RunNodeResource runs a node container. Assigns containerName to the container.
// Mounts the container on valConfigDir volume on the running host. Returns the container resource and error if any.
func (m *Manager) RunNodeResource(chainId, containerName, valConfigDir string) (*dockertest.Resource, error) {
	runOpts := &dockertest.RunOptions{
		Name:       containerName,
		Repository: "core",
		Tag:        "debug",
		NetworkID:  m.network.Network.ID,
		User:       "root:root",
		Cmd:        []string{"start"},
		Mounts: []string{
			fmt.Sprintf("%s/:/core/.terrad", valConfigDir),
		},
	}

	resource, err := m.pool.RunWithOptions(runOpts, noRestart)
	if err != nil {
		return nil, err
	}

	m.resourcesMutex.Lock()
	m.resources[containerName] = resource
	m.resourcesMutex.Unlock()

	return resource, nil
}

// RemoveNodeResource removes a node container specified by containerName.
// Returns error if any.
func (m *Manager) RemoveNodeResource(containerName string) error {
	resource, err := m.GetNodeResource(containerName)
	if err != nil {
		return err
	}
	var opts docker.RemoveContainerOptions
	opts.ID = resource.Container.ID
	opts.Force = true
	if err := m.pool.Client.RemoveContainer(opts); err != nil {
		return err
	}
	delete(m.resources, containerName)
	return nil
}

// ClearResources removes all outstanding Docker resources created by the Manager.
func (m *Manager) ClearResources() error {
	for _, resource := range m.resources {
		if err := m.pool.Purge(resource); err != nil {
			return err
		}
	}

	if err := m.pool.RemoveNetwork(m.network); err != nil {
		return err
	}
	return nil
}

// GetNodeResource returns the node resource for containerName.
func (m *Manager) GetNodeResource(containerName string) (*dockertest.Resource, error) {
	resource, exists := m.resources[containerName]
	if !exists {
		return nil, fmt.Errorf("node resource not found: container name: %s", containerName)
	}
	return resource, nil
}

func noRestart(config *docker.HostConfig) {
	// in this case we don't want the nodes to restart on failure
	config.RestartPolicy = docker.RestartPolicy{
		Name: "no",
	}
}

// ExecCmd executes command by running it on the node container (specified by containerName)
// success is the output of the command that needs to be observed for the command to be deemed successful.
// It is found by checking if stdout or stderr contains the success string anywhere within it.
// returns container std out, container std err, and error if any.
// An error is returned if the command fails to execute or if the success string is not found in the output.
func (m *Manager) ExecCmd(t *testing.T, containerName string, command []string, success string, checkTxHash, returnTxHashInfoAsJSON bool) (bytes.Buffer, bytes.Buffer, error) {
	t.Helper()
	if _, ok := m.resources[containerName]; !ok {
		return bytes.Buffer{}, bytes.Buffer{}, fmt.Errorf("no resource %s found", containerName)
	}
	containerId := m.resources[containerName].Container.ID

	var (
		exec   *docker.Exec
		outBuf bytes.Buffer
		errBuf bytes.Buffer
		err    error
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	t.Logf("\n\nRunning: \"%s\", success condition is \"%s\"", command, success)
	maxDebugLogTriesLeft := 3

	expectedSequence := 0
	var sequenceMismatchRegex = regexp.MustCompile(`account sequence mismatch, expected (\d+),`)

	// We use the `require.Eventually` function because it is only allowed to do one transaction per block without
	// sequence numbers. For simplicity, we avoid keeping track of the sequence number and just use the `require.Eventually`.
	require.Eventually(
		t,
		func() bool {
			outBuf.Reset()
			errBuf.Reset()

			exec, err = m.pool.Client.CreateExec(docker.CreateExecOptions{
				Context:      ctx,
				AttachStdout: true,
				AttachStderr: true,
				Container:    containerId,
				User:         "root",
				Cmd:          command,
			})
			require.NoError(t, err)

			err = m.pool.Client.StartExec(exec.ID, docker.StartExecOptions{
				Context:      ctx,
				Detach:       false,
				OutputStream: &outBuf,
				ErrorStream:  &errBuf,
			})
			if err != nil {
				return false
			}

			// Sometimes a node hangs and doesn't vote in time, as long as it passes that is all we care about
			if strings.Contains(outBuf.String(), "inactive proposal") || strings.Contains(errBuf.String(), "inactive proposal") {
				return true
			}

			errBufString := errBuf.String()
			// When a validator attempts to send multiple transactions in the same block, the expected sequence number
			// will be thrown off, causing the transaction to fail. It will eventually clear, but what the following code
			// does is it takes the expected sequence number from the error message, adds a sequence number flag with that
			// number, and retries the transaction. This allows for multiple txs from the same validator to be committed in the same block.
			if (errBufString != "" || outBuf.String() != "") && containerName != "hermes-relayer" {
				// Check if the error message matches the expected pattern
				errBufMatches := sequenceMismatchRegex.FindAllStringSubmatch(errBufString, -1)
				outBufMatches := sequenceMismatchRegex.FindAllStringSubmatch(outBuf.String(), -1)
				if len(errBufMatches) > 0 {
					lastArg := command[len(command)-1]
					if strings.Contains(lastArg, "--sequence") {
						// Remove the last argument from the command
						command = command[:len(command)-1]
					}
					expectedSequenceStr := errBufMatches[len(errBufMatches)-1][1]
					expectedSequence, _ = strconv.Atoi(expectedSequenceStr)
					modifiedCommand := append(command, fmt.Sprintf("--sequence=%d", expectedSequence))
					// Update the command for the next iteration
					command = modifiedCommand
				} else if len(outBufMatches) > 0 {
					lastArg := command[len(command)-1]
					if strings.Contains(lastArg, "--sequence") {
						// Remove the last argument from the command
						command = command[:len(command)-1]
					}
					expectedSequenceStr := outBufMatches[len(outBufMatches)-1][1]
					expectedSequence, _ = strconv.Atoi(expectedSequenceStr)
					modifiedCommand := append(command, fmt.Sprintf("--sequence=%d", expectedSequence))
					// Update the command for the next iteration
					command = modifiedCommand
				}
			}

			// Note that this does not match all errors.
			// This only works if CLI outpurs "Error" or "error"
			// to stderr.
			if maxDebugLogTriesLeft > 0 &&
				!strings.Contains(errBufString, "not found") {
				t.Log("\nstderr:")
				t.Log(errBufString)

				t.Log("\nstdout:")
				t.Log(outBuf.String())
				// N.B: We should not be returning false here
				// because some applications such as Hermes might log
				// "error" to stderr when they function correctly,
				// causing test flakiness. This log is needed only for
				// debugging purposes.
				maxDebugLogTriesLeft--
			}

			if success != "" && !checkTxHash {
				return strings.Contains(outBuf.String(), success) || strings.Contains(errBufString, success)
			}

			if success != "" && checkTxHash {
				// Now that sdk got rid of block.. we need to query the txhash to get the result
				outStr := outBuf.String()

				txResponse, err := parseTxResponse(outStr)
				if err != nil {
					return false
				}

				// Don't even attempt to query the tx hash if the initial response code is not 0
				if txResponse.Code != 0 {
					return false
				}

				// This method attempts to query the txhash until the block is committed, at which point it returns an error here,
				// causing the tx to be submitted again.
				outBuf, errBuf, err = m.ExecQueryTxHash(t, containerName, txResponse.TxHash, returnTxHashInfoAsJSON)
				if err != nil {
					return false
				}
			}

			return true
		},
		time.Minute,
		10*time.Millisecond,
		fmt.Sprintf("success condition (%s) command %s was not met.\nstdout:\n %s\nstderr:\n %s\n \nerror: %v\n",
			success, command, outBuf.String(), errBuf.String(), err),
	)

	return outBuf, errBuf, nil
}

func (m *Manager) ExecQueryTxHash(t *testing.T, containerName, txHash string, returnAsJson bool) (bytes.Buffer, bytes.Buffer, error) {
	t.Helper()
	if _, ok := m.resources[containerName]; !ok {
		return bytes.Buffer{}, bytes.Buffer{}, fmt.Errorf("no resource %s found", containerName)
	}
	containerId := m.resources[containerName].Container.ID

	var (
		exec   *docker.Exec
		outBuf bytes.Buffer
		errBuf bytes.Buffer
		err    error
	)

	var command []string
	if returnAsJson {
		command = []string{"terrad", "query", "tx", txHash, "-o=json"}
	} else {
		command = []string{"terrad", "query", "tx", txHash}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	t.Logf("\n\nRunning: \"%s\", success condition is \"code: 0\"", txHash)
	maxDebugLogTriesLeft := 3

	successConditionMet := false
	startTime := time.Now()
	for time.Since(startTime) < time.Second*5 {
		outBuf.Reset()
		errBuf.Reset()

		exec, err = m.pool.Client.CreateExec(docker.CreateExecOptions{
			Context:      ctx,
			AttachStdout: true,
			AttachStderr: true,
			Container:    containerId,
			User:         "root",
			Cmd:          command,
		})
		if err != nil {
			return outBuf, errBuf, err
		}

		err = m.pool.Client.StartExec(exec.ID, docker.StartExecOptions{
			Context:      ctx,
			Detach:       false,
			OutputStream: &outBuf,
			ErrorStream:  &errBuf,
		})
		if err != nil {
			return outBuf, errBuf, err
		}

		errBufString := errBuf.String()

		if maxDebugLogTriesLeft > 0 &&
			!strings.Contains(errBufString, "not found") {
			t.Log("\nstderr:")
			t.Log(errBufString)

			t.Log("\nstdout:")
			t.Log(outBuf.String())
			maxDebugLogTriesLeft--
		}

		successConditionMet = strings.Contains(outBuf.String(), "code: 0") || strings.Contains(errBufString, "code: 0") || strings.Contains(outBuf.String(), "code\":0") || strings.Contains(errBufString, "code\":0")
		if successConditionMet {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if !successConditionMet {
		return outBuf, errBuf, fmt.Errorf("success condition for txhash %s \"code: 0\" command %s was not met.\nstdout:\n %s\nstderr:\n %s\n \nerror: %v\n",
			txHash, command, outBuf.String(), errBuf.String(), err)
	}

	return outBuf, errBuf, nil
}

func parseTxResponse(outStr string) (txResponse TxResponse, err error) {
	if strings.Contains(outStr, "{\"height\":\"") {
		startIdx := strings.Index(outStr, "{\"height\":\"")
		if startIdx == -1 {
			return txResponse, fmt.Errorf("Start of JSON data not found")
		}
		// Trim the string to start from the identified index
		outStrTrimmed := outStr[startIdx:]
		// JSON format
		err = json.Unmarshal([]byte(outStrTrimmed), &txResponse)
		if err != nil {
			return txResponse, fmt.Errorf("JSON Unmarshal error: %v", err)
		}
	} else {
		// Find the start of the YAML data
		startIdx := strings.Index(outStr, "code: ")
		if startIdx == -1 {
			return txResponse, fmt.Errorf("Start of YAML data not found")
		}
		// Trim the string to start from the identified index
		outStrTrimmed := outStr[startIdx:]
		err = yaml.Unmarshal([]byte(outStrTrimmed), &txResponse)
		if err != nil {
			return txResponse, fmt.Errorf("YAML Unmarshal error: %v", err)
		}
	}
	return txResponse, err
}
