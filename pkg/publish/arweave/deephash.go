package arweave

import (
	"crypto/sha256"
	"crypto/sha512"
	"strconv"
)

// deepHash implements Arweave's recursive SHA-384 deep-hash algorithm.
//
// Reference (Erlang): https://github.com/ArweaveTeam/arweave/blob/master/apps/arweave/src/ar_deep_hash.erl
//
// Rules:
//
//	deepHash(blob B)  = sha384( sha384("blob") || sha384(intToString(len(B))) || sha384(B) )
//	                   actually: tag(blob, len(B)) || sha384(B), then sha384 of concat
//	deepHash(list L)  = recursively fold left over L: acc starts at
//	                   sha384( sha384("list") || sha384(intToString(len(L))) ),
//	                   then for each item: acc = sha384(acc || deepHash(item)).
//
// All "blob"/"list" prefixes are UTF-8 string literals; lengths are decimal.
type deepHashChunk interface{ deepHashKind() }

// DHBlob is a leaf byte chunk in a deep-hash list.
type DHBlob []byte

func (DHBlob) deepHashKind() {}

// DHList is a nested deep-hash list.
type DHList []deepHashChunk

func (DHList) deepHashKind() {}

func sha384(data ...[]byte) []byte {
	h := sha512.New384()
	for _, d := range data {
		h.Write(d)
	}
	return h.Sum(nil)
}

// deepHash returns the 48-byte deep hash of x.
func deepHash(x deepHashChunk) []byte {
	switch v := x.(type) {
	case DHBlob:
		tag := append([]byte("blob"), []byte(strconv.Itoa(len(v)))...)
		// Per spec: H = sha384(sha384("blob" || len) || sha384(data))
		// But Arweave actually does:
		//   tagHash = sha384("blob" || strLen)
		//   sha384(tagHash || sha384(data))
		// The strLen is the *decimal-string* length, not the binary length:
		//   "blob" + utf8(itoa(len(data)))
		tagHash := sha384(tag)
		dataHash := sha384(v)
		return sha384(tagHash, dataHash)

	case DHList:
		tag := append([]byte("list"), []byte(strconv.Itoa(len(v)))...)
		acc := sha384(tag)
		for _, item := range v {
			acc = sha384(acc, deepHash(item))
		}
		return acc
	}
	panic("deepHash: unknown chunk")
}

// sha256Sum is a small convenience helper.
func sha256Sum(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}
