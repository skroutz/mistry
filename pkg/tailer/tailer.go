// Package tailer emulates the features of the tail program (reading from
// continuously updated files).
package tailer

import (
	"io"
	"os"
	"time"
)

// A Tailer holds an io.ReadCloser interface. It implements a Read() function
// which emulates the tailf UNIX program.
type Tailer struct {
	io.ReadCloser
}

// New returns a new Tailer for the given path.
func New(path string) (*Tailer, error) {
	f, err := os.Open(path)
	if err != nil {
		return &Tailer{}, err
	}

	if _, err := f.Seek(0, 2); err != nil {
		return &Tailer{}, err
	}
	return &Tailer{f}, nil
}

// Read provides a tailf like generator by handling the io.EOF error.
// It returns the number of bytes read and any error encountered.
// At end of file, when no more input is available, Read handles the io.EOF
// error by continuing the reading loop.
func (t *Tailer) Read(b []byte) (int, error) {
	for {
		n, err := t.ReadCloser.Read(b)
		if n > 0 {
			return n, nil
		} else if err != io.EOF {
			return n, err
		}
		time.Sleep(500 * time.Millisecond)
	}
}
