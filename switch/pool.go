package vswitch

import "sync"

var frameBufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 1518)
		return &buf
	},
}

func getFrameBuffer() []byte {
	return *frameBufferPool.Get().(*[]byte)
}

func putFrameBuffer(buf []byte) {
	if cap(buf) >= 1518 {
		buf = buf[:1518]
		frameBufferPool.Put(&buf)
	}
}
