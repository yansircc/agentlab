// Package strictjson decodes exactly one JSON value without unknown fields.
package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

func Decode(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("JSON has trailing input")
	}
	return nil
}
