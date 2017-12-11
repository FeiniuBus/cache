package cache

import "errors"
import "encoding/json"

// A Sink receives data from a Get call.
type Sink interface {
	// SetString sets the value to s.
	SetString(s string) error

	// SetBytes sets the value to the contents of v.
	SetBytes(v []byte) error

	// SetJSON sets the value to the encoded version of m.
	SetJSON(m interface{}) error

	// view returns a frozen view of the bytes for caching.
	view() (ByteView, error)
}

func setSinkView(s Sink, v ByteView) error {
	type viewSetter interface {
		setView(v ByteView) error
	}
	if vs, ok := s.(viewSetter); ok {
		return vs.setView(v)
	}
	if v.b != nil {
		return s.SetBytes(v.b)
	}
	return s.SetString(v.s)
}

// StringSink returns a Sink that populates the provided string pointer.
func StringSink(sp *string) Sink {
	return &stringSink{sp: sp}
}

type stringSink struct {
	sp *string
	v  ByteView
}

func (s *stringSink) view() (ByteView, error) {
	return s.v, nil
}

func (s *stringSink) SetString(v string) error {
	s.v.b = nil
	s.v.s = v
	*s.sp = v
	return nil
}

func (s *stringSink) SetBytes(v []byte) error {
	return s.SetString(string(v))
}

func (s *stringSink) SetJSON(m interface{}) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	s.v.b = b
	*s.sp = string(b)
	return nil
}

// ByteViewSink returns a Sink that populates a ByteView.
func ByteViewSink(dst *ByteView) Sink {
	if dst == nil {
		panic("nil dst")
	}
	return &byteViewSink{dst: dst}
}

type byteViewSink struct {
	dst *ByteView
}

func (s *byteViewSink) setView(v ByteView) error {
	*s.dst = v
	return nil
}

func (s *byteViewSink) view() (ByteView, error) {
	return *s.dst, nil
}

func (s *byteViewSink) SetBytes(b []byte) error {
	*s.dst = ByteView{b: cloneBytes(b)}
	return nil
}

func (s *byteViewSink) SetString(v string) error {
	*s.dst = ByteView{s: v}
	return nil
}

func (s *byteViewSink) SetJSON(m interface{}) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	*s.dst = ByteView{b: b}
	return nil
}

// AllocatingByteSliceSink returns a Sink that allocates
// a byte slice to hold received value and assigns
// it to *dst.
func AllocatingByteSliceSink(dst *[]byte) Sink {
	return &allocBytesSink{dst: dst}
}

type allocBytesSink struct {
	dst *[]byte
	v   ByteView
}

func (s *allocBytesSink) view() (ByteView, error) {
	return s.v, nil
}

func (s *allocBytesSink) setView(v ByteView) error {
	if v.b != nil {
		*s.dst = cloneBytes(v.b)
	} else {
		*s.dst = []byte(v.s)
	}
	s.v = v
	return nil
}

func (s *allocBytesSink) SetBytes(b []byte) error {
	return s.setBytesOwned(cloneBytes(b))
}

func (s *allocBytesSink) setBytesOwned(b []byte) error {
	if s.dst == nil {
		return errors.New("nil AllocatingByteSliceSink *[]byte dst")
	}
	*s.dst = cloneBytes(b) // another copy, protecting s.v.b
	s.v.b = b
	s.v.s = ""
	return nil
}

func (s *allocBytesSink) SetString(v string) error {
	if s.dst == nil {
		return errors.New("nil AllocatingByteSliceSink *[]byte dst")
	}
	*s.dst = []byte(v)
	s.v.b = nil
	s.v.s = v
	return nil
}

func (s *allocBytesSink) SetJSON(m interface{}) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return s.setBytesOwned(b)
}

// JSONSink returns a sink that unmarshals binary values into m.
func JSONSink(m interface{}) Sink {
	return &jsonSink{
		dst: m,
	}
}

type jsonSink struct {
	dst interface{}
	typ string

	v ByteView
}

func (s *jsonSink) view() (ByteView, error) {
	return s.v, nil
}

func (s *jsonSink) SetBytes(b []byte) error {
	err := json.Unmarshal(b, s.dst)
	if err != nil {
		return err
	}
	s.v.b = cloneBytes(b)
	s.v.s = ""
	return nil
}

func (s *jsonSink) SetString(v string) error {
	b := []byte(v)
	err := json.Unmarshal(b, s.dst)
	if err != nil {
		return err
	}
	s.v.b = b
	s.v.s = ""
	return nil
}

func (s *jsonSink) SetJSON(m interface{}) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, s.dst)
	if err != nil {
		return err
	}
	s.v.b = b
	s.v.s = ""
	return nil
}
