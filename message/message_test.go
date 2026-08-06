package message

import (
	"reflect"
	"testing"
)

func TestMessage_MarshalUnmarshal_Query(t *testing.T) {
	msg := Message{
		Header: Header{
			ID:     0x1234,
			Opcode: OpcodeQuery,
			RD:     true,
		},
		Questions: []Question{
			{Name: "example.com.", Type: TypeA, Class: ClassIN},
		},
		Answers:     []ResourceRecord{},
		Authorities: []ResourceRecord{},
		Additionals: []ResourceRecord{},
	}

	buf, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	got, err := Unmarshal(buf)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := msg
	want.Header.QDCount = 1

	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("round trip mismatch:\ngot  %+v\nwant %+v", *got, want)
	}
}

func TestMessage_MarshalUnmarshal_ResponseWithAnswer(t *testing.T) {
	msg := Message{
		Header: Header{
			ID:     0x1234,
			QR:     true,
			Opcode: OpcodeQuery,
			RD:     true,
			RA:     true,
			RCode:  RCodeSuccess,
		},
		Questions: []Question{
			{Name: "example.com.", Type: TypeA, Class: ClassIN},
		},
		Answers: []ResourceRecord{
			{
				Name:  "example.com.",
				Type:  TypeA,
				Class: ClassIN,
				TTL:   300,
				RData: AData{Address: [4]byte{1, 2, 3, 4}},
			},
		},
		Authorities: []ResourceRecord{},
		Additionals: []ResourceRecord{},
	}

	buf, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	got, err := Unmarshal(buf)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := msg
	want.Header.QDCount = 1
	want.Header.ANCount = 1

	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("round trip mismatch:\ngot  %+v\nwant %+v", *got, want)
	}
}

// TestMessage_Unmarshal_KnownBytesWithCompression は、Answerセクションの名前が
// Questionセクションの名前を圧縮ポインタで参照する、実際のDNS応答に近い
// 手作りバイト列をデコードして検証する。
func TestMessage_Unmarshal_KnownBytesWithCompression(t *testing.T) {
	buf := []byte{
		// Header (12 bytes)
		0x12, 0x34, // ID
		0x81, 0x80, // flags: QR=1 RD=1 RA=1
		0x00, 0x01, // QDCOUNT=1
		0x00, 0x01, // ANCOUNT=1
		0x00, 0x00, // NSCOUNT=0
		0x00, 0x00, // ARCOUNT=0

		// Question (offset 12..29)
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,
		0x00, 0x01, // QTYPE=A
		0x00, 0x01, // QCLASS=IN

		// Answer (offset 29..45)
		0xC0, 0x0C, // NAME = pointer to offset 12
		0x00, 0x01, // TYPE=A
		0x00, 0x01, // CLASS=IN
		0x00, 0x00, 0x01, 0x2C, // TTL=300
		0x00, 0x04, // RDLENGTH=4
		1, 2, 3, 4, // RDATA
	}

	got, err := Unmarshal(buf)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := &Message{
		Header: Header{
			ID:      0x1234,
			QR:      true,
			RD:      true,
			RA:      true,
			QDCount: 1,
			ANCount: 1,
		},
		Questions: []Question{
			{Name: "example.com.", Type: TypeA, Class: ClassIN},
		},
		Answers: []ResourceRecord{
			{
				Name:  "example.com.",
				Type:  TypeA,
				Class: ClassIN,
				TTL:   300,
				RData: AData{Address: [4]byte{1, 2, 3, 4}},
			},
		},
		Authorities: []ResourceRecord{},
		Additionals: []ResourceRecord{},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Unmarshal() mismatch:\ngot  %+v\nwant %+v", got, want)
	}
}

func TestMessage_Unmarshal_TooShort(t *testing.T) {
	if _, err := Unmarshal([]byte{0x00, 0x01}); err == nil {
		t.Fatal("Unmarshal() error = nil, want error for short buffer")
	}
}
