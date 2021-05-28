package rsync

import (
	"bytes"
	"testing"
)

func TestRSync_CreateSignature(t *testing.T) {
	inputString := `Mary had a little lamb,
Its fleece was white as snow;
And everywhere that Mary went
The lamb was sure to go.

It followed her to school one day,
Which was against the rule;
It made the children laugh and play
To see a lamb at school.<pad>\n`
	inputBytes := []byte(inputString)

	input := bytes.NewReader(inputBytes)

	rs := &RSync{
		BlockSize:    10,
		UniqueHasher: nil,
		buffer:       nil,
	}

	sig := make([]BlockHash, 0, 10)
	writeSignature := func(bl BlockHash) error {
		sig = append(sig, bl)
		return nil
	}

	rs.CreateSignature(input, writeSignature)

	if len(sig) != len(inputBytes) / 10 {
		t.Errorf("Signature contains unexpected number of blocks")
	}
}
