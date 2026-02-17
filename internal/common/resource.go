package common

import "sync/atomic"

func (c *Common) GetTCPBuffer() []byte {
	buf := c.TCPBufferPool.Get().(*[]byte)
	return (*buf)[:TCPDataBufSize]
}

func (c *Common) PutTCPBuffer(buf []byte) {
	if buf != nil && cap(buf) >= TCPDataBufSize {
		c.TCPBufferPool.Put(&buf)
	}
}

func (c *Common) GetUDPBuffer() []byte {
	buf := c.UDPBufferPool.Get().(*[]byte)
	return (*buf)[:UDPDataBufSize]
}

func (c *Common) PutUDPBuffer(buf []byte) {
	if buf != nil && cap(buf) >= UDPDataBufSize {
		c.UDPBufferPool.Put(&buf)
	}
}

func (c *Common) TryAcquireSlot(isUDP bool) bool {
	if c.SlotLimit == 0 {
		return true
	}

	currentTotal := atomic.LoadInt32(&c.TCPSlot) + atomic.LoadInt32(&c.UDPSlot)
	if currentTotal >= c.SlotLimit {
		return false
	}

	if isUDP {
		atomic.AddInt32(&c.UDPSlot, 1)
	} else {
		atomic.AddInt32(&c.TCPSlot, 1)
	}
	return true
}

func (c *Common) ReleaseSlot(isUDP bool) {
	if c.SlotLimit == 0 {
		return
	}

	if isUDP {
		if current := atomic.LoadInt32(&c.UDPSlot); current > 0 {
			atomic.AddInt32(&c.UDPSlot, -1)
		}
	} else {
		if current := atomic.LoadInt32(&c.TCPSlot); current > 0 {
			atomic.AddInt32(&c.TCPSlot, -1)
		}
	}
}

func (c *Common) GetTCPRX() *uint64  { return &c.TCPRX }
func (c *Common) GetTCPTX() *uint64  { return &c.TCPTX }
func (c *Common) GetUDPRX() *uint64  { return &c.UDPRX }
func (c *Common) GetUDPTX() *uint64  { return &c.UDPTX }
func (c *Common) GetTCPSlot() *int32 { return &c.TCPSlot }
func (c *Common) GetUDPSlot() *int32 { return &c.UDPSlot }

func (c *Common) LoadTCPSlot() int32 { return atomic.LoadInt32(&c.TCPSlot) }
func (c *Common) LoadUDPSlot() int32 { return atomic.LoadInt32(&c.UDPSlot) }
func (c *Common) LoadTCPRX() uint64  { return atomic.LoadUint64(&c.TCPRX) }
func (c *Common) LoadTCPTX() uint64  { return atomic.LoadUint64(&c.TCPTX) }
func (c *Common) LoadUDPRX() uint64  { return atomic.LoadUint64(&c.UDPRX) }
func (c *Common) LoadUDPTX() uint64  { return atomic.LoadUint64(&c.UDPTX) }
