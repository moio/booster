// Portions of this file from: https://github.com/jbreiding/rsync-go
// in turn based on the rsync/rdiff algorithm: http://www.samba.org/~tridge/phd_thesis.pdf

package rsync

import (
	"bytes"
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

// Operation Types.
type OpType byte

const (
	OpBlock OpType = iota
	OpData
	OpHash
	OpBlockRange
)

const MaxDataOp = BlockSize * 10

// Operation is an instruction to mutate a target to align to a source
type Operation struct {
	Type          OpType
	BlockIndex    uint64
	BlockIndexEnd uint64
	Data          []byte
}

// OperationWriter writes operations
type OperationWriter func(op Operation) error

// Create the operation list to mutate the target signature into the source.
// Any data operation from the OperationWriter must have the data copied out
// within the span of the function; the data buffer underlying the operation
// data is reused. The sourceSum create a complete hash sum of the source if
// present.
func CreateDelta(source io.Reader, signature []BlockHash, ops OperationWriter) (err error) {
	buffer := make([]byte, BlockSize*2+MaxDataOp)

	// A single β hashes may correlate with a many unique hashes.
	hashLookup := make(map[uint32][]BlockHash, len(signature))
	for _, h := range signature {
		key := h.WeakHash
		hashLookup[key] = append(hashLookup[key], h)
	}

	type section struct {
		tail int
		head int
	}

	var data, sum section
	var n, validTo int
	var αPop, αPush, β, β1, β2 uint32
	var blockIndex uint64
	var rolling, lastRun, foundHash bool

	// Store the previous non-data operation for combining.
	var prevOp *Operation

	// Send the last operation if there is one waiting.
	defer func() {
		if prevOp == nil {
			return
		}
		err = ops(*prevOp)
		prevOp = nil
	}()

	// Combine OpBlock into OpBlockRange. To do this store the previous
	// non-data operation and determine if it can be extended.
	enqueue := func(op Operation) (err error) {
		switch op.Type {
		case OpBlock:
			if prevOp != nil {
				switch prevOp.Type {
				case OpBlock:
					if prevOp.BlockIndex+1 == op.BlockIndex {
						prevOp = &Operation{
							Type:          OpBlockRange,
							BlockIndex:    prevOp.BlockIndex,
							BlockIndexEnd: op.BlockIndex,
						}
						return
					}
				case OpBlockRange:
					if prevOp.BlockIndexEnd+1 == op.BlockIndex {
						prevOp.BlockIndexEnd = op.BlockIndex
						return
					}
				}
				err = ops(*prevOp)
				if err != nil {
					return
				}
				prevOp = nil
			}
			prevOp = &op
		case OpData:
			// Never save a data operation, as it would corrupt the buffer.
			if prevOp != nil {
				err = ops(*prevOp)
				if err != nil {
					return
				}
			}
			err = ops(op)
			if err != nil {
				return
			}
			prevOp = nil
		}
		return
	}

	for !lastRun {
		// Determine if the buffer should be extended.
		if sum.tail+BlockSize > validTo {
			// Determine if the buffer should be wrapped.
			if validTo+BlockSize > len(buffer) {
				// Before wrapping the buffer, send any trailing data off.
				if data.tail < data.head {
					err = enqueue(Operation{Type: OpData, Data: buffer[data.tail:data.head]})
					if err != nil {
						return err
					}
				}
				// Wrap the buffer.
				l := validTo - sum.tail
				copy(buffer[:l], buffer[sum.tail:validTo])

				// Reset indexes.
				validTo = l
				sum.tail = 0
				data.head = 0
				data.tail = 0
			}

			n, err = io.ReadAtLeast(source, buffer[validTo:validTo+BlockSize], BlockSize)
			validTo += n
			if err != nil {
				if err != io.EOF && err != io.ErrUnexpectedEOF {
					return err
				}
				lastRun = true

				data.head = validTo
			}
			if n == 0 {
				break
			}
		}

		// Set the hash sum window head. Must either be a block size
		// or be at the end of the buffer.
		sum.head = min(sum.tail+BlockSize, validTo)

		// Compute the rolling hash.
		if !rolling {
			β, β1, β2 = βhash(buffer[sum.tail:sum.head])
			rolling = true
		} else {
			αPush = uint32(buffer[sum.head-1])
			β1 = (β1 - αPop + αPush) % _M
			β2 = (β2 - uint32(sum.head-sum.tail)*αPop + β1) % _M
			β = β1 + _M*β2
		}

		// Determine if there is a hash match.
		foundHash = false
		if hh, ok := hashLookup[β]; ok && !lastRun {
			blockIndex, foundHash = findUniqueHash(hh, uniqueHash(buffer[sum.tail:sum.head]))
		}
		// Send data off if there is data available and a hash is found (so the buffer before it
		// must be flushed first), or the data chunk size has reached it's maximum size (for buffer
		// allocation purposes) or to flush the end of the data.
		if data.tail < data.head && (foundHash || data.head-data.tail >= MaxDataOp || lastRun) {
			err = enqueue(Operation{Type: OpData, Data: buffer[data.tail:data.head]})
			if err != nil {
				return err
			}
			data.tail = data.head
		}

		if foundHash {
			err = enqueue(Operation{Type: OpBlock, BlockIndex: blockIndex})
			if err != nil {
				return err
			}
			rolling = false
			sum.tail += BlockSize

			// There is prior knowledge that any available data
			// buffered will have already been sent. Thus we can
			// assume data.head and data.tail are the same.
			// May trigger "data wrap".
			data.head = sum.tail
			data.tail = sum.tail
		} else {
			// The following is for the next loop iteration, so don't try to calculate if last.
			if !lastRun && rolling {
				αPop = uint32(buffer[sum.tail])
			}
			sum.tail += 1

			// May trigger "data wrap".
			data.head = sum.tail
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Searches for a given strong hash among all strong hashes in this bucket.
func findUniqueHash(hh []BlockHash, hashValue []byte) (uint64, bool) {
	if len(hashValue) == 0 {
		return 0, false
	}
	for _, block := range hh {
		if bytes.Equal(block.StrongHash, hashValue) {
			return block.Index, true
		}
	}
	return 0, false
}
