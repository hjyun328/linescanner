package linescanner

import (
	"bytes"
	"errors"
	"io"
)

var (
	ErrBufferOverflow    = errors.New("buffer is overflow")
	ErrInvalidLineCount  = errors.New("line count is invalid")
	ErrInvalidChunkSize  = errors.New("chunk size is invalid")
	ErrInvalidBufferSize = errors.New("buffer size is invalid")
	ErrGreaterBufferSize = errors.New("buffer size must be greater than chunk size")
)

const (
	defaultChunkSize  = 4096
	defaultBufferSize = 1 << 20
)

type Scanner struct {
	reader io.ReaderAt

	chunk  []byte
	buffer []byte

	bufferLineStartPos int
	readerPos          int
	readerLineStartPos int

	backupBufferLineStartPos int
	backupReaderPos          int
	backupReaderLineStartPos int

	endOfFile bool
	endOfScan bool
}

func New(reader io.ReaderAt, position int) *Scanner {
	return NewWithSize(reader, position, defaultChunkSize, defaultBufferSize)
}

func NewWithSize(reader io.ReaderAt, position int, chunkSize int, bufferSize int) *Scanner {
	if chunkSize <= 0 {
		panic(ErrInvalidChunkSize)
	}
	if bufferSize <= 0 {
		panic(ErrInvalidBufferSize)
	}
	if chunkSize > bufferSize {
		panic(ErrGreaterBufferSize)
	}
	return &Scanner{
		reader:             reader,
		chunk:              make([]byte, chunkSize),
		buffer:             make([]byte, 0, bufferSize),
		readerPos:          position,
		readerLineStartPos: position,
	}
}

func (s *Scanner) backupPosition() {
	s.backupBufferLineStartPos = s.bufferLineStartPos
	s.backupReaderPos = s.readerPos
	s.backupBufferLineStartPos = s.readerLineStartPos
}

func (s *Scanner) recoverPosition() {
	s.bufferLineStartPos = s.backupBufferLineStartPos
	s.readerPos = s.backupReaderPos
	s.readerLineStartPos = s.backupBufferLineStartPos
}

func (s *Scanner) getLineSizeExcludingLF() int {
	lineSize := bytes.IndexByte(s.buffer[s.bufferLineStartPos:], '\n')
	if lineSize < 0 && s.endOfFile {
		s.endOfScan = true
		return len(s.buffer[s.bufferLineStartPos:])
	}
	return lineSize
}

func (s *Scanner) getLineExcludingCR(lineSize int) string {
	line := s.buffer[s.bufferLineStartPos : s.bufferLineStartPos+lineSize]
	if line[len(line)-1] == '\r' {
		return string(line[:len(line)-1])
	}
	return string(line)
}

func (s *Scanner) rearrangeBuffer(n int) error {
	lineSize := len(s.buffer[s.bufferLineStartPos:])
	if lineSize+n > cap(s.buffer) {
		return ErrBufferOverflow
	}
	if s.bufferLineStartPos+lineSize+n > cap(s.buffer) {
		copy(s.buffer, s.buffer[s.bufferLineStartPos:])
		s.buffer = s.buffer[:lineSize]
		s.bufferLineStartPos = 0
	}
	return nil
}

func (s *Scanner) read() error {
	n, err := s.reader.ReadAt(s.chunk, int64(s.readerPos))
	if err != nil && err != io.EOF {
		return err
	}
	if err == io.EOF {
		s.endOfFile = true
	}
	if n > 0 {
		if err := s.rearrangeBuffer(n); err != nil {
			return err
		}
		s.buffer = append(s.buffer, s.chunk[:n]...)
		s.readerPos += n
	}
	return nil
}

func (s *Scanner) Line(lineCount int) (lines []string, err error) {
	s.backupPosition()
	if lineCount <= 0 {
		return lines, ErrInvalidLineCount
	}
	if s.endOfScan {
		return lines, io.EOF
	}
	for {
		lineSize := s.getLineSizeExcludingLF()
		if lineSize < 0 {
			if err := s.read(); err != nil {
				s.recoverPosition()
				return nil, err
			}
			continue
		}
		if lineSize > 0 {
			lines = append(lines, s.getLineExcludingCR(lineSize))
			s.bufferLineStartPos += lineSize
			s.readerLineStartPos += lineSize
		}
		if s.endOfScan {
			return lines, io.EOF
		}
		s.bufferLineStartPos++ // skip line feed position
		s.readerLineStartPos++ // skip line feed position
		if len(lines) == lineCount {
			return lines, nil
		}
	}
}

func (s *Scanner) Position() int {
	return s.readerLineStartPos
}
