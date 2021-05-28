package streams

import (
	"bytes"
	"testing"
)

func TestCompress(t *testing.T) {
	inputBytes := []byte(`Mary had a little lamb,
Its fleece was white as snow;
And everywhere that Mary went
The lamb was sure to go.

It followed her to school one day,
Which was against the rule;
It made the children laugh and play
To see a lamb at school.`)

	input := bytes.NewReader(inputBytes)
	compressed := new(bytes.Buffer)

	err := Compress(input, compressed)
	if err != nil {
		t.Fatal(err)
	}

	compressedBytes := compressed.Bytes()
	if len(compressedBytes) >= len(inputBytes) {
		t.Errorf("compression did not compress (%v bytes >= %v bytes)", len(compressedBytes), len(inputBytes))
	}

	decompressedWriter := new(bytes.Buffer)
	compressedReader := bytes.NewReader(compressedBytes)
	err = Decompress(compressedReader, decompressedWriter)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(inputBytes, decompressedWriter.Bytes()) {
		t.Fatal("Decompressing did not yield expected result")
	}
}


func TestIsRecompressible(t *testing.T) {
	inputBytes := []byte(`Mary had a little lamb,
Its fleece was white as snow;
And everywhere that Mary went
The lamb was sure to go.

It followed her to school one day,
Which was against the rule;
It made the children laugh and play
To see a lamb at school.`)

	input := bytes.NewReader(inputBytes)
	compressed := new(bytes.Buffer)

	err := Compress(input, compressed)
	if err != nil {
		t.Fatal(err)
	}

	compressedBytes := compressed.Bytes()
	compressedReader := bytes.NewReader(compressedBytes)

	correct, err := IsRecompressible(compressedReader)
	if err != nil {
		t.Fatal(err)
	}
	if !correct {
		t.Fatal("Expected archive to be decompressible")
	}
}
