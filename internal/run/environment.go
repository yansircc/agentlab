package run

import (
	"bytes"
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"
)

var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type resolvedEnvironment struct {
	values  []string
	secrets []string
}

func resolveEnvironment(public, handles map[string]string) (resolvedEnvironment, error) {
	if len(public)+len(handles) > 100 {
		return resolvedEnvironment{}, errors.New("worker environment exceeds bound")
	}
	values := make(map[string]string, len(public)+len(handles))
	seen := make(map[string]bool, len(public)+len(handles))
	for name, value := range public {
		if !environmentName.MatchString(name) || len(value) > 4096 || strings.ContainsRune(value, 0) {
			return resolvedEnvironment{}, errors.New("public worker environment is invalid")
		}
		values[name] = value
		seen[name] = true
	}
	secrets := make([]string, 0, len(handles))
	for name, handle := range handles {
		if !environmentName.MatchString(name) || !environmentName.MatchString(handle) || seen[name] {
			return resolvedEnvironment{}, errors.New("secret worker environment mapping is invalid or overlaps public environment")
		}
		value, ok := os.LookupEnv(handle)
		if !ok || value == "" || len(value) > 4096 || strings.ContainsAny(value, "\x00\r\n") {
			return resolvedEnvironment{}, errors.New("secret worker environment handle is unavailable or invalid: " + handle)
		}
		values[name] = value
		seen[name] = true
		secrets = append(secrets, value)
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	environment := make([]string, 0, len(names))
	for _, name := range names {
		environment = append(environment, name+"="+values[name])
	}
	sort.Slice(secrets, func(left, right int) bool { return len(secrets[left]) > len(secrets[right]) })
	return resolvedEnvironment{values: environment, secrets: secrets}, nil
}

func redactSecrets(data []byte, secrets []string) []byte {
	result := append([]byte(nil), data...)
	for _, secret := range secrets {
		result = bytes.ReplaceAll(result, []byte(secret), []byte("[REDACTED_SECRET]"))
	}
	return result
}
