package msgpackzip

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type testVector struct {
	name   string
	data   string
	inFile bool
}

var vectors = []testVector{
	{
		"inbox-2",
		"g61jb252ZXJzYXRpb25zkYanZXhwdW5nZYKlYmFzaXMApHVwdG8Ar21heE1zZ1N1bW1hcmllc5KFpWN0aW1lzwAAAWbp7L/1q21lc3NhZ2VUeXBlAaVtc2dJRAKndGxmTmFtZap0ZXN0LHRlc3QxqXRsZlB1YmxpY8KFpWN0aW1lzwAAAWbp7L8wq21lc3NhZ2VUeXBlBqVtc2dJRAGndGxmTmFtZap0ZXN0LHRlc3QxqXRsZlB1YmxpY8KnbWF4TXNnc8CobWV0YWRhdGGNqmFjdGl2ZUxpc3SRxBAbTw6YUZcZmOcyB4VEyWsZp2FsbExpc3SSxBAbTw6YUZcZmOcyB4VEyWsZxBCfhtCBiEx9ZZov6qDFWtAZrmNvbnZlcnNhdGlvbklExCAAANGzkOwmkujKzkTl1HxTBvhpb5ZCNgE6/R5WXkpOSalleGlzdGVuY2UAqGlkVHJpcGxlg6V0bGZpZMQQg9mnw8NksSgbME8qTYp5JKd0b3BpY0lExBAfp8PZdZ2QR9u6l+IrYEAgqXRvcGljVHlwZQGrbWVtYmVyc1R5cGUCqXJlc2V0TGlzdMCmc3RhdHVzAKxzdXBlcnNlZGVkQnnAqnN1cGVyc2VkZXPAqHRlYW1UeXBlAKd2ZXJzaW9uzQJJqnZpc2liaWxpdHkCrW5vdGlmaWNhdGlvbnOCq2NoYW5uZWxXaWRlw6hzZXR0aW5nc4IBggHDAMMAggHDAMOqcmVhZGVySW5mb4SobWF4TXNnaWQCpW10aW1lzwAAAWbp7L/4qXJlYWRNc2dpZAKmc3RhdHVzAKpwYWdpbmF0aW9uhKRsYXN0w6RuZXh0xDCCoUPEIAAA0bOQ7CaS6MrOROXUfFMG+GlvlkI2ATr9HlZeSk5JoU3PAAABZunsv/ijbnVtAahwcmV2aW91c8QwgqFDxCAAANGzkOwmkujKzkTl1HxTBvhpb5ZCNgE6/R5WXkpOSaFNzwAAAWbp7L/4pHZlcnPNAkk=",
		false,
	},
	{
		"inbox-3-cropped",
		"inbox-3-cropped.b64",
		true,
	},
	{
		"thread-106",
		"thread-106.b64",
		true,
	},
	{
		"ints",
		"k4SjYWFhAaNiYmLMgaNjY2POAAEAAqRkZGRkzwAAAAEAAAADhKNhYWH/o2JiYtDYo2NjY9L//v/+o2RkZNP////+/////YOjYWFhgqNiYmLAo2NjY8OjZGRkkwECA6NlZWXC",
		false,
	},
	{
		"megatest",
		"megatest.b64",
		true,
	},
}

func loadTestVector(t *testing.T, tv testVector) []byte {
	var data string
	var err error
	if tv.inFile {
		path := filepath.Clean(filepath.Join("testdata", tv.data)) // relative path
		raw, err := os.ReadFile(path)
		data = string(raw)
		require.NoError(t, err)
	} else {
		data = tv.data
	}
	b, err := base64.StdEncoding.DecodeString(data)
	require.NoError(t, err)
	return b
}

func TestSimpleVectors(t *testing.T) {
	for _, v := range vectors {
		b := loadTestVector(t, v)
		test1(t, v.name, b)
	}
}

func test1(t *testing.T, name string, dat []byte) {
	out, err := Compress(dat)
	require.NoError(t, err)
	fmt.Printf("%s: %d -> %d\n", name, len(dat), len(out))
	dat2, err := Inflate(out)
	require.NoError(t, err)
	require.True(t, bytes.Equal(dat, dat2))
}

