// Portions of this file from: https://github.com/jbreiding/rsync-go
// in turn based on the rsync/rdiff algorithm: http://www.samba.org/~tridge/phd_thesis.pdf

package rsync

import (
	"crypto/md5"
	"hash"
	"io"
)

// If no BlockSize is specified in the RSync instance, this value is used.
const DefaultBlockSize = 1024 * 6

// Internal constant used in rolling checksum.
const _M = 1 << 16

// Signature hash item generated from target.
type BlockHash struct {
	Index      uint64
	StrongHash []byte
	WeakHash   uint32
}

// Write signatures as they are generated.
type SignatureWriter func(bl BlockHash) error

// Properties to use while working with the rsync algorithm.
// A single RSync should not be used concurrently as it may contain
// internal buffers and hash sums.
type RSync struct {
	BlockSize int

	// If this is nil an MD5 hash is used.
	UniqueHasher hash.Hash

	buffer []byte
}

// Calculate the signature of target.
func (r *RSync) CreateSignature(target io.Reader, sw SignatureWriter) error {
	if r.BlockSize <= 0 {
		r.BlockSize = DefaultBlockSize
	}
	if r.UniqueHasher == nil {
		r.UniqueHasher = md5.New()
	}
	var err error
	var n int

	minBufferSize := r.BlockSize
	if len(r.buffer) < minBufferSize {
		r.buffer = make([]byte, minBufferSize)
	}
	buffer := r.buffer

	var block []byte
	loop := true
	var index uint64
	for loop {
		n, err = io.ReadAtLeast(target, buffer, r.BlockSize)
		if err != nil {
			// n == 0.
			if err == io.EOF {
				return nil
			}
			if err != io.ErrUnexpectedEOF {
				return err
			}
			// n > 0.
			loop = false
		}
		block = buffer[:n]
		weak, _, _ := βhash(block)
		err = sw(BlockHash{StrongHash: r.uniqueHash(block), WeakHash: weak, Index: index})
		if err != nil {
			return err
		}
		index++
	}
	return nil
}

// Use a more unique way to identify a set of bytes.
func (r *RSync) uniqueHash(v []byte) []byte {
	r.UniqueHasher.Reset()
	r.UniqueHasher.Write(v)
	return r.UniqueHasher.Sum(nil)
}

// Use a faster way to identify a set of bytes.
func βhash(block []byte) (β uint32, β1 uint32, β2 uint32) {
	var a, b uint32
	for i, val := range block {
		a += uint32(val)
		b += (uint32(len(block)-1) - uint32(i) + 1) * uint32(val)
	}
	β = (a % _M) + (_M * (b % _M))
	β1 = a % _M
	β2 = b % _M
	return
}
