package ledger

import "fmt"

func (l *Ledger) Read(after uint64, limit int) ([]Record, error) {
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("limit must be between 1 and 1000")
	}
	result := make([]Record, 0, limit)
	err := scan(l.path, func(record Record) error {
		if record.Sequence > after && len(result) < limit {
			result = append(result, record)
		}
		return nil
	})
	return result, err
}

func (l *Ledger) Replay() ([]Record, error) {
	var records []Record
	err := scan(l.path, func(record Record) error {
		records = append(records, record)
		return nil
	})
	return records, err
}

func (l *Ledger) Visit(visit func(Record) error) error {
	return scan(l.path, visit)
}
