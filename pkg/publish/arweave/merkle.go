package arweave

import (
	"crypto/sha256"
	"encoding/binary"
)

// Arweave chunking constants. See:
// https://github.com/ArweaveTeam/arweave/blob/master/apps/arweave/src/ar_tx.erl
// https://github.com/ArweaveTeam/arweave-js/blob/master/src/common/lib/merkle.ts
const (
	MaxChunkSize = 256 * 1024
	MinChunkSize = MaxChunkSize / 4
	NoteSize     = 32 // big-endian byte-range encoding length
)

// Chunk is a single piece of an Arweave transaction's data with the
// merkle metadata needed to upload it via /chunk.
type Chunk struct {
	DataHash     []byte // sha256(chunk-bytes)
	Data         []byte // raw chunk bytes
	MinByteRange int
	MaxByteRange int
}

// node is a merkle tree node (leaf or branch).
type node struct {
	id           []byte
	leftChild    *node
	rightChild   *node
	dataHash     []byte // leaf-only
	minByteRange int    // leaf-only
	maxByteRange int    // covers entire subtree
	byteRange    int    // branch-only: split point = leftChild.maxByteRange
	isLeaf       bool
}

// chunkData splits data into Arweave-compliant chunks, applying the
// rebalance rule that prevents a final chunk smaller than MinChunkSize
// (when the remainder permits).
func chunkData(data []byte) []Chunk {
	if len(data) == 0 {
		// Arweave still requires a single empty-leaf to produce a data_root.
		h := sha256.Sum256(nil)
		return []Chunk{{
			DataHash:     h[:],
			Data:         []byte{},
			MinByteRange: 0,
			MaxByteRange: 0,
		}}
	}

	var chunks []Chunk
	rest := data
	cursor := 0
	for len(rest) >= MaxChunkSize {
		size := MaxChunkSize
		nextSize := len(rest) - MaxChunkSize
		if nextSize > 0 && nextSize < MinChunkSize {
			// Rebalance: split rest evenly so both halves are >= MinChunkSize.
			size = (len(rest) + 1) / 2
		}
		chunkBytes := rest[:size]
		hash := sha256.Sum256(chunkBytes)
		cursor += len(chunkBytes)
		chunks = append(chunks, Chunk{
			DataHash:     hash[:],
			Data:         chunkBytes,
			MinByteRange: cursor - len(chunkBytes),
			MaxByteRange: cursor,
		})
		rest = rest[size:]
	}
	if len(rest) > 0 {
		hash := sha256.Sum256(rest)
		cursor += len(rest)
		chunks = append(chunks, Chunk{
			DataHash:     hash[:],
			Data:         rest,
			MinByteRange: cursor - len(rest),
			MaxByteRange: cursor,
		})
	}
	return chunks
}

// noteBuffer encodes n as a 32-byte big-endian unsigned integer, the
// Arweave "note" representation used inside merkle leaf and branch hashes.
func noteBuffer(n int) []byte {
	buf := make([]byte, NoteSize)
	// Spec is unsigned big-endian; uint64 fits all real-world byte ranges.
	binary.BigEndian.PutUint64(buf[NoteSize-8:], uint64(n))
	return buf
}

// hash256 returns sha256(concat(parts)).
func hash256(parts ...[]byte) []byte {
	h := sha256.New()
	for _, p := range parts {
		h.Write(p)
	}
	return h.Sum(nil)
}

// buildLeaves builds the leaf layer of the merkle tree.
func buildLeaves(chunks []Chunk) []*node {
	out := make([]*node, len(chunks))
	for i, c := range chunks {
		// id = sha256( sha256(dataHash) || sha256(note(maxByteRange)) )
		id := hash256(hash256(c.DataHash), hash256(noteBuffer(c.MaxByteRange)))
		out[i] = &node{
			id:           id,
			isLeaf:       true,
			dataHash:     c.DataHash,
			minByteRange: c.MinByteRange,
			maxByteRange: c.MaxByteRange,
		}
	}
	return out
}

// buildLayer pairs adjacent nodes into branches, propagating an odd
// trailing node up unchanged (Arweave merkle convention).
func buildLayer(nodes []*node) []*node {
	if len(nodes) <= 1 {
		return nodes
	}
	var next []*node
	for i := 0; i < len(nodes); i += 2 {
		if i+1 >= len(nodes) {
			next = append(next, nodes[i])
			continue
		}
		left, right := nodes[i], nodes[i+1]
		// id = sha256( sha256(left.id) || sha256(right.id) || sha256(note(left.maxByteRange)) )
		id := hash256(hash256(left.id), hash256(right.id), hash256(noteBuffer(left.maxByteRange)))
		next = append(next, &node{
			id:           id,
			leftChild:    left,
			rightChild:   right,
			byteRange:    left.maxByteRange,
			maxByteRange: right.maxByteRange,
		})
	}
	return next
}

// buildMerkleTree returns the root and the leaf layer (leaves are needed
// later for path generation).
func buildMerkleTree(chunks []Chunk) (root *node, leaves []*node) {
	leaves = buildLeaves(chunks)
	if len(leaves) == 0 {
		return nil, nil
	}
	layer := leaves
	for len(layer) > 1 {
		layer = buildLayer(layer)
	}
	return layer[0], leaves
}

// generatePath returns the merkle proof bytes for the chunk at leafIdx.
//
// Path layout (concatenated, root → leaf):
//
//	for each branch on the path:
//	    left.id (32) || right.id (32) || note(byteRange) (32)   = 96 bytes
//	leaf:
//	    dataHash (32) || note(maxByteRange) (32)                = 64 bytes
//
// Routing at each branch is deterministic by byte-range: we descend into
// the subtree whose maxByteRange covers the target leaf's offset.
func generatePath(root *node, leafIdx int, leaves []*node) []byte {
	target := leaves[leafIdx]
	var path []byte
	cur := root
	for cur != nil && !cur.isLeaf {
		path = append(path, cur.leftChild.id...)
		path = append(path, cur.rightChild.id...)
		path = append(path, noteBuffer(cur.byteRange)...)
		// target.maxByteRange is the leaf's right edge; left subtree owns
		// everything up to and including byteRange.
		if target.maxByteRange <= cur.byteRange {
			cur = cur.leftChild
		} else {
			cur = cur.rightChild
		}
	}
	// Append leaf descriptor.
	path = append(path, target.dataHash...)
	path = append(path, noteBuffer(target.maxByteRange)...)
	return path
}
