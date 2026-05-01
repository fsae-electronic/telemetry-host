package serial

import "errors"

var ErrCOBSDecode = errors.New("cobs: invalid encoded data")

// cobsEncode encodes src using Consistent Overhead Byte Stuffing.
// The output contains no 0x00 bytes.
func cobsEncode(src []byte) []byte {
	if len(src) == 0 {
		return []byte{0x01}
	}

	dst := make([]byte, 0, len(src)+len(src)/254+2)
	dst = append(dst, 0x00) // placeholder for first code byte
	codeIdx := 0
	code := byte(1)

	for _, b := range src {
		if b == 0x00 {
			dst[codeIdx] = code
			codeIdx = len(dst)
			dst = append(dst, 0x00) // placeholder
			code = 1
		} else {
			dst = append(dst, b)
			code++
			if code == 0xFF {
				dst[codeIdx] = code
				codeIdx = len(dst)
				dst = append(dst, 0x00) // placeholder
				code = 1
			}
		}
	}
	dst[codeIdx] = code
	return dst
}

// cobsDecode decodes a COBS-encoded byte slice (without the trailing 0x00 delimiter).
func cobsDecode(src []byte) ([]byte, error) {
	if len(src) == 0 {
		return nil, ErrCOBSDecode
	}

	dst := make([]byte, 0, len(src))
	idx := 0

	for idx < len(src) {
		code := int(src[idx])
		if code == 0 {
			return nil, ErrCOBSDecode
		}
		idx++

		for i := 1; i < code; i++ {
			if idx >= len(src) {
				return nil, ErrCOBSDecode
			}
			dst = append(dst, src[idx])
			idx++
		}

		// Insert a 0x00 between groups, but not after the final group
		if code != 0xFF && idx < len(src) {
			dst = append(dst, 0x00)
		}
	}
	return dst, nil
}
