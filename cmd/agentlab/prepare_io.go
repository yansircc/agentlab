package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
)

type prepareRequestFlags struct {
	root    string
	request string
}

func parsePrepareRequest(name string, args []string) (prepareRequestFlags, error) {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	root := set.String("root", defaultRoot(), "storage root")
	request := set.String("request", "-", "JSON request path or - for stdin")
	if err := set.Parse(args); err != nil {
		return prepareRequestFlags{}, err
	}
	return prepareRequestFlags{root: *root, request: *request}, nil
}

func readRequest(path string, target any) error {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request has trailing input")
	}
	return nil
}

func readRequiredFile(path, label string) ([]byte, error) {
	if path == "" || path == "-" {
		return nil, errors.New(label + " file path is required")
	}
	return os.ReadFile(path)
}