func TestReportValuesFrequencies(t *testing.T) {
	b := loadTestVector(t, vectors[2])
	v, err := ReportValuesFrequencies(b)
	require.NoError(t, err)
	for _, freq := range v {
		if freq.Freq < 2 {
			continue
		}
		var k string
		switch t := freq.Key.(type) {
		case BinaryMapKey:
			k = "b:" + hex.EncodeToString([]byte(string(t)))
		case string:
			k = t
		case int:
			k = fmt.Sprintf("%d", t)
		default:
			k = "nil"
		}
		fmt.Printf("%s\t%d\n", k, freq.Freq)
	}
}

func TestCompressWithWhitelist(t *testing.T) {
	d := func(s string) []byte {
		ret, _ := hex.DecodeString(s)
		return ret
	}

	// This white list was devined using the above test, TestReportValuesFrequencies
	wl := NewValueWhitelist()
	wl.AddString("team1")
	wl.AddBinary(d("6fb1e234cad5e24be2fa84809ef0d518"))
	wl.AddBinary(d("1b4f0e9851971998e732078544c96b19"))
	wl.AddBinary(d("4a8d6e318170eef7233b6cccedf8d1dea4828fbca1c6d5fe279d4be13994d50c0e0c7b655655a9188a6eaf73a90198ec643c4ea66f68ac46a1eaea7ba2489ee1"))
	wl.AddBinary(d("2b38d80c7fb55c8001754c0559f7d520"))
	wl.AddBinary(d("57601e2c28b9a72eb7aa95559c096c24"))
	wl.AddBinary(d("01209647b483afa8f6c0a2c79dd5aa86660250a2ad8370cf9dcaf3edb35572bf973c0a"))

	b := loadTestVector(t, vectors[2])
	out, err := CompressWithWhitelist(b, *wl)
	require.NoError(t, err)
	fmt.Printf("compressed t106: %d -> %d\n", len(b), len(out))
	dat2, err := Inflate(out)
	require.NoError(t, err)
	require.True(t, bytes.Equal(b, dat2))
}

