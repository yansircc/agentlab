package main

import (
	"context"
	"time"

	httporacle "github.com/yansircc/agentlab/internal/oracle/http"
)

type httpOracleRequest struct {
	Method                 string            `json:"method"`
	URL                    string            `json:"url"`
	Timeout                string            `json:"timeout"`
	MaxBodyBytes           int               `json:"max_body_bytes"`
	PublicHeaders          map[string]string `json:"public_headers,omitempty"`
	SecretHeaderHandles    map[string]string `json:"secret_header_handles,omitempty"`
	PublicBodyPath         string            `json:"public_body_path,omitempty"`
	SecretBodyHandle       string            `json:"secret_body_handle,omitempty"`
	CaptureResponseHeaders []string          `json:"capture_response_headers,omitempty"`
	FollowRedirects        bool              `json:"follow_redirects"`
	SideEffects            []string          `json:"side_effects"`
}

func runHTTPOracle(args []string) (any, error) {
	flags, err := parseOracleFlags("oracle http", args)
	if err != nil {
		return nil, err
	}
	var request httpOracleRequest
	if err := readRequest(flags.request, &request); err != nil {
		return nil, err
	}
	timeout, err := time.ParseDuration(request.Timeout)
	if err != nil {
		return nil, err
	}
	var body []byte
	if request.PublicBodyPath != "" {
		body, err = readRequiredFile(request.PublicBodyPath, "public body")
		if err != nil {
			return nil, err
		}
	}
	return httporacle.Execute(context.Background(), flags.store, httporacle.Spec{
		Method: request.Method, URL: request.URL, Timeout: timeout, MaxBodyBytes: request.MaxBodyBytes,
		PublicHeaders: request.PublicHeaders, SecretHeaderHandles: request.SecretHeaderHandles,
		PublicBody: body, SecretBodyHandle: request.SecretBodyHandle,
		CaptureResponseHeaders: request.CaptureResponseHeaders, FollowRedirects: request.FollowRedirects,
		SideEffects: request.SideEffects,
	})
}
