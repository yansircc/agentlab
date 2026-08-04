package run

import (
	"errors"
	"path/filepath"
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/transaction"
)

// RecordHostOracleEvidence admits one immutable Host-produced objective oracle
// artifact while a run is active. It is deliberately not a provider tool or
// adapter event: Host runtime code owns this fact, and comparison/meta-audit
// validate the artifact's domain contract before relying on it.
func (o *Operation) RecordHostOracleEvidence(raw artifact.Ref) (EvidenceRef, error) {
	if !raw.Valid() {
		return EvidenceRef{}, errors.New("host oracle evidence reference is invalid")
	}
	if _, err := o.artifacts.Read(raw); err != nil {
		return EvidenceRef{}, err
	}
	lease, err := transaction.Acquire(filepath.Join(o.dir, "producer.lock"))
	if err != nil {
		return EvidenceRef{}, err
	}
	defer lease.Release()
	state, err := o.currentState()
	if err != nil || state.started == nil || state.exit != nil || state.stopRequested || state.hostOracleRecorded {
		return EvidenceRef{}, errors.New("host oracle evidence is not admissible")
	}
	record, err := o.appendEvent(time.Now().UTC(), eventHostOracle, hostOracleEvidence{Raw: raw})
	if err != nil {
		return EvidenceRef{}, err
	}
	return EvidenceRef{ExperimentID: o.experimentID, RunID: o.runID, Sequence: record.Sequence}, nil
}