func TestIntegerOverflowProtection(t *testing.T) {
	t.Run("msgpackInt_toUint32_overflow", func(t *testing.T) {
		// Test uint64 value exceeding MaxUint32
		mpi := msgpackInt{typ: intTypeUint64, uval: uint64(math.MaxUint32) + 1}
		_, err := mpi.toUint32()
		require.Error(t, err)
		require.Contains(t, err.Error(), "int")

		// Test negative int64 value
		mpi = msgpackInt{typ: intTypeInt32, val: -1}
		_, err = mpi.toUint32()
		require.Error(t, err)
		require.Contains(t, err.Error(), "int")

		// Test int64 value exceeding MaxUint32
		mpi = msgpackInt{typ: intTypeInt64, val: int64(math.MaxUint32) + 1}
		_, err = mpi.toUint32()
		require.Error(t, err)
		require.Contains(t, err.Error(), "int")
	})

	t.Run("msgpackInt_toLen_negative", func(t *testing.T) {
		// Test negative length
		mpi := msgpackInt{typ: intTypeInt32, val: -1}
		_, err := mpi.toLen()
		require.Error(t, err)
		require.Contains(t, err.Error(), "negative length")
	})

	t.Run("outputInt_range_validation", func(t *testing.T) {
		var o outputter

		// Test uint8 overflow
		mpi := msgpackInt{typ: intTypeUint8, val: 256}
		err := o.outputInt(mpi)
		require.Error(t, err)
		require.Contains(t, err.Error(), "uint8")

		// Test uint16 overflow
		mpi = msgpackInt{typ: intTypeUint16, val: 65536}
		err = o.outputInt(mpi)
		require.Error(t, err)
		require.Contains(t, err.Error(), "uint16")

		// Test uint32 overflow
		mpi = msgpackInt{typ: intTypeUint32, val: int64(math.MaxUint32) + 1}
		err = o.outputInt(mpi)
		require.Error(t, err)
		require.Contains(t, err.Error(), "uint32")

		// Test int8 underflow
		mpi = msgpackInt{typ: intTypeInt8, val: -129}
		err = o.outputInt(mpi)
		require.Error(t, err)
		require.Contains(t, err.Error(), "int8")

		// Test int8 overflow
		mpi = msgpackInt{typ: intTypeInt8, val: 128}
		err = o.outputInt(mpi)
		require.Error(t, err)
		require.Contains(t, err.Error(), "int8")

		// Test int16 underflow
		mpi = msgpackInt{typ: intTypeInt16, val: -32769}
		err = o.outputInt(mpi)
		require.Error(t, err)
		require.Contains(t, err.Error(), "int16")

		// Test int16 overflow
		mpi = msgpackInt{typ: intTypeInt16, val: 32768}
		err = o.outputInt(mpi)
		require.Error(t, err)
		require.Contains(t, err.Error(), "int16")

		// Test int32 underflow
		mpi = msgpackInt{typ: intTypeInt32, val: int64(math.MinInt32) - 1}
		err = o.outputInt(mpi)
		require.Error(t, err)
		require.Contains(t, err.Error(), "int32")

		// Test int32 overflow
		mpi = msgpackInt{typ: intTypeInt32, val: int64(math.MaxInt32) + 1}
		err = o.outputInt(mpi)
		require.Error(t, err)
		require.Contains(t, err.Error(), "int32")

		// Test negative value for fixed uint
		mpi = msgpackInt{typ: intTypeFixedUint, val: -1}
		err = o.outputInt(mpi)
		require.Error(t, err)
		require.Contains(t, err.Error(), "fixed uint")

		// Test out of range for fixed int
		mpi = msgpackInt{typ: intTypeFixedInt, val: 0}
		err = o.outputInt(mpi)
		require.Error(t, err)
		require.Contains(t, err.Error(), "fixed int")
	})

	t.Run("outputStringOrUintOrBinary_negative_int64", func(t *testing.T) {
		var o outputter

		// Test negative int64 conversion to uint
		err := o.outputStringOrUintOrBinary(int64(-1))
		require.Error(t, err)
		require.Contains(t, err.Error(), "negative int64")
	})

	t.Run("outputContainerPrefix_overflow", func(t *testing.T) {
		var o outputter

		// Test uint16 overflow in container prefix
		mpi := msgpackInt{typ: intTypeUint16, val: 65536}
		err := o.outputContainerPrefix(mpi, 0xa0, 0x1f, 0xd9, 0xda, 0xdb)
		require.Error(t, err)
		require.Contains(t, err.Error(), "uint16")

		// Test uint32 overflow in container prefix
		mpi = msgpackInt{typ: intTypeUint32, val: int64(math.MaxUint32) + 1}
		err = o.outputContainerPrefix(mpi, 0xa0, 0x1f, 0xd9, 0xda, 0xdb)
		require.Error(t, err)
		require.Contains(t, err.Error(), "uint32")
	})

	t.Run("valid_conversions_should_pass", func(t *testing.T) {
		var o outputter

		// Test valid uint8
		mpi := msgpackInt{typ: intTypeUint8, val: 255}
		err := o.outputInt(mpi)
		require.NoError(t, err)

		// Test valid int8 (negative)
		mpi = msgpackInt{typ: intTypeInt8, val: -128}
		err = o.outputInt(mpi)
		require.NoError(t, err)

		// Test valid int8 (positive)
		mpi = msgpackInt{typ: intTypeInt8, val: 127}
		err = o.outputInt(mpi)
		require.NoError(t, err)

		// Test valid uint32
		mpi = msgpackInt{typ: intTypeUint32, val: math.MaxUint32}
		err = o.outputInt(mpi)
		require.NoError(t, err)

		// Test valid toUint32
		mpi = msgpackInt{typ: intTypeUint32, val: math.MaxUint32}
		val, err := mpi.toUint32()
		require.NoError(t, err)
		require.Equal(t, uint32(math.MaxUint32), val)

		// Test valid toLen
		mpi = msgpackInt{typ: intTypeUint32, val: 1000}
		length, err := mpi.toLen()
		require.NoError(t, err)
		require.Equal(t, 1000, length)
	})
}
