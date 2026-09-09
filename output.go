package msgpackzip

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// limitedBuffer wraps bytes.Buffer and rejects writes that would push the
// total past maxSize (0 = no limit). It tracks total bytes written rather
// than using Len() so the check remains correct even if the buffer is
// partially drained between writes.
type limitedBuffer struct {
	bytes.Buffer
	maxSize int64
	written int64
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.maxSize > 0 && b.written+int64(len(p)) > b.maxSize {
		return 0, ErrOutputTooBig
	}
	n, err := b.Buffer.Write(p)
	b.written += int64(n)
	return n, err
}

type outputter struct {
	buf limitedBuffer
}

func newOutputterWithLimit(maxSize int64) outputter {
	return outputter{buf: limitedBuffer{maxSize: maxSize}}
}

func (o *outputter) Bytes() []byte {
	return o.buf.Bytes()
}

func (o *outputter) outputRawUint(i uint) error {
	return o.outputInt(msgpackIntFromUint(i))
}

func (o *outputter) outputMapPrefix(i msgpackInt) (err error) {
	return o.outputContainerPrefix(i, 0x80, 0x0f, 0x00, 0xde, 0xdf)
}

func (o *outputter) outputArrayPrefix(i msgpackInt) (err error) {
	return o.outputContainerPrefix(i, 0x90, 0x0f, 0x00, 0xdc, 0xdd)
}

func (o *outputter) outputByte(b byte) error {
	buf := []byte{b}
	_, err := o.buf.Write(buf)
	return err
}

func (o *outputter) outputBinaryInt(i any) error {
	return binary.Write(&o.buf, binary.BigEndian, i)
}

func (o *outputter) outputPrefixAndBinaryInt(b byte, i any) error {
	err := o.outputByte(b)
	if err != nil {
		return err
	}
	return binary.Write(&o.buf, binary.BigEndian, i)
}

func (o *outputter) outputContainerPrefix(i msgpackInt, fixed byte, numFixed byte, u8 byte, u16 byte, u32 byte) (err error) {
	switch i.typ {
	case intTypeFixedUint:
		// Validate value fits in byte range
		if i.val < 0 || i.val > 255 {
			return errors.New("integer overflow: value out of range for fixed uint")
		}
		if fixed != 0x0 && byte(i.val) <= numFixed {
			return o.outputByte(fixed | byte(i.val))
		}
		fallthrough
	case intTypeUint8:
		if u8 != 0x0 {
			// Validate value fits in byte range
			if i.val < 0 || i.val > 255 {
				return errors.New("integer overflow: value out of range for uint8")
			}
			var b [2]byte
			b[0] = u8
			b[1] = byte(i.val)
			_, err = o.buf.Write(b[:])
			return err
		}
		fallthrough
	case intTypeUint16:
		err = o.outputByte(u16)
		if err != nil {
			return err
		}
		if i.val < 0 || i.val > math.MaxUint16 {
			return errors.New("integer overflow: value out of range for uint16")
		}
		err = o.outputBinaryInt(uint16(i.val))
		return err
	case intTypeUint32:
		err = o.outputByte(u32)
		if err != nil {
			return err
		}
		if i.val < 0 || i.val > math.MaxUint32 {
			return errors.New("integer overflow: value out of range for uint32")
		}
		err = o.outputBinaryInt(uint32(i.val))
		return err
	default:
		return errors.New("bad container length")
	}
}

func (o *outputter) outputString(i msgpackInt, s string) (err error) {
	err = o.outputContainerPrefix(i, 0xa0, 0x1f, 0xd9, 0xda, 0xdb)
	if err != nil {
		return err
	}
	_, err = o.buf.Write([]byte(s))
	return err
}

func (o *outputter) outputBinary(i msgpackInt, b []byte) (err error) {
	err = o.outputContainerPrefix(i, 0x00, 0x00, 0xc4, 0xc5, 0xc6)
	if err != nil {
		return err
	}
	_, err = o.buf.Write(b)
	return err
}

func (o *outputter) outputNil() error {
	return o.outputByte(0xc0)
}

func (o *outputter) outputBool(b bool) error {
	var val byte
	if b {
		val = 0xc3
	} else {
		val = 0xc2
	}
	return o.outputByte(val)
}

