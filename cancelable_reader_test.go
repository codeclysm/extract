package extract

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCancelableReader(t *testing.T) {
	var b [100000]byte
	ctx, cancel := context.WithCancel(context.Background())
	reader := newCancelableReader(ctx, bytes.NewReader(b[:]))
	defer cancel()

	var buff [1000]byte
	readed := 0
	for {
		n, err := reader.Read(buff[:])
		if err != nil {
			fmt.Println("exit error:", err)
			require.Equal(t, "EOF", err.Error())
			break
		}
		require.NotZero(t, n)
		time.Sleep(10 * time.Millisecond)
		readed += n
	}

	fmt.Println("Readed", readed, "out of", len(b))
	require.Equal(t, len(b), readed)
}

func TestCancelableReaderWithInterruption(t *testing.T) {
	var b [100000]byte
	ctx, cancel := context.WithCancel(context.Background())
	reader := newCancelableReader(ctx, bytes.NewReader(b[:]))
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	var buff [1000]byte
	readed := 0
	for {
		n, err := reader.Read(buff[:])
		if err != nil {
			fmt.Println("exit error:", err)
			require.Equal(t, "interrupted", err.Error())
			break
		}
		require.NotZero(t, n)
		time.Sleep(10 * time.Millisecond)
		readed += n
	}
	fmt.Println("Readed", readed, "out of", len(b))
	require.True(t, readed < len(b))
}
