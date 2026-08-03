package command

type boundedBuffer struct {
	data      []byte
	limit     int
	truncated bool
}

func newBoundedBuffer(limit int) *boundedBuffer {
	return &boundedBuffer{data: make([]byte, 0, min(limit, 4096)), limit: limit}
}

func (b *boundedBuffer) Write(input []byte) (int, error) {
	remaining := b.limit - len(b.data)
	if remaining > 0 {
		keep := min(remaining, len(input))
		b.data = append(b.data, input[:keep]...)
	}
	if len(input) > remaining {
		b.truncated = true
	}
	return len(input), nil
}
