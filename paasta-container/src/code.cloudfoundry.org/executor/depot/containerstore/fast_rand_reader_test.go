package containerstore_test

import "math/rand"

// a fast random reader used in the test suite
type fastRandReader struct{}

func (r fastRandReader) Read(p []byte) (n int, err error) {
	return rand.Read(p)
}
