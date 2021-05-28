package rsync

import (
	"bytes"
	"strings"
	"testing"
)

func TestCreateSignature(t *testing.T) {
	inputString := strings.Repeat("0123456789ABCDEF", 24 * BlockSize / 16)
	input := bytes.NewReader([]byte(inputString))

	sig, err := CreateSignature(input)
	if err != nil {
		t.Fatal(err)
	}

	if len(sig) != 24 {
		t.Errorf("Signature contains unexpected number of blocks %v instead of %v", len(sig), 24)
	}
}
