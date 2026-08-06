package message

import (
	"reflect"
	"testing"
)

func TestResourceRecord_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		rr   ResourceRecord
	}{
		{
			name: "A",
			rr: ResourceRecord{
				Name:  "example.com.",
				Type:  TypeA,
				Class: ClassIN,
				TTL:   300,
				RData: AData{Address: [4]byte{1, 2, 3, 4}},
			},
		},
		{
			name: "AAAA",
			rr: ResourceRecord{
				Name:  "example.com.",
				Type:  TypeAAAA,
				Class: ClassIN,
				TTL:   300,
				RData: AAAAData{Address: [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}},
			},
		},
		{
			name: "NS",
			rr: ResourceRecord{
				Name:  "example.com.",
				Type:  TypeNS,
				Class: ClassIN,
				TTL:   3600,
				RData: NSData{NSDName: "ns1.example.com."},
			},
		},
		{
			name: "CNAME",
			rr: ResourceRecord{
				Name:  "www.example.com.",
				Type:  TypeCNAME,
				Class: ClassIN,
				TTL:   300,
				RData: CNAMEData{CName: "example.com."},
			},
		},
		{
			name: "MX",
			rr: ResourceRecord{
				Name:  "example.com.",
				Type:  TypeMX,
				Class: ClassIN,
				TTL:   3600,
				RData: MXData{Preference: 10, Exchange: "mail.example.com."},
			},
		},
		{
			name: "TXT",
			rr: ResourceRecord{
				Name:  "example.com.",
				Type:  TypeTXT,
				Class: ClassIN,
				TTL:   300,
				RData: TXTData{Txt: []string{"hello", "world"}},
			},
		},
		{
			name: "SOA",
			rr: ResourceRecord{
				Name:  "example.com.",
				Type:  TypeSOA,
				Class: ClassIN,
				TTL:   3600,
				RData: SOAData{
					MName:   "ns1.example.com.",
					RName:   "admin.example.com.",
					Serial:  2024010101,
					Refresh: 7200,
					Retry:   3600,
					Expire:  1209600,
					Minimum: 300,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := tt.rr.marshal(nil)
			if err != nil {
				t.Fatalf("marshal() error = %v", err)
			}

			d := newDecoder(buf)
			got, err := d.readResourceRecord()
			if err != nil {
				t.Fatalf("readResourceRecord() error = %v", err)
			}

			if !reflect.DeepEqual(got, tt.rr) {
				t.Fatalf("round trip mismatch:\ngot  %+v\nwant %+v", got, tt.rr)
			}
			if d.pos != len(buf) {
				t.Fatalf("decoder.pos = %d, want %d", d.pos, len(buf))
			}
		})
	}
}

// TestResourceRecord_ReadName_CompressionPointer は、NAME/RDATA双方が
// 手前に出現した名前を圧縮ポインタで参照するケースを検証する。
func TestResourceRecord_ReadName_CompressionPointer(t *testing.T) {
	buf := []byte{
		// オフセット0: "example.com."
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,
		// オフセット13から: RR NAME = ポインタ(0を指す)
		0xC0, 0x00,
		0x00, 0x02, // TYPE=NS
		0x00, 0x01, // CLASS=IN
		0x00, 0x00, 0x0e, 0x10, // TTL=3600
		0x00, 0x02, // RDLENGTH=2
		0xC0, 0x00, // RDATA: NSDNAME = ポインタ(0を指す)
	}

	d := &decoder{buf: buf, pos: 13}
	got, err := d.readResourceRecord()
	if err != nil {
		t.Fatalf("readResourceRecord() error = %v", err)
	}

	want := ResourceRecord{
		Name:  "example.com.",
		Type:  TypeNS,
		Class: ClassIN,
		TTL:   3600,
		RData: NSData{NSDName: "example.com."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\ngot  %+v\nwant %+v", got, want)
	}
	if d.pos != len(buf) {
		t.Fatalf("decoder.pos = %d, want %d", d.pos, len(buf))
	}
}

func TestResourceRecord_UnknownType(t *testing.T) {
	rr := ResourceRecord{
		Name:  "example.com.",
		Type:  Type(65280), // private use, 未対応の型
		Class: ClassIN,
		TTL:   300,
		RData: RawData{Type: Type(65280), Data: []byte{0xDE, 0xAD, 0xBE, 0xEF}},
	}

	buf, err := rr.marshal(nil)
	if err != nil {
		t.Fatalf("marshal() error = %v", err)
	}

	d := newDecoder(buf)
	got, err := d.readResourceRecord()
	if err != nil {
		t.Fatalf("readResourceRecord() error = %v", err)
	}

	if !reflect.DeepEqual(got, rr) {
		t.Fatalf("round trip mismatch:\ngot  %+v\nwant %+v", got, rr)
	}
}

func TestResourceRecord_RDLengthMismatch(t *testing.T) {
	buf := []byte{
		0, // NAME = root
		0x00, 0x01, // TYPE=A
		0x00, 0x01, // CLASS=IN
		0x00, 0x00, 0x00, 0x3c, // TTL
		0x00, 0x08, // RDLENGTH=8 (Aレコードなのに8byte分確保されている)
		1, 2, 3, 4, 5, 6, 7, 8,
	}

	d := newDecoder(buf)
	if _, err := d.readResourceRecord(); err == nil {
		t.Fatal("readResourceRecord() error = nil, want error for rdlength mismatch")
	}
}
