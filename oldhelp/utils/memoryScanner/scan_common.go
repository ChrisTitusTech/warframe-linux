package memoryScanner

import (
	"bytes"
	"io"
)

const scanChunkSize = 1 << 20

type authzStatus uint8

const (
	authzInvalid authzStatus = iota
	authzComplete
	authzIncomplete
)

func scanRegionChunked(regionSize uintptr, readChunk func(offset uintptr, dst []byte) (int, error)) (map[string]int, error) {
	candidates := make(map[string]int)
	if regionSize == 0 {
		return candidates, nil
	}

	pattern := []byte(authzPattern)
	buf := make([]byte, scanChunkSize)
	carry := make([]byte, 0, len(pattern)-1)

	for offset := uintptr(0); offset < regionSize; {
		size := len(buf)
		if remaining := regionSize - offset; remaining < uintptr(size) {
			size = int(remaining)
		}

		n, err := readChunk(offset, buf[:size])
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, io.ErrUnexpectedEOF
		}

		carry = scanChunk(carry, buf[:n], offset+uintptr(n) >= regionSize, candidates)
		offset += uintptr(n)
	}

	return candidates, nil
}

func scanChunk(carry, chunk []byte, final bool, candidates map[string]int) []byte {
	combined := append(carry, chunk...)
	start := 0
	carryStart := -1

	for start < len(combined) {
		idx := bytes.Index(combined[start:], []byte(authzPattern))
		if idx == -1 {
			break
		}
		idx += start

		authz, status := extractAuthz(combined[idx:], final)
		switch status {
		case authzComplete:
			candidates[authz]++
		case authzIncomplete:
			carryStart = idx
			start = len(combined)
			continue
		}

		start = idx + len(authzPattern)
	}

	if final {
		return nil
	}
	if carryStart != -1 {
		return bytes.Clone(combined[carryStart:])
	}

	tailStart := len(combined) - len(authzPattern) + 1
	if tailStart < 0 {
		tailStart = 0
	}
	return bytes.Clone(combined[tailStart:])
}

func extractAuthz(buf []byte, final bool) (string, authzStatus) {
	const accountIDLen = 24
	const noncePrefix = "&nonce="

	need := len(authzPattern) + accountIDLen
	if len(buf) < need {
		if final {
			return "", authzInvalid
		}
		return "", authzIncomplete
	}

	offset := len(authzPattern)
	accountID := string(buf[offset : offset+accountIDLen])
	offset += accountIDLen

	if len(buf[offset:]) < len(noncePrefix) {
		if final {
			return "", authzInvalid
		}
		return "", authzIncomplete
	}
	if !bytes.HasPrefix(buf[offset:], []byte(noncePrefix)) {
		return "", authzInvalid
	}
	offset += len(noncePrefix)

	digitStart := offset
	for offset < len(buf) {
		c := buf[offset]
		if c < '0' || c > '9' {
			break
		}
		offset++
	}

	if offset == digitStart {
		if offset < len(buf) {
			return "", authzInvalid
		}
		if final {
			return "", authzInvalid
		}
		return "", authzIncomplete
	}
	if offset == len(buf) && !final {
		return "", authzIncomplete
	}

	return authzPattern + accountID + noncePrefix + string(buf[digitStart:offset]), authzComplete
}

func findConfident(candidates map[string]int) string {
	for authz, count := range candidates {
		if count >= authzConfidence {
			return authz
		}
	}
	return ""
}

func mergeCandidates(dst, src map[string]int) {
	for authz, count := range src {
		dst[authz] += count
	}
}
