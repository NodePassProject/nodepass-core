package common

import "sync/atomic"

func (c *Common) GetTCPBuffer() []byte {
	buf := c.TcpBufferPool.Get().(*[]byte)
	return (*buf)[:TcpDataBufSize]
}

func (c *Common) PutTCPBuffer(buf []byte) {
	if buf != nil && cap(buf) >= TcpDataBufSize {
		c.TcpBufferPool.Put(&buf)
	}
}

func (c *Common) GetUDPBuffer() []byte {
	buf := c.UdpBufferPool.Get().(*[]byte)
	return (*buf)[:UdpDataBufSize]
}

func (c *Common) PutUDPBuffer(buf []byte) {
	if buf != nil && cap(buf) >= UdpDataBufSize {
		c.UdpBufferPool.Put(&buf)
	}
}

func (c *Common) TryAcquireSlot(isUDP bool) bool {
	if c.SlotLimit == 0 {
		return true
	}

	currentTotal := atomic.LoadInt32(&c.TcpSlot) + atomic.LoadInt32(&c.UdpSlot)
	if currentTotal >= c.SlotLimit {
		return false
	}

	if isUDP {
		atomic.AddInt32(&c.UdpSlot, 1)
	} else {
		atomic.AddInt32(&c.TcpSlot, 1)
	}
	return true
}

func (c *Common) ReleaseSlot(isUDP bool) {
	if c.SlotLimit == 0 {
		return
	}

	if isUDP {
		if current := atomic.LoadInt32(&c.UdpSlot); current > 0 {
			atomic.AddInt32(&c.UdpSlot, -1)
		}
	} else {
		if current := atomic.LoadInt32(&c.TcpSlot); current > 0 {
			atomic.AddInt32(&c.TcpSlot, -1)
		}
	}
}

func (c *Common) GetTcpRX() *uint64  { return &c.TcpRX }
func (c *Common) GetTcpTX() *uint64  { return &c.TcpTX }
func (c *Common) GetUdpRX() *uint64  { return &c.UdpRX }
func (c *Common) GetUdpTX() *uint64  { return &c.UdpTX }
func (c *Common) GetTcpSlot() *int32 { return &c.TcpSlot }
func (c *Common) GetUdpSlot() *int32 { return &c.UdpSlot }

func (c *Common) LoadTcpSlot() int32 { return atomic.LoadInt32(&c.TcpSlot) }
func (c *Common) LoadUdpSlot() int32 { return atomic.LoadInt32(&c.UdpSlot) }
func (c *Common) LoadTcpRX() uint64  { return atomic.LoadUint64(&c.TcpRX) }
func (c *Common) LoadTcpTX() uint64  { return atomic.LoadUint64(&c.TcpTX) }
func (c *Common) LoadUdpRX() uint64  { return atomic.LoadUint64(&c.UdpRX) }
func (c *Common) LoadUdpTX() uint64  { return atomic.LoadUint64(&c.UdpTX) }
