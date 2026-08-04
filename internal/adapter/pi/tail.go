package pi

import (
	"bufio"
	"errors"
	"io"
)

func readTail(source io.Reader, cursor Cursor, sink Sink) (Cursor, error) {
	reader := bufio.NewReaderSize(source, maxLineBytes+1)
	if cursor.DiscardUntilNewline {
		next, complete, err := discardPartial(reader, cursor)
		if err != nil || !complete {
			return cursor, err
		}
		if err := sink.Commit(next, Batch{}); err != nil {
			return cursor, err
		}
		cursor = next
	}
	for {
		line, err := reader.ReadSlice('\n')
		switch {
		case err == nil:
			batch, translateErr := translateWithSession(cursor.SessionID, line[:len(line)-1])
			if translateErr != nil {
				return cursor, translateErr
			}
			next := cursor
			next.Offset += int64(len(line))
			if err := sink.Commit(next, batch); err != nil {
				return cursor, err
			}
			cursor = next
		case errors.Is(err, bufio.ErrBufferFull):
			return cursor, ErrLineTooLarge
		case errors.Is(err, io.EOF):
			return cursor, nil
		default:
			return cursor, err
		}
	}
}

func discardPartial(reader *bufio.Reader, cursor Cursor) (Cursor, bool, error) {
	consumed := int64(0)
	for {
		chunk, err := reader.ReadSlice('\n')
		consumed += int64(len(chunk))
		switch {
		case err == nil:
			next := cursor
			next.Offset += consumed
			next.DiscardUntilNewline = false
			return next, true, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			return cursor, false, nil
		default:
			return cursor, false, err
		}
	}
}
