package pi

import (
	"errors"
	"os"
	"path/filepath"
)

// VerifyRuntimeIdentity extends static SDK discovery with the adjacent
// bundled executable required by the context-filter extension at runtime.
func VerifyRuntimeIdentity(config IdentityConfig) (AdapterIdentity, error) {
	identity, err := DiscoverIdentity(config)
	if err != nil {
		return AdapterIdentity{}, err
	}
	binary := filepath.Join(filepath.Dir(config.ContextFilterPath), "bin", "agentlab")
	data, err := os.ReadFile(binary)
	if err != nil || len(data) == 0 || len(data) > 64<<20 || sha256Digest(data) != identity.AdapterDigest {
		return AdapterIdentity{}, errors.New("Pi bundled runtime identity differs")
	}
	return identity, nil
}
