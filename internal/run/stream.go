package run

import (
	"bufio"
	"encoding/json"
	"io"
)

type streamItem struct {
	stream string
	line   []byte
	err    error
	eof    bool
}

func scanStream(name string, reader io.Reader, out chan<- streamItem) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		out <- streamItem{stream: name, line: append([]byte(nil), scanner.Bytes()...)}
	}
	out <- streamItem{stream: name, err: scanner.Err(), eof: true}
}

func classify(line []byte) string {
	var envelope struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(line, &envelope) == nil && envelope.Type == "result" {
		return "terminal_candidate"
	}
	return "public_output"
}
