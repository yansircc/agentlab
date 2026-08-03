package oracle

import (
	"encoding/json"
	"errors"
	"os"
	"sort"

	"github.com/yansircc/agentlab/internal/artifact"
)

const ReceiptContract = "agentlab.oracle-receipt.v1"

type Receipt struct {
	Contract      string       `json:"contract"`
	Kind          string       `json:"kind"`
	Engine        artifact.Ref `json:"engine"`
	Configuration artifact.Ref `json:"configuration"`
	Output        artifact.Ref `json:"output"`
	SideEffects   artifact.Ref `json:"side_effects"`
}

func Record(store artifact.Store, kind string, configuration, output any, sideEffects []string) (Receipt, error) {
	if kind == "" || len(sideEffects) == 0 {
		return Receipt{}, errors.New("oracle kind and explicit side effects are required")
	}
	declared := append([]string(nil), sideEffects...)
	sort.Strings(declared)
	for index, value := range declared {
		if value == "" || (index > 0 && value == declared[index-1]) {
			return Receipt{}, errors.New("side effect declarations must be nonempty and unique")
		}
	}
	enginePath, err := os.Executable()
	if err != nil {
		return Receipt{}, err
	}
	engineBytes, err := os.ReadFile(enginePath)
	if err != nil {
		return Receipt{}, err
	}
	engine, err := store.Put(engineBytes)
	if err != nil {
		return Receipt{}, err
	}
	configRef, err := putJSON(store, configuration)
	if err != nil {
		return Receipt{}, err
	}
	outputRef, err := putJSON(store, output)
	if err != nil {
		return Receipt{}, err
	}
	effectsRef, err := putJSON(store, declared)
	if err != nil {
		return Receipt{}, err
	}
	return Receipt{Contract: ReceiptContract, Kind: kind, Engine: engine, Configuration: configRef, Output: outputRef, SideEffects: effectsRef}, nil
}

func putJSON(store artifact.Store, value any) (artifact.Ref, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return artifact.Ref{}, err
	}
	return store.PutCanonicalJSON(data)
}
