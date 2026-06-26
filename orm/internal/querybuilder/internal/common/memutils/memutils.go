package memutils

// ZeroBytes overwrites the byte slice with zeros.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// ZeroInterfaces overwrites the elements of an interface slice with nil.
func ZeroInterfaces(s []interface{}) {
	for i := range s {
		s[i] = nil
	}
}
