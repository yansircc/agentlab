package httporacle

import (
	"time"

	"github.com/yansircc/agentlab/internal/artifact"
	"github.com/yansircc/agentlab/internal/oracle"
)

type Spec struct {
	Method                 string            `json:"method"`
	URL                    string            `json:"url"`
	Timeout                time.Duration     `json:"timeout"`
	MaxBodyBytes           int               `json:"max_body_bytes"`
	PublicHeaders          map[string]string `json:"public_headers,omitempty"`
	SecretHeaderHandles    map[string]string `json:"secret_header_handles,omitempty"`
	PublicBody             []byte            `json:"-"`
	SecretBodyHandle       string            `json:"secret_body_handle,omitempty"`
	CaptureResponseHeaders []string          `json:"capture_response_headers,omitempty"`
	FollowRedirects        bool              `json:"follow_redirects"`
	SideEffects            []string          `json:"side_effects"`
}

type Output struct {
	StatusCode int                 `json:"status_code"`
	FinalURL   string              `json:"final_url"`
	Headers    map[string][]string `json:"headers"`
	Body       artifact.Ref        `json:"body"`
	Truncated  bool                `json:"truncated"`
}

type Result struct {
	Receipt oracle.Receipt `json:"receipt"`
	Output  Output         `json:"output"`
}

type configuration struct {
	Method                 string            `json:"method"`
	URL                    string            `json:"url"`
	Timeout                time.Duration     `json:"timeout"`
	MaxBodyBytes           int               `json:"max_body_bytes"`
	PublicHeaders          map[string]string `json:"public_headers,omitempty"`
	SecretHeaderHandles    map[string]string `json:"secret_header_handles,omitempty"`
	PublicBody             *artifact.Ref     `json:"public_body,omitempty"`
	SecretBodyHandle       string            `json:"secret_body_handle,omitempty"`
	CaptureResponseHeaders []string          `json:"capture_response_headers,omitempty"`
	FollowRedirects        bool              `json:"follow_redirects"`
}
