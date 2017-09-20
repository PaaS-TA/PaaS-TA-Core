// This file was generated by counterfeiter
package fakes

import (
	"net"
	"sync"
	"time"
)

type FakePacketConn struct {
	ReadFromStub        func(b []byte) (n int, addr net.Addr, err error)
	readFromMutex       sync.RWMutex
	readFromArgsForCall []struct {
		b []byte
	}
	readFromReturns struct {
		result1 int
		result2 net.Addr
		result3 error
	}
	WriteToStub        func(b []byte, addr net.Addr) (n int, err error)
	writeToMutex       sync.RWMutex
	writeToArgsForCall []struct {
		b    []byte
		addr net.Addr
	}
	writeToReturns struct {
		result1 int
		result2 error
	}
	CloseStub        func() error
	closeMutex       sync.RWMutex
	closeArgsForCall []struct{}
	closeReturns     struct {
		result1 error
	}
	LocalAddrStub        func() net.Addr
	localAddrMutex       sync.RWMutex
	localAddrArgsForCall []struct{}
	localAddrReturns     struct {
		result1 net.Addr
	}
	SetDeadlineStub        func(t time.Time) error
	setDeadlineMutex       sync.RWMutex
	setDeadlineArgsForCall []struct {
		t time.Time
	}
	setDeadlineReturns struct {
		result1 error
	}
	SetReadDeadlineStub        func(t time.Time) error
	setReadDeadlineMutex       sync.RWMutex
	setReadDeadlineArgsForCall []struct {
		t time.Time
	}
	setReadDeadlineReturns struct {
		result1 error
	}
	SetWriteDeadlineStub        func(t time.Time) error
	setWriteDeadlineMutex       sync.RWMutex
	setWriteDeadlineArgsForCall []struct {
		t time.Time
	}
	setWriteDeadlineReturns struct {
		result1 error
	}
}

func (fake *FakePacketConn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	fake.readFromMutex.Lock()
	fake.readFromArgsForCall = append(fake.readFromArgsForCall, struct {
		b []byte
	}{b})
	fake.readFromMutex.Unlock()
	if fake.ReadFromStub != nil {
		return fake.ReadFromStub(b)
	} else {
		return fake.readFromReturns.result1, fake.readFromReturns.result2, fake.readFromReturns.result3
	}
}

func (fake *FakePacketConn) ReadFromCallCount() int {
	fake.readFromMutex.RLock()
	defer fake.readFromMutex.RUnlock()
	return len(fake.readFromArgsForCall)
}

func (fake *FakePacketConn) ReadFromArgsForCall(i int) []byte {
	fake.readFromMutex.RLock()
	defer fake.readFromMutex.RUnlock()
	return fake.readFromArgsForCall[i].b
}

func (fake *FakePacketConn) ReadFromReturns(result1 int, result2 net.Addr, result3 error) {
	fake.ReadFromStub = nil
	fake.readFromReturns = struct {
		result1 int
		result2 net.Addr
		result3 error
	}{result1, result2, result3}
}

func (fake *FakePacketConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	fake.writeToMutex.Lock()
	fake.writeToArgsForCall = append(fake.writeToArgsForCall, struct {
		b    []byte
		addr net.Addr
	}{b, addr})
	fake.writeToMutex.Unlock()
	if fake.WriteToStub != nil {
		return fake.WriteToStub(b, addr)
	} else {
		return fake.writeToReturns.result1, fake.writeToReturns.result2
	}
}

func (fake *FakePacketConn) WriteToCallCount() int {
	fake.writeToMutex.RLock()
	defer fake.writeToMutex.RUnlock()
	return len(fake.writeToArgsForCall)
}

func (fake *FakePacketConn) WriteToArgsForCall(i int) ([]byte, net.Addr) {
	fake.writeToMutex.RLock()
	defer fake.writeToMutex.RUnlock()
	return fake.writeToArgsForCall[i].b, fake.writeToArgsForCall[i].addr
}

func (fake *FakePacketConn) WriteToReturns(result1 int, result2 error) {
	fake.WriteToStub = nil
	fake.writeToReturns = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *FakePacketConn) Close() error {
	fake.closeMutex.Lock()
	fake.closeArgsForCall = append(fake.closeArgsForCall, struct{}{})
	fake.closeMutex.Unlock()
	if fake.CloseStub != nil {
		return fake.CloseStub()
	} else {
		return fake.closeReturns.result1
	}
}

func (fake *FakePacketConn) CloseCallCount() int {
	fake.closeMutex.RLock()
	defer fake.closeMutex.RUnlock()
	return len(fake.closeArgsForCall)
}

