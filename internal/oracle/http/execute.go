package httporacle

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/oracle"
)

func Execute(ctx context.Context, store artifact.Store, spec Spec) (Result, error) {
	if err := validateSpec(spec); err != nil {
		return Result{}, err
	}
	secrets, err := resolveSecrets(spec)
	if err != nil {
		return Result{}, err
	}
	body, config, err := requestBody(store, spec, secrets)
	if err != nil {
		return Result{}, err
	}
	requestContext, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, spec.Method, spec.URL, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	for name, value := range spec.PublicHeaders {
		request.Header.Set(name, value)
	}
	for name, value := range secrets.headers {
		request.Header.Set(name, value)
	}
	client := &http.Client{Timeout: spec.Timeout}
	if !spec.FollowRedirects {
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}
	response, err := client.Do(request)
	if err != nil {
		return Result{}, err
	}
	defer response.Body.Close()
	redactions := append([]string(nil), secrets.values...)
	raw, err := io.ReadAll(io.LimitReader(response.Body, int64(spec.MaxBodyBytes+longest(redactions)+1)))
	if err != nil {
		return Result{}, err
	}
	truncated := len(raw) > spec.MaxBodyBytes+longest(redactions)
	redacted := redact(raw, redactions)
	if len(redacted) > spec.MaxBodyBytes {
		redacted, truncated = redacted[:spec.MaxBodyBytes], true
	}
	bodyRef, err := store.Put(redacted)
	if err != nil {
		return Result{}, err
	}
	output := Output{
		StatusCode: response.StatusCode, FinalURL: string(redact([]byte(response.Request.URL.String()), redactions)),
		Headers: captureHeaders(response.Header, spec.CaptureResponseHeaders, redactions),
		Body:    bodyRef, Truncated: truncated,
	}
	receipt, err := oracle.Record(store, "http", config, output, spec.SideEffects)
	if err != nil {
		return Result{}, err
	}
	return Result{Receipt: receipt, Output: output}, nil
}

func validateSpec(spec Spec) error {
	parsed, err := url.Parse(spec.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("absolute HTTP URL is required")
	}
	if spec.Method == "" || spec.Timeout <= 0 || spec.MaxBodyBytes < 1 || spec.MaxBodyBytes > 8*1024*1024 || len(spec.SideEffects) == 0 {
		return errors.New("method, timeout, bounded body, and side effects are required")
	}
	if len(spec.PublicBody) != 0 && spec.SecretBodyHandle != "" {
		return errors.New("public and secret request body are mutually exclusive")
	}
	publicHeaders := map[string]bool{}
	for name := range spec.PublicHeaders {
		publicHeaders[strings.ToLower(http.CanonicalHeaderKey(name))] = true
	}
	for name := range spec.SecretHeaderHandles {
		if name == "" || publicHeaders[strings.ToLower(http.CanonicalHeaderKey(name))] {
			return errors.New("public and secret request headers overlap or are invalid")
		}
	}
	captured := map[string]bool{}
	for _, name := range spec.CaptureResponseHeaders {
		canonical := strings.ToLower(http.CanonicalHeaderKey(name))
		if name == "" || captured[canonical] {
			return errors.New("captured response headers must be nonempty and unique")
		}
		captured[canonical] = true
	}
	return nil
}

type secretValues struct {
	headers map[string]string
	body    string
	values  []string
}

func resolveSecrets(spec Spec) (secretValues, error) {
	result := secretValues{headers: map[string]string{}}
	for header, handle := range spec.SecretHeaderHandles {
		value, err := secret(handle)
		if header == "" || err != nil {
			return secretValues{}, err
		}
		result.headers[header] = value
		result.values = append(result.values, value)
	}
	if spec.SecretBodyHandle != "" {
		value, err := secret(spec.SecretBodyHandle)
		if err != nil {
			return secretValues{}, err
		}
		result.body = value
		result.values = append(result.values, value)
	}
	return result, nil
}

func secret(handle string) (string, error) {
	value, ok := os.LookupEnv(handle)
	if handle == "" || !ok {
		return "", errors.New("secret handle is unavailable: " + handle)
	}
	if len(value) > 64*1024 {
		return "", errors.New("secret exceeds redaction bound: " + handle)
	}
	return value, nil
}
