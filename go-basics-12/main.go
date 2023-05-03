package circular

import "errors"

// Implement a circular buffer of bytes supporting both overflow-checked writes
// and unconditional, possibly overwriting, writes.
//
// We chose the provided API so that Buffer implements io.ByteReader
// and io.ByteWriter and can be used (size permitting) as a drop in
// replacement for anything using that interface.

// Define the Buffer type here.
type Buffer struct {
	data    []byte
	read    int
	write   int
	content int
}

func NewBuffer(size int) *Buffer {
	return &Buffer{data: make([]byte, size)}
}

func (buf *Buffer) ReadByte() (byte, error) {
	if buf.content == 0 {
		return 0, errors.New("buffer is empty")
	}
	buf.content--
	b := buf.data[buf.read]
	buf.read++
	buf.read %= len(buf.data)
	return b, nil
}

func (buf *Buffer) WriteByte(c byte) error {
	if buf.content == len(buf.data) {
		return errors.New("buffer is full")
	}
	buf.content++
	buf.data[buf.write] = c
	buf.write++
	buf.write %= len(buf.data)
	return nil
}

func (buf *Buffer) Overwrite(c byte) {
	if buf.content == len(buf.data) {
		buf.ReadByte()
	}
	buf.WriteByte(c)
}

func (buf *Buffer) Reset() {
	buf.content = 0
	buf.read = 0
	buf.write = 0
}
