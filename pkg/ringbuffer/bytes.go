package ringbuffer

import "io"

var _ io.ReadWriter = (*RingBuffer[byte])(nil)

func (rb *RingBuffer[byte]) Write(p []byte) (n int, err error) {
	for _, b := range p {
		rb.Push(b)
		n++
	}
	return n, nil
}

func (rb *RingBuffer[byte]) Read(p []byte) (n int, err error) {
	if rb.IsEmpty() {
		return 0, nil
	}

	for i := range p {
		b, ok := rb.TryPop()
		if !ok {
			if n == 0 {
				return 0, nil
			}
			return n, nil
		}
		p[i] = b
		n++
	}

	return n, nil
}
