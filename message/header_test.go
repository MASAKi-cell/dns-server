package message

import (
	"reflect"
	"testing"
)

func TestHeader_MarshalKnownBytes(t *testing.T) {
	h := Header{
		ID:      0x1234,
		QR:      true,
		Opcode:  OpcodeQuery,
		AA:      false,
		TC:      false,
		RD:      true,
		RA:      true,
		Z:       0,
		RCode:   RCodeSuccess,
		QDCount: 1,
		ANCount: 1,
		NSCount: 0,
		ARCount: 0,
	}

	want := []byte{
		0x12, 0x34, // ID
		0x81, 0x80, // flags: QR=1 Opcode=0000 AA=0 TC=0 RD=1 RA=1 Z=000 RCODE=0000
		0x00, 0x01, // QDCOUNT
		0x00, 0x01, // ANCOUNT
		0x00, 0x00, // NSCOUNT
		0x00, 0x00, // ARCOUNT
	}

	got := h.marshal(nil)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("marshal() = % x, want % x", got, want)
	}
}

func TestHeader_ReadKnownBytes(t *testing.T) {
	buf := []byte{
		0x12, 0x34,
		0x81, 0x80,
		0x00, 0x01,
		0x00, 0x01,
		0x00, 0x00,
		0x00, 0x00,
	}

	d := newDecoder(buf)
	got, err := d.readHeader()
	if err != nil {
		t.Fatalf("readHeader() error = %v", err)
	}

	want := Header{
		ID:      0x1234,
		QR:      true,
		Opcode:  OpcodeQuery,
		AA:      false,
		TC:      false,
		RD:      true,
		RA:      true,
		Z:       0,
		RCode:   RCodeSuccess,
		QDCount: 1,
		ANCount: 1,
		NSCount: 0,
		ARCount: 0,
	}

	if got != want {
		t.Fatalf("readHeader() = %+v, want %+v", got, want)
	}
	if d.pos != headerSize {
		t.Fatalf("decoder.pos = %d, want %d", d.pos, headerSize)
	}
}

func TestHeader_RoundTrip(t *testing.T) {
	h := Header{
		ID:      0xABCD,
		QR:      false,
		Opcode:  OpcodeStatus,
		AA:      true,
		TC:      true,
		RD:      false,
		RA:      false,
		Z:       0,
		RCode:   RCodeRefused,
		QDCount: 3,
		ANCount: 2,
		NSCount: 1,
		ARCount: 4,
	}

	buf := h.marshal(nil)

	d := newDecoder(buf)
	got, err := d.readHeader()
	if err != nil {
		t.Fatalf("readHeader() error = %v", err)
	}

	if got != h {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, h)
	}
}

func TestHeader_ReadTooShort(t *testing.T) {
	d := newDecoder([]byte{0x00, 0x01})
	if _, err := d.readHeader(); err == nil {
		t.Fatal("readHeader() error = nil, want error for short buffer")
	}
}
