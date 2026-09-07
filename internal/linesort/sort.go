// Package linesort sorts and deduplicates text records using bounded buffers
// and temporary files. The number of records is limited only by available disk.
package linesort

import (
	"bufio"
	"container/heap"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	defaultChunkBytes = 4 << 20
	defaultChunkLines = 65536
	defaultFanIn      = 32
	ioBufferBytes     = 64 << 10
)

// Sorter owns its temporary directory. Close must be called, including on error.
// Memory depends on the chunk budget and the longest record, not total output.
type Sorter struct {
	dir                           string
	chunk                         []string
	bytes, chunkBytes, chunkLines int
	fanIn                         int
	runs                          uint64
	finished, closed              bool
}

// New creates a sorter whose temporary files are stored under parent.
func New(parent string) (*Sorter, error) {
	return newSorter(parent, defaultChunkBytes, defaultChunkLines, defaultFanIn)
}

func newSorter(parent string, chunkBytes, chunkLines, fanIn int) (*Sorter, error) {
	if chunkBytes < 1 || chunkLines < 1 || fanIn < 2 {
		return nil, errors.New("invalid sort buffer settings")
	}
	parent, err := filepath.Abs(parent)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(parent, "sort-*")
	if err != nil {
		return nil, err
	}
	return &Sorter{dir: dir, chunkBytes: chunkBytes, chunkLines: chunkLines, fanIn: fanIn}, nil
}

// Add accepts one record without a newline. Records larger than the chunk budget
// are written as single-record runs instead of rejecting the job.
func (s *Sorter) Add(ctx context.Context, line string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.finished || s.closed {
		return errors.New("sorter is already finalized")
	}
	if strings.ContainsRune(line, '\n') {
		return errors.New("sort record contains a newline")
	}
	// Include the string descriptor in the budget, and independently bound the
	// number of descriptors for very short records.
	cost := len(line) + 16
	if len(s.chunk) > 0 && (cost > s.chunkBytes-s.bytes || len(s.chunk) >= s.chunkLines) {
		if err := s.spill(ctx); err != nil {
			return err
		}
	}
	s.chunk = append(s.chunk, line)
	s.bytes += cost
	if s.bytes >= s.chunkBytes || len(s.chunk) >= s.chunkLines {
		return s.spill(ctx)
	}
	return nil
}

func (s *Sorter) path(pass, run uint64) string {
	return filepath.Join(s.dir, fmt.Sprintf("p%d-r%020d", pass, run))
}

func (s *Sorter) spill(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(s.chunk) == 0 {
		return nil
	}
	slices.Sort(s.chunk)
	err := writeRun(s.path(0, s.runs), func(w io.Writer) error {
		for i, line := range s.chunk {
			if err := ctx.Err(); err != nil {
				return err
			}
			if i == 0 || line != s.chunk[i-1] {
				if err := writeLine(w, line); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("write sort run: %w", err)
	}
	s.runs++
	clear(s.chunk)
	s.chunk = s.chunk[:0]
	s.bytes = 0
	return nil
}

// WriteTo emits unique records in lexical order. It may only be called once.
// Run names are derived from counters, so even the run inventory stays on disk.
// Each merge opens at most fanIn input files, regardless of the number of runs.
func (s *Sorter) WriteTo(ctx context.Context, w io.Writer) (count uint64, err error) {
	if s.finished || s.closed {
		return 0, errors.New("sorter is already finalized")
	}
	s.finished = true
	if err := s.spill(ctx); err != nil {
		return 0, err
	}
	var pass uint64
	runs := s.runs
	for runs > 1 {
		var next uint64
		for first := uint64(0); first < runs; {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
			size := min(uint64(s.fanIn), runs-first)
			paths := make([]string, 0, size)
			for i := uint64(0); i < size; i++ {
				paths = append(paths, s.path(pass, first+i))
			}
			if err := writeRun(s.path(pass+1, next), func(out io.Writer) error {
				return merge(ctx, paths, out)
			}); err != nil {
				return 0, fmt.Errorf("merge sort runs: %w", err)
			}
			for _, name := range paths {
				if err := os.Remove(name); err != nil {
					return 0, fmt.Errorf("remove merged run: %w", err)
				}
			}
			first += size
			next++
		}
		pass++
		runs = next
	}
	if runs == 0 {
		return 0, ctx.Err()
	}
	f, err := os.Open(s.path(pass, 0))
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	r := bufio.NewReaderSize(f, ioBufferBytes)
	for {
		if err := ctx.Err(); err != nil {
			return count, err
		}
		line, err := readLine(r)
		if errors.Is(err, io.EOF) {
			return count, nil
		}
		if err != nil {
			return count, err
		}
		if err := writeLine(w, line); err != nil {
			return count, err
		}
		count++
	}
}

// Close removes only the private directory created by New.
func (s *Sorter) Close() error {
	s.closed = true
	s.chunk = nil
	return os.RemoveAll(s.dir)
}

func writeRun(name string, produce func(io.Writer) error) (err error) {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	w := bufio.NewWriterSize(f, ioBufferBytes)
	if err := produce(w); err != nil {
		return err
	}
	return w.Flush()
}

func writeLine(w io.Writer, line string) error {
	for _, text := range []string{line, "\n"} {
		n, err := io.WriteString(w, text)
		if err != nil {
			return err
		}
		if n != len(text) {
			return io.ErrShortWrite
		}
	}
	return nil
}

// Run files always end records with LF. A partial final record is corruption,
// not normal EOF. Reader avoids imposing a second line limit on generated names.
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if errors.Is(err, io.EOF) && line != "" {
		return "", io.ErrUnexpectedEOF
	}
	if err != nil {
		return "", err
	}
	return line[:len(line)-1], nil
}

type head struct {
	line string
	r    *bufio.Reader
}

type heads []*head

func (h heads) Len() int           { return len(h) }
func (h heads) Less(i, j int) bool { return h[i].line < h[j].line }
func (h heads) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *heads) Push(x any)        { *h = append(*h, x.(*head)) }
func (h *heads) Pop() any {
	n := len(*h) - 1
	v := (*h)[n]
	(*h)[n] = nil
	*h = (*h)[:n]
	return v
}

func merge(ctx context.Context, paths []string, out io.Writer) (err error) {
	var files []*os.File
	defer func() {
		for _, f := range files {
			err = errors.Join(err, f.Close())
		}
	}()
	var queue heads
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		files = append(files, f)
		r := bufio.NewReaderSize(f, ioBufferBytes)
		line, err := readLine(r)
		if errors.Is(err, io.EOF) {
			continue
		}
		if err != nil {
			return err
		}
		queue = append(queue, &head{line: line, r: r})
	}
	heap.Init(&queue)
	var previous string
	havePrevious := false
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry := heap.Pop(&queue).(*head)
		if !havePrevious || entry.line != previous {
			if err := writeLine(out, entry.line); err != nil {
				return err
			}
			previous, havePrevious = entry.line, true
		}
		line, err := readLine(entry.r)
		if errors.Is(err, io.EOF) {
			continue
		}
		if err != nil {
			return err
		}
		entry.line = line
		heap.Push(&queue, entry)
	}
	return ctx.Err()
}
