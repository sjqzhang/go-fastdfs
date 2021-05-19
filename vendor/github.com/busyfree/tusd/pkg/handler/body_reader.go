package handler

import (
	"io"
	"sync/atomic"
)

// bodyReader is an io.Reader, which is intended to wrap the request
// body reader. If an error occurr during reading the request body, it
// will not return this error to the reading entity, but instead store
// the error and close the io.Reader, so that the error can be checked
// afterwards. This is helpful, so that the stores do not have to handle
// the error but this can instead be done in the handler.
// In addition, the bodyReader keeps track of how many bytes were read.
type bodyReader struct {
	reader       io.Reader
	err          error
	bytesCounter int64
}

func newBodyReader(r io.Reader) *bodyReader {
	return &bodyReader{
		reader: r,
	}
}

func (r *bodyReader) Read(b []byte) (int, error) {
	if r.err != nil {
		return 0, io.EOF
	}

	n, err := r.reader.Read(b)
	atomic.AddInt64(&r.bytesCounter, int64(n))
	r.err = err

	if err == io.EOF {
		return n, io.EOF
	} else {
		return n, nil
	}
}

func (r bodyReader) hasError() error {
	if r.err == io.EOF {
		return nil
	}

	return r.err
}

func (r *bodyReader) bytesRead() int64 {
	return atomic.LoadInt64(&r.bytesCounter)
}
