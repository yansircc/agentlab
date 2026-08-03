package httporacle

import (
	"bytes"
	"net/http"
	"sort"
	"strings"

	"github.com/yansircc/agentlab/internal/artifact"
)

func requestBody(store artifact.Store, spec Spec, secrets secretValues) ([]byte, configuration, error) {
	config := configuration{
		Method: spec.Method, URL: spec.URL, Timeout: spec.Timeout, MaxBodyBytes: spec.MaxBodyBytes,
		PublicHeaders: spec.PublicHeaders, SecretHeaderHandles: spec.SecretHeaderHandles,
		SecretBodyHandle: spec.SecretBodyHandle, CaptureResponseHeaders: sorted(spec.CaptureResponseHeaders),
		FollowRedirects: spec.FollowRedirects,
	}
	if spec.SecretBodyHandle != "" {
		return []byte(secrets.body), config, nil
	}
	if len(spec.PublicBody) != 0 {
		ref, err := store.Put(spec.PublicBody)
		if err != nil {
			return nil, configuration{}, err
		}
		config.PublicBody = &ref
	}
	return spec.PublicBody, config, nil
}

func captureHeaders(header http.Header, names, secrets []string) map[string][]string {
	result := map[string][]string{}
	for _, name := range sorted(names) {
		canonical := http.CanonicalHeaderKey(name)
		for _, value := range header.Values(canonical) {
			result[canonical] = append(result[canonical], string(redact([]byte(value), secrets)))
		}
	}
	return result
}

func redact(input []byte, secrets []string) []byte {
	result := append([]byte(nil), input...)
	ordered := append([]string(nil), secrets...)
	sort.Slice(ordered, func(left, right int) bool {
		if len(ordered[left]) != len(ordered[right]) {
			return len(ordered[left]) > len(ordered[right])
		}
		return ordered[left] < ordered[right]
	})
	for _, value := range ordered {
		if value != "" {
			result = bytes.ReplaceAll(result, []byte(value), []byte("[REDACTED]"))
		}
	}
	return result
}

func longest(values []string) int {
	result := 0
	for _, value := range values {
		result = max(result, len(value))
	}
	return result
}

func sorted(values []string) []string {
	result := append([]string(nil), values...)
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i]) < strings.ToLower(result[j]) })
	return result
}