func (fake *FakePacketConn) CloseReturns(result1 error) {
	fake.CloseStub = nil
	fake.closeReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakePacketConn) LocalAddr() net.Addr {
	fake.localAddrMutex.Lock()
	fake.localAddrArgsForCall = append(fake.localAddrArgsForCall, struct{}{})
	fake.localAddrMutex.Unlock()
	if fake.LocalAddrStub != nil {
		return fake.LocalAddrStub()
	} else {
		return fake.localAddrReturns.result1
	}
}

func (fake *FakePacketConn) LocalAddrCallCount() int {
	fake.localAddrMutex.RLock()
	defer fake.localAddrMutex.RUnlock()
	return len(fake.localAddrArgsForCall)
}

func (fake *FakePacketConn) LocalAddrReturns(result1 net.Addr) {
	fake.LocalAddrStub = nil
	fake.localAddrReturns = struct {
		result1 net.Addr
	}{result1}
}

func (fake *FakePacketConn) SetDeadline(t time.Time) error {
	fake.setDeadlineMutex.Lock()
	fake.setDeadlineArgsForCall = append(fake.setDeadlineArgsForCall, struct {
		t time.Time
	}{t})
	fake.setDeadlineMutex.Unlock()
	if fake.SetDeadlineStub != nil {
		return fake.SetDeadlineStub(t)
	} else {
		return fake.setDeadlineReturns.result1
	}
}

func (fake *FakePacketConn) SetDeadlineCallCount() int {
	fake.setDeadlineMutex.RLock()
	defer fake.setDeadlineMutex.RUnlock()
	return len(fake.setDeadlineArgsForCall)
}

func (fake *FakePacketConn) SetDeadlineArgsForCall(i int) time.Time {
	fake.setDeadlineMutex.RLock()
	defer fake.setDeadlineMutex.RUnlock()
	return fake.setDeadlineArgsForCall[i].t
}

func (fake *FakePacketConn) SetDeadlineReturns(result1 error) {
	fake.SetDeadlineStub = nil
	fake.setDeadlineReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakePacketConn) SetReadDeadline(t time.Time) error {
	fake.setReadDeadlineMutex.Lock()
	fake.setReadDeadlineArgsForCall = append(fake.setReadDeadlineArgsForCall, struct {
		t time.Time
	}{t})
	fake.setReadDeadlineMutex.Unlock()
	if fake.SetReadDeadlineStub != nil {
		return fake.SetReadDeadlineStub(t)
	} else {
		return fake.setReadDeadlineReturns.result1
	}
}

func (fake *FakePacketConn) SetReadDeadlineCallCount() int {
	fake.setReadDeadlineMutex.RLock()
	defer fake.setReadDeadlineMutex.RUnlock()
	return len(fake.setReadDeadlineArgsForCall)
}

func (fake *FakePacketConn) SetReadDeadlineArgsForCall(i int) time.Time {
	fake.setReadDeadlineMutex.RLock()
	defer fake.setReadDeadlineMutex.RUnlock()
	return fake.setReadDeadlineArgsForCall[i].t
}

func (fake *FakePacketConn) SetReadDeadlineReturns(result1 error) {
	fake.SetReadDeadlineStub = nil
	fake.setReadDeadlineReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakePacketConn) SetWriteDeadline(t time.Time) error {
	fake.setWriteDeadlineMutex.Lock()
	fake.setWriteDeadlineArgsForCall = append(fake.setWriteDeadlineArgsForCall, struct {
		t time.Time
	}{t})
	fake.setWriteDeadlineMutex.Unlock()
	if fake.SetWriteDeadlineStub != nil {
		return fake.SetWriteDeadlineStub(t)
	} else {
		return fake.setWriteDeadlineReturns.result1
	}
}

func (fake *FakePacketConn) SetWriteDeadlineCallCount() int {
	fake.setWriteDeadlineMutex.RLock()
	defer fake.setWriteDeadlineMutex.RUnlock()
	return len(fake.setWriteDeadlineArgsForCall)
}

func (fake *FakePacketConn) SetWriteDeadlineArgsForCall(i int) time.Time {
	fake.setWriteDeadlineMutex.RLock()
	defer fake.setWriteDeadlineMutex.RUnlock()
	return fake.setWriteDeadlineArgsForCall[i].t
}

func (fake *FakePacketConn) SetWriteDeadlineReturns(result1 error) {
	fake.SetWriteDeadlineStub = nil
	fake.setWriteDeadlineReturns = struct {
		result1 error
	}{result1}
}

var _ net.PacketConn = new(FakePacketConn)