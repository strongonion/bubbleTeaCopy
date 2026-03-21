package config

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"bubblecopy/internal/model"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var requiredHeaders = []string{"source", "target", "op", "clear_target", "group"}
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// LoadCSV parses task rows from a CSV file with required headers:
// source,target,op,clear_target,group
func LoadCSV(path string) ([]model.Task, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}

	decoded, err := decodeCSVContent(raw)
	if err != nil {
		return nil, fmt.Errorf("decode csv: %w", err)
	}

	reader := csv.NewReader(bytes.NewReader(decoded))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	headerRow, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("csv is empty")
		}
		return nil, fmt.Errorf("read header: %w", err)
	}

	headerIndex := make(map[string]int, len(headerRow))
	for i, raw := range headerRow {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		headerIndex[name] = i
	}

	for _, header := range requiredHeaders {
		if _, ok := headerIndex[header]; !ok {
			return nil, fmt.Errorf("missing required header: %s", header)
		}
	}

	var tasks []model.Task
	rowNumber := 1
	for {
		rowNumber++
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read row %d: %w", rowNumber, err)
		}

		source := valueAt(record, headerIndex["source"])
		target := valueAt(record, headerIndex["target"])
		rawOp := strings.ToLower(valueAt(record, headerIndex["op"]))
		rawClear := strings.ToLower(valueAt(record, headerIndex["clear_target"]))
		group := valueAt(record, headerIndex["group"])

		if source == "" {
			return nil, fmt.Errorf("row %d: source is required", rowNumber)
		}
		if target == "" {
			return nil, fmt.Errorf("row %d: target is required", rowNumber)
		}

		var op model.Operation
		switch rawOp {
		case string(model.OpCopy):
			op = model.OpCopy
		case string(model.OpMove):
			op = model.OpMove
		default:
			return nil, fmt.Errorf("row %d: invalid op %q (expected copy or move)", rowNumber, rawOp)
		}

		clearTarget, err := parseStrictBool(rawClear)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid clear_target %q (expected true or false)", rowNumber, rawClear)
		}

		if group == "" {
			group = model.DefaultGroup
		}

		tasks = append(tasks, model.Task{
			Index:       len(tasks),
			Source:      source,
			Target:      target,
			Op:          op,
			ClearTarget: clearTarget,
			Group:       group,
			Status:      model.StatusPending,
		})
	}

	return tasks, nil
}

func decodeCSVContent(raw []byte) ([]byte, error) {
	raw = bytes.TrimPrefix(raw, utf8BOM)
	if utf8.Valid(raw) {
		return raw, nil
	}

	decoded, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), raw)
	if err == nil && utf8.Valid(decoded) {
		return decoded, nil
	}

	return nil, fmt.Errorf("unsupported csv encoding, please save as UTF-8 or GB18030")
}

func valueAt(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

func parseStrictBool(raw string) (bool, error) {
	switch raw {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool")
	}
}