func (o *outputter) outputInt(i msgpackInt) error {
	switch i.typ {
	case intTypeFixedUint:
		if i.val < 0 || i.val > 0x7f {
			return errors.New("integer overflow: value out of range for fixed uint")
		}
		return o.outputByte(byte(i.val))
	case intTypeFixedInt:
		if i.val < -32 || i.val >= 0 {
			return errors.New("integer overflow: value out of range for fixed int")
		}
		return o.outputByte(byte(0x100 + i.val))
	case intTypeUint8:
		if i.val < 0 || i.val > math.MaxUint8 {
			return errors.New("integer overflow: value out of range for uint8")
		}
		return o.outputPrefixAndBinaryInt(0xcc, uint8(i.val))
	case intTypeUint16:
		if i.val < 0 || i.val > math.MaxUint16 {
			return errors.New("integer overflow: value out of range for uint16")
		}
		return o.outputPrefixAndBinaryInt(0xcd, uint16(i.val))
	case intTypeUint32:
		if i.val < 0 || i.val > math.MaxUint32 {
			return errors.New("integer overflow: value out of range for uint32")
		}
		return o.outputPrefixAndBinaryInt(0xce, uint32(i.val))
	case intTypeUint64:
		return o.outputPrefixAndBinaryInt(0xcf, i.uval)
	case intTypeInt8:
		if i.val < math.MinInt8 || i.val > math.MaxInt8 {
			return fmt.Errorf("integer overflow: value %d out of range for int8 [%d, %d]", i.val, math.MinInt8, math.MaxInt8)
		}
		return o.outputPrefixAndBinaryInt(0xd0, int8(i.val))
	case intTypeInt16:
		if i.val < math.MinInt16 || i.val > math.MaxInt16 {
			return errors.New("integer overflow: value out of range for int16")
		}
		return o.outputPrefixAndBinaryInt(0xd1, int16(i.val))
	case intTypeInt32:
		if i.val < math.MinInt32 || i.val > math.MaxInt32 {
			return errors.New("integer overflow: value out of range for int32")
		}
		return o.outputPrefixAndBinaryInt(0xd2, int32(i.val))
	case intTypeInt64:
		return o.outputPrefixAndBinaryInt(0xd3, i.val)
	default:
		return errors.New("unhandled int output case")
	}
}

func (o *outputter) outputFloat32(b []byte) error {
	err := o.outputByte(0xca)
	if err != nil {
		return err
	}
	_, err = o.buf.Write(b)
	return err
}

func (o *outputter) outputFloat64(b []byte) error {
	err := o.outputByte(0xcb)
	if err != nil {
		return err
	}
	_, err = o.buf.Write(b)
	return err
}

func (o *outputter) decoderHooks() msgpackDecoderHooks {
	return msgpackDecoderHooks{
		mapStartHook: func(d decodeStack, i msgpackInt) (decodeStack, error) {
			err := o.outputMapPrefix(i)
			return d, err
		},
		arrayStartHook: func(d decodeStack, i msgpackInt) (decodeStack, error) {
			err := o.outputArrayPrefix(i)
			return d, err
		},
		stringHook:  o.outputString,
		binaryHook:  o.outputBinary,
		nilHook:     o.outputNil,
		boolHook:    o.outputBool,
		intHook:     o.outputInt,
		float32Hook: o.outputFloat32,
		float64Hook: o.outputFloat64,
	}
}

func (o *outputter) outputStringOrUintOrBinary(i any) error {
	switch t := i.(type) {
	case BinaryMapKey:
		return o.outputBinary(msgpackIntFromUint(uint(len(t))), []byte(t))
	case string:
		return o.outputString(msgpackIntFromUint(uint(len(t))), t)
	case int64:
		if t < 0 {
			return errors.New("integer overflow: cannot convert negative int64 to uint")
		}
		if uint64(t) > math.MaxUint {
			return errors.New("integer overflow: int64 value exceeds uint max")
		}
		return o.outputInt(msgpackIntFromUint(uint(t)))
	default:
		return errors.New("Unhandled map key interface type in output path")
	}
}

// we we substitute in a binary or string for something in our dictionary,
// we output it as a big endian integer, prefixed by the "external" byte.
func (o *outputter) outputExtUint(u uint) error {
	var i any
	var b byte
	switch {
	case u <= 0xff:
		b = 0xd4
		i = uint8(u)
	case u <= 0xffff:
		b = 0xd5
		i = uint16(u)
	case u <= 0xffffffff:
		b = 0xd6
		i = uint32(u)
	default:
		b = 0xd7
		i = uint64(u)
	}
	return o.outputPrefixAndBinaryInt(b, i)
}
