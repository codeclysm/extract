package extract

import (
	"errors"
	"io"
)

func copyCancel(dst io.Writer, src io.Reader, cancel <-chan bool) (int64, error) {
	return io.Copy(dst, newCancelableReader(src, cancel))
}

type cancelableReader struct {
	cancel <-chan bool
	src    io.Reader
}

func (r *cancelableReader) Read(p []byte) (int, error) {
	select {
	case <-r.cancel:
		return 0, errors.New("interrupted")
	default:
		return r.src.Read(p)
	}
}

func newCancelableReader(src io.Reader, cancel <-chan bool) *cancelableReader {
	return &cancelableReader{
		cancel: cancel,
		src:    src,
	}
}
