package rsync

import (
	"bytes"
	"strings"
	"testing"
)

func TestCreateSignature(t *testing.T) {
	inputString := strings.Repeat("0123456789ABCDEF", 24*BlockSize/16)
	input := bytes.NewReader([]byte(inputString))

	sig, err := CreateSignature(input)
	if err != nil {
		t.Fatal(err)
	}

	if len(sig) != 24 {
		t.Errorf("Signature contains unexpected number of blocks %v instead of %v", len(sig), 24)
	}
}

func TestCreateDelta(t *testing.T) {
	inputString := strings.Repeat("0123456789ABCDEF", 24*BlockSize/16)
	input := bytes.NewReader([]byte(inputString))
	sig, err := CreateSignature(input)
	if err != nil {
		t.Fatal(err)
	}

	opsOut := make([]Operation, 0)
	writeOp := func(op Operation) error {
		opsOut = append(opsOut, op)
		return nil
	}

	CreateDelta(bytes.NewReader([]byte(inputString)), sig, writeOp)
	if len(opsOut) != 24 {
		t.Errorf("Delta contains unexpected number of operations %v instead of %v", len(sig), 24)
	}
}
