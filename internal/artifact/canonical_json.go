package artifact

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"strings"
)

func (s Store) PutCanonicalJSON(input []byte) (Ref, error) {
	canonical, err := CanonicalJSON(input)
	if err != nil {
		return Ref{}, err
	}
	return s.Put(canonical)
}

func CanonicalJSON(input []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	value, err := decodeValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("canonical JSON has trailing input")
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(output.Bytes(), []byte{'\n'}), nil
}

func decodeValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			return decodeObject(decoder)
		case '[':
			return decodeArray(decoder)
		default:
			return nil, fmt.Errorf("unexpected delimiter %q", value)
		}
	case json.Number:
		normalized, err := normalizeNumber(value.String())
		if err != nil {
			return nil, err
		}
		return json.Number(normalized), nil
	case string, bool, nil:
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported JSON token %T", token)
	}
}

var numberPattern = regexp.MustCompile(`^(-?)(0|[1-9][0-9]*)(?:\.([0-9]+))?(?:[eE]([+-]?[0-9]+))?$`)

func normalizeNumber(input string) (string, error) {
	parts := numberPattern.FindStringSubmatch(input)
	if parts == nil {
		return "", fmt.Errorf("invalid canonical number %q", input)
	}
	digits := strings.TrimLeft(parts[2]+parts[3], "0")
	if digits == "" {
		return "0", nil
	}
	exponent := new(big.Int)
	if parts[4] != "" {
		if _, ok := exponent.SetString(parts[4], 10); !ok {
			return "", fmt.Errorf("invalid canonical exponent %q", parts[4])
		}
	}
	exponent.Sub(exponent, big.NewInt(int64(len(parts[3]))))
	trimmed := strings.TrimRight(digits, "0")
	exponent.Add(exponent, big.NewInt(int64(len(digits)-len(trimmed))))
	sign := parts[1]
	if integer, ok := canonicalInt64(sign, trimmed, exponent); ok {
		return integer, nil
	}
	return sign + trimmed + "e" + exponent.String(), nil
}

func canonicalInt64(sign, digits string, exponent *big.Int) (string, bool) {
	if exponent.Sign() < 0 || !exponent.IsInt64() {
		return "", false
	}
	zeroCount := exponent.Int64()
	if int64(len(digits))+zeroCount > 19 {
		return "", false
	}
	value := sign + digits + strings.Repeat("0", int(zeroCount))
	integer := new(big.Int)
	if _, ok := integer.SetString(value, 10); !ok || !integer.IsInt64() {
		return "", false
	}
	return value, true
}

func decodeObject(decoder *json.Decoder) (map[string]any, error) {
	result := map[string]any{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, errors.New("JSON object key is not a string")
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate JSON key %q", key)
		}
		value, err := decodeValue(decoder)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, errors.New("unterminated JSON object")
	}
	return result, nil
}

func decodeArray(decoder *json.Decoder) ([]any, error) {
	var result []any
	for decoder.More() {
		value, err := decodeValue(decoder)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
		return nil, errors.New("unterminated JSON array")
	}
	return result, nil
}
