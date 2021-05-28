// Portions of this file from: https://github.com/jbreiding/rsync-go
// in turn based on the rsync/rdiff algorithm: http://www.samba.org/~tridge/phd_thesis.pdf

package rsync

import (
	"crypto/md5"
	"io"
)

const BlockSize = 1024 * 6

// Internal constant used in rolling checksum.
const _M = 1 << 16

// Signature hash item generated from target.
type BlockHash struct {
	Index      uint64
	StrongHash []byte
	WeakHash   uint32
}

// Calculate the signature of target.
func CreateSignature(target io.Reader) ([]BlockHash, error) {
	var err error
	var n int
	buffer := make([]byte, BlockSize)
	result := make([]BlockHash, 0)

	var block []byte
	loop := true
	var index uint64
	for loop {
		n, err = io.ReadAtLeast(target, buffer, BlockSize)
		if err != nil {
			// n == 0.
			if err == io.EOF {
				return result, nil
			}
			if err != io.ErrUnexpectedEOF {
				return nil, err
			}
			// n > 0.
			loop = false
		}
		block = buffer[:n]
		weak, _, _ := βhash(block)
		result = append(result, BlockHash{StrongHash: uniqueHash(block), WeakHash: weak, Index: index})
		index++
	}
	return result, nil
}

// Use a more unique way to identify a set of bytes.
func uniqueHash(v []byte) []byte {
	h := md5.New()
	h.Write(v)
	return h.Sum(nil)
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
