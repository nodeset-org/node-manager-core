package eth

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Gets the logs for a particular log request, breaking the calls into batches if necessary
func GetLogs(ec IExecutionClient, addressFilter []common.Address, topicFilter [][]common.Hash, intervalSize, fromBlock, toBlock *big.Int, blockHash *common.Hash) ([]types.Log, error) {
	// Handle unlimited intervals with a single call
	if intervalSize == nil {
		logs, err := ec.FilterLogs(context.Background(), ethereum.FilterQuery{
			Addresses: addressFilter,
			Topics:    topicFilter,
			FromBlock: fromBlock,
			ToBlock:   toBlock,
			BlockHash: blockHash,
		})
		if err != nil {
			return nil, err
		}
		return logs, nil
	}

	// Get the latest block
	if toBlock == nil {
		latestBlock, err := ec.BlockNumber(context.Background())
		if err != nil {
			return nil, err
		}
		toBlock = big.NewInt(0)
		toBlock.SetUint64(latestBlock)
	}

	// Set the start and end, clamping on the latest block
	logs := []types.Log{}
	iterationSize := new(big.Int).Set(intervalSize)
	iterationSize.Sub(iterationSize, common.Big1)
	start := new(big.Int).Set(fromBlock)
	end := new(big.Int).Add(start, iterationSize)
	if end.Cmp(toBlock) == 1 {
		end.Set(toBlock)
	}
	for {
		// Get the logs using the current interval
		newLogs, err := ec.FilterLogs(context.Background(), ethereum.FilterQuery{
			Addresses: addressFilter,
			Topics:    topicFilter,
			FromBlock: start,
			ToBlock:   end,
			BlockHash: blockHash,
		})
		if err != nil {
			return nil, err
		}

		// Append the logs to the total list
		logs = append(logs, newLogs...)

		// Return once we've finished iterating
		if end.Cmp(toBlock) == 0 {
			return logs, nil
		}

		// Update to the next interval (end+1 : that + interval - 1)
		start.Add(end, common.Big1)
		end.Add(start, iterationSize)
		if end.Cmp(toBlock) == 1 {
			end.Set(toBlock)
		}
	}
}
