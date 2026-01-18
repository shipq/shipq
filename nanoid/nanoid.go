package nanoid

import (
	"crypto/rand"
	"sync"
)

// Character set for the nanoid - 64 characters (2^6) for fast bit masking
// Using URL-safe characters: 0-9, a-z, A-Z, -, _
var alphabetChars = [64]byte{
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', // 0-9
	'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', // a-m
	'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', // n-z
	'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', // A-M
	'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', // N-Z
	'-', '_', // Additional URL-safe characters to make 64 total
}

const (
	bytesPerID   = 16                       // bytes needed per nanoid (21 chars × 6 bits = 126 bits)
	poolBufSize  = bytesPerID * 128         // buffer 128 IDs worth of random bytes per syscall
	poolBufCount = poolBufSize / bytesPerID // number of IDs we can generate per buffer
)

// randomBuffer holds a pre-fetched buffer of random bytes
type randomBuffer struct {
	data  [poolBufSize]byte
	index int // which ID slot we're on (0 to poolBufCount-1)
}

var bufferPool = sync.Pool{
	New: func() any {
		buf := &randomBuffer{}
		if _, err := rand.Read(buf.data[:]); err != nil {
			panic("failed to generate random bytes: " + err.Error())
		}
		return buf
	},
}

// New generates a cryptographically secure 21-character ID using a URL-friendly alphabet
// (e.g., 0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_).
func New() string {
	// Result buffer
	var result [21]byte

	// Get a buffer from the pool
	buf := bufferPool.Get().(*randomBuffer)

	// Calculate offset into the buffer for this ID
	offset := buf.index * bytesPerID
	randomBytes := buf.data[offset : offset+bytesPerID]

	// Extract 6-bit values using bit manipulation.
	// Pattern: every 4 characters consume exactly 3 bytes (24 bits = 4 × 6 bits)
	// - char 0: byte[i] bits 0-5
	// - char 1: byte[i] bits 6-7 + byte[i+1] bits 0-3
	// - char 2: byte[i+1] bits 4-7 + byte[i+2] bits 0-1
	// - char 3: byte[i+2] bits 2-7

	// Group 0: bytes 0-2 -> chars 0-3
	result[0] = alphabetChars[randomBytes[0]&63]
	result[1] = alphabetChars[((randomBytes[0]>>6)|(randomBytes[1]<<2))&63]
	result[2] = alphabetChars[((randomBytes[1]>>4)|(randomBytes[2]<<4))&63]
	result[3] = alphabetChars[(randomBytes[2]>>2)&63]

	// Group 1: bytes 3-5 -> chars 4-7
	result[4] = alphabetChars[randomBytes[3]&63]
	result[5] = alphabetChars[((randomBytes[3]>>6)|(randomBytes[4]<<2))&63]
	result[6] = alphabetChars[((randomBytes[4]>>4)|(randomBytes[5]<<4))&63]
	result[7] = alphabetChars[(randomBytes[5]>>2)&63]

	// Group 2: bytes 6-8 -> chars 8-11
	result[8] = alphabetChars[randomBytes[6]&63]
	result[9] = alphabetChars[((randomBytes[6]>>6)|(randomBytes[7]<<2))&63]
	result[10] = alphabetChars[((randomBytes[7]>>4)|(randomBytes[8]<<4))&63]
	result[11] = alphabetChars[(randomBytes[8]>>2)&63]

	// Group 3: bytes 9-11 -> chars 12-15
	result[12] = alphabetChars[randomBytes[9]&63]
	result[13] = alphabetChars[((randomBytes[9]>>6)|(randomBytes[10]<<2))&63]
	result[14] = alphabetChars[((randomBytes[10]>>4)|(randomBytes[11]<<4))&63]
	result[15] = alphabetChars[(randomBytes[11]>>2)&63]

	// Group 4: bytes 12-14 -> chars 16-19
	result[16] = alphabetChars[randomBytes[12]&63]
	result[17] = alphabetChars[((randomBytes[12]>>6)|(randomBytes[13]<<2))&63]
	result[18] = alphabetChars[((randomBytes[13]>>4)|(randomBytes[14]<<4))&63]
	result[19] = alphabetChars[(randomBytes[14]>>2)&63]

	// Final char: byte 15 -> char 20 (only need 1 char = 6 bits)
	result[20] = alphabetChars[randomBytes[15]&63]

	// Advance index; if exhausted, refill and reset
	buf.index++
	if buf.index >= poolBufCount {
		buf.index = 0
		if _, err := rand.Read(buf.data[:]); err != nil {
			panic("failed to generate random bytes: " + err.Error())
		}
	}

	// Return buffer to pool (after we're done reading from it!)
	bufferPool.Put(buf)

	return string(result[:])
}
