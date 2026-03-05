package gotmuxcc

// decodeOctal decodes tmux's octal escape encoding. Bytes below space (0x20)
// and backslash are encoded as \xxx (3-digit octal). All other printable ASCII
// bytes are passed through literally.
func decodeOctal(s string) []byte {
	buf := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			a, b, c := s[i+1], s[i+2], s[i+3]
			if isOctalDigit(a) && isOctalDigit(b) && isOctalDigit(c) {
				val := (a-'0')*64 + (b-'0')*8 + (c - '0')
				buf = append(buf, val)
				i += 3
				continue
			}
		}
		buf = append(buf, s[i])
	}
	return buf
}

func isOctalDigit(b byte) bool {
	return b >= '0' && b <= '7'
}
