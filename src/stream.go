package src

import (
	"io"
	"net"
	"sync"
	"errors"
	"sync/atomic"
)

type Session struct {
	r io.Reader
	w io.Writer
	wlock sync.Mutex

	next_id uint16
	streams map[uint16]*Stream
	idlock sync.Mutex
	on_conn func (addr net.TCPAddr, streamid uint16) (s *Stream, err error)
}

func (s *Session) Dail (addr net.TCPAddr) (stream *Stream, err error) {
	return
}

func (s *Session) Auth (username string, password string) (err error) {
	return
}

func (s *Session) GetAuth (on_auth func (username string, password string) (bool)) (err error) {
	return
}

func (s *Session) GetNextId () (id uint16, err error) {
	s.idlock.Lock()
	defer s.idlock.Unlock()

	startid := s.next_id;
	_, ok := s.streams[s.next_id]
	for ok {
		s.next_id += 1
		if s.next_id == startid {
			return 0, errors.New("run out of stream id")
		}
		_, ok = s.streams[s.next_id]
	}
	id = s.next_id
	s.next_id += 1
	return id, nil
}

func (s *Session) Run () {
	var err error
	
	for {
		f, _ := ReadFrame(s.r)

		switch ft := f.(type) {
		default:
			panic("what the hell")
		case *FrameOK:
			// ??
		case *FrameFAILED:
			// ??
		case *FrameData:
			stream, ok := s.streams[ft.streamid]
			if !ok {
				// failed
			}
			_, err := stream.pw.Write(ft.data)
			// write all?
			if err != nil {
				panic(err)
			}
		case *FrameSyn:
			stream, ok := s.streams[ft.streamid]
			if !ok {
				// failed
			}
			addr, _ := ft.GetTcpAddr()
			stream, err = s.on_conn(addr, ft.streamid)
			if err != nil {
				// failed
				// fr := new(FrameFAILED)
				// fr.streamid = 
			} else {
				s.streams[ft.streamid] = stream
				fr := new(FrameOK)
				fr.streamid = ft.streamid
				fr.WriteFrame(s.w)
			}
		case *FrameAck:
			stream, ok := s.streams[ft.streamid]
			if !ok {
				// failed
			}
			atomic.AddUint32(&stream.write_window, ft.move_window)
		case *FrameFin:
			stream, ok := s.streams[ft.streamid]
			if !ok {
				// failed
			}
			stream.read_closed = true
			if stream.write_closed {
				stream.on_close()
			}
		case *FrameRst:
			stream, ok := s.streams[ft.streamid]
			if !ok {
				// failed
			}
			stream.on_close()
		}
	}
}

type Stream struct {
	s *Session
	streamid uint16

	write_closed bool
	read_closed bool

	write_window uint32
	pr io.PipeReader // will this block?
	pw io.PipeWriter
}

func (s *Stream) Read(p []byte) (n int, err error) {
	if s.read_closed {
		return 0, io.EOF
	}

	n, err = s.pr.Read(p)
	if err != nil {
		return
	}
	// s.s.Write()
	// read data
	// send msg_ack back
	return
}

func (s *Stream) Write(p []byte) (n int, err error) {
	if s.write_closed {
		return 0, io.EOF
	}

	// check s.write_window
	fd := &FrameData{streamid: s.streamid, data: p}
	s.s.wlock.Lock()
	defer s.s.wlock.Unlock()
	fd.WriteFrame(s.s.w)
	return
}

func (s *Stream) Close() error {
	s.write_closed = true
	// send MSG_FIN to remote
	if s.read_closed {
		s.on_close()
	}
	return nil
}

func (s *Stream) on_close() {
	delete(s.s.streams, s.streamid)
}