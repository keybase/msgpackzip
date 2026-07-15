package msgpackzip

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

type inflator struct {
	input   *bytes.Buffer
	maxSize int64 // 0 = no limit
}

func newInflator(b []byte) *inflator {
	return newInflatorWithLimit(b, 0)
}

func newInflatorWithLimit(b []byte, maxSize int64) *inflator {
	return &inflator{input: bytes.NewBuffer(b), maxSize: maxSize}
}

func Inflate(input []byte) (output []byte, err error) {
	return newInflator(input).run()
}

// InflateWithLimit inflates input but returns ErrOutputTooBig if the
// decompressed output would exceed maxSize bytes. The output limit is enforced
// incrementally via limitedBuffer. Note: the internal keymap intermediate is
// always capped at bigLen (128 MB) regardless of maxSize.
func InflateWithLimit(input []byte, maxSize int64) (output []byte, err error) {
	return newInflatorWithLimit(input, maxSize).run()
}

func (c *inflator) run() (output []byte, err error) {
	version, compressedData, compressedKeymap, err := c.openOuter()
	if err != nil {
		return nil, err
	}
	if version != 1 {
		return nil, errors.New("can only support inflation of version 1")
	}

	keymap, err := c.inflateKeymap(compressedKeymap)
	if err != nil {
		return nil, err
	}

	ret, err := c.inflateData(keymap, compressedData)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (c *inflator) openOuter() (version int, compressedData []byte, compressedKeymap []byte, err error) {
	idx := 0

	hooks := msgpackDecoderHooks{
		arrayStartHook: func(d decodeStack, mpi msgpackInt) (decodeStack, error) {
			if idx != 0 {
				return d, fmt.Errorf("should only have an array at the top level, but found one at index=%d", idx)
			}
			i, err := mpi.toLen()
			if err != nil {
				return d, err
			}
			if i != 3 {
				return d, fmt.Errorf("outside encoding should have 3 elements; got %d", i)
			}
			return d, nil
		},
		intHook: func(mpi msgpackInt) error {
			i, err := mpi.toLen()
			if err != nil {
				return err
			}
			if idx != 0 {
				return fmt.Errorf("need a version as first index (found at %d)", idx)
			}
			version = i
			idx++
			return nil
		},
		binaryHook: func(_ msgpackInt, b []byte) error {
			switch idx {
			case 1:
				compressedData = b
			case 2:
				compressedKeymap = b
			default:
				return fmt.Errorf("got unexpected binary data at index=%d", idx)
			}
			idx++
			return nil
		},
		fallthroughHook: func(_ any, typ string) error {
			return fmt.Errorf("unexpected value of type %q at top level", typ)
		},
	}
	err = newMsgpackDecoder(c.input).run(hooks)
	if err != nil {
		return 0, nil, nil, err
	}
	return version, compressedData, compressedKeymap, nil
}

func (c *inflator) inflateKeymap(compressedKeymap []byte) (keymap map[uint]any, err error) {
	// Always cap keymap decompression at bigLen regardless of the caller-supplied
	// output limit: the keymap is an internal intermediate, not part of the output.
	rawKeymap, err := flateInflateWithLimit(compressedKeymap, bigLen)
	if err != nil {
		return nil, err
	}

	var isValue bool
	var key uint

	putKey := func(k uint) error {
		if isValue {
			return errors.New("got a call to putKey when we expected a value")
		}
		val, dup := keymap[k]
		if dup {
			return fmt.Errorf("got a duplicated key in the keyMap: %d -> %v", k, val)
		}
		key = k
		isValue = true
		return nil
	}

	putValue := func(i any) error {
		if !isValue {
			return errors.New("got a call to putValue when we expected a key")
		}
		keymap[key] = i
		isValue = false
		return nil
	}

	fallthroughHook := func(_ any, typ string) error {
		return fmt.Errorf("unexpected value of type %q in keymap", typ)
	}

	hooks := msgpackDecoderHooks{
		mapStartHook: func(d decodeStack, mpi msgpackInt) (decodeStack, error) {
			if keymap != nil {
				return d, fmt.Errorf("found a nested map in the keymap")
			}
			i, err := mpi.toLen()
			if err != nil {
				return d, err
			}
			keymap = make(map[uint]any, min(i, len(rawKeymap)/2))
			return d, nil
		},
		mapKeyHook: func(d decodeStack) (decodeStack, error) {
			d.hooks = msgpackDecoderHooks{
				intHook: func(mpi msgpackInt) error {
					i, err := mpi.toLen()
					if err != nil {
						return err
					}
					return putKey(uint(i)) //nolint:gosec // G115: toLen() validates i is non-negative and bounded
				},
				fallthroughHook: fallthroughHook,
			}
			return d, nil
		},
		mapValueHook: func(d decodeStack) (decodeStack, error) {
			d.hooks = msgpackDecoderHooks{
				intHook: func(mpi msgpackInt) error {
					i, err := mpi.toInt64()
					if err != nil {
						return err
					}
					return putValue(i)
				},
				stringHook: func(_ msgpackInt, s string) error {
					return putValue(s)
				},
				binaryHook: func(_ msgpackInt, b []byte) error {
					return putValue(BinaryMapKey(string(b)))
				},
				fallthroughHook: fallthroughHook,
			}
			return d, nil
		},
		fallthroughHook: fallthroughHook,
	}
	buf := bytes.NewBuffer(rawKeymap)
	err = newMsgpackDecoder(buf).run(hooks)
	if err != nil {
		return nil, err
	}
	return keymap, nil
}

func decodeBufToUint32(b []byte) (uint32, error) {
	switch len(b) {
	case 1:
		return uint32(b[0]), nil
	case 2:
		var u uint16
		err := binary.Read(bytes.NewBuffer(b), binary.BigEndian, &u)
		if err != nil {
			return 0, err
		}
		return uint32(u), nil
	case 4:
		var u uint32
		err := binary.Read(bytes.NewBuffer(b), binary.BigEndian, &u)
		if err != nil {
			return 0, err
		}
		return u, nil
	case 8:
		var u uint64
		err := binary.Read(bytes.NewBuffer(b), binary.BigEndian, &u)
		if err != nil {
			return 0, err
		}
		if u >= math.MaxUint32 {
			return 0, ErrIntTooBig
		}
		return uint32(u), nil
	default:
		return 0, fmt.Errorf("bad external element; len=%d", len(b))
	}
}

func (c *inflator) inflateData(keymap map[uint]any, compressedData []byte) (ret []byte, err error) {
	var data outputter
	if c.maxSize > 0 {
		data = newOutputterWithLimit(c.maxSize)
	}
	hooks := data.decoderHooks()
	hooks.mapKeyHook = func(d decodeStack) (decodeStack, error) {
		d.hooks = msgpackDecoderHooks{
			intHook: func(mpi msgpackInt) error {
				i, err := mpi.toLen()
				if err != nil {
					return err
				}
				key, ok := keymap[uint(i)] //nolint:gosec // G115: toLen() validates i is non-negative and bounded
				if !ok {
					return fmt.Errorf("Unknown map key: %d", i)
				}
				return data.outputStringOrUintOrBinary(key)
			},
			fallthroughHook: func(_ any, typ string) error {
				return fmt.Errorf("Expected only int map keys; got a %q", typ)
			},
		}
		return d, nil
	}
	hooks.extHook = func(b []byte) error {
		u, err := decodeBufToUint32(b)
		if err != nil {
			return err
		}
		key, ok := keymap[uint(u)]
		if !ok {
			return fmt.Errorf("Unknown map key: %d", u)
		}
		return data.outputStringOrUintOrBinary(key)
	}
	buf := bytes.NewBuffer(compressedData)
	err = newMsgpackDecoder(buf).run(hooks)
	if err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}
