package ioscanner

import (
	"bytes"
	"errors"
	"io"
)

var (
	ErrBufferOverflow   = errors.New("buffer is overflow")
	ErrInvalidLineCount = errors.New("line count is invalid")
)

const (
	defaultChunkSize  = 4096
	defaultBufferSize = 1 << 20
)

type Scanner struct {
	reader             io.ReaderAt
	chunk              []byte
	buffer             []byte
	bufferLineStartPos int
	filePos            int
	fileLineStartPos   int
	eof                bool
	eob                bool
}

func New(reader io.ReaderAt, position int) *Scanner {
	return NewWithSize(reader, position, defaultChunkSize, defaultBufferSize)
}

func NewWithSize(reader io.ReaderAt, position int, chunkSize int, bufferSize int) *Scanner {
	return &Scanner{
		reader:             reader,
		chunk:              make([]byte, chunkSize),
		buffer:             make([]byte, 0, bufferSize),
		bufferLineStartPos: 0,
		filePos:            position,
		fileLineStartPos:   position,
		eof:                false,
		eob:                false,
	}
}

func (s *Scanner) getLineSizeExcludingLF() int {
	lineSize := bytes.IndexByte(s.buffer[s.bufferLineStartPos:], '\n')
	if lineSize < 0 && s.eof {
		s.eob = true
		lineSize = len(s.buffer[s.bufferLineStartPos:])
	}
	return lineSize
}

func (s *Scanner) getLineExcludingCR(lineSize int) string {
	line := s.buffer[s.bufferLineStartPos : s.bufferLineStartPos+lineSize]
	if len(line) > 0 && line[len(line)-1] == '\r' {
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
	n, err := s.reader.ReadAt(s.chunk, int64(s.filePos))
	if err != nil && err != io.EOF {
		return err
	}
	if err == io.EOF {
		s.eof = true
	}
	if n > 0 {
		s.filePos += n
		if err := s.rearrangeBuffer(n); err != nil {
			return err
		}
		s.buffer = append(s.buffer, s.chunk[:n]...)
	}
	return nil
}

func (s *Scanner) Line(lineCount int) (lines []string, err error) {
	if lineCount <= 0 {
		return lines, ErrInvalidLineCount
	}
	if s.eob {
		return lines, io.EOF
	}
	for {
		lineSize := s.getLineSizeExcludingLF()
		if lineSize < 0 {
			if err := s.read(); err != nil {
				return nil, err
			}
			continue
		}
		lines = append(lines, s.getLineExcludingCR(lineSize))
		s.bufferLineStartPos += lineSize + 1
		s.fileLineStartPos += lineSize + 1
		if s.eob {
			return lines, io.EOF
		}
		if len(lines) == lineCount {
			return lines, nil
		}
	}
}

func (s *Scanner) Position() int {
	return s.fileLineStartPos
}