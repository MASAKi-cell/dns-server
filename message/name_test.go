package message

import (
	"reflect"
	"testing"
)

func TestName_MarshalKnownBytes(t *testing.T) {
	tests := []struct {
		name string
		n    Name
		want []byte
	}{
		{
			name: "root",
			n:    ".",
			want: []byte{0x00},
		},
		{
			name: "www.example.com.",
			n:    "www.example.com.",
			want: []byte{
				3, 'w', 'w', 'w',
				7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
				3, 'c', 'o', 'm',
				0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.n.marshal(nil)
			if err != nil {
				t.Fatalf("marshal() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("marshal() = % x, want % x", got, tt.want)
			}
		})
	}
}

func TestName_MarshalLabelTooLong(t *testing.T) {
	longLabel := make([]byte, maxLabelLength+1)
	for i := range longLabel {
		longLabel[i] = 'a'
	}
	n := Name(string(longLabel) + ".")

	if _, err := n.marshal(nil); err == nil {
		t.Fatal("marshal() error = nil, want error for over-length label")
	}
}

func TestName_MarshalEmptyLabel(t *testing.T) {
	n := Name("www..com.")
	if _, err := n.marshal(nil); err == nil {
		t.Fatal("marshal() error = nil, want error for empty label")
	}
}

func TestDecoder_ReadName_NoCompression(t *testing.T) {
	buf := []byte{
		3, 'w', 'w', 'w',
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,
	}

	d := newDecoder(buf)
	got, err := d.readName()
	if err != nil {
		t.Fatalf("readName() error = %v", err)
	}

	want := Name("www.example.com.")
	if got != want {
		t.Fatalf("readName() = %q, want %q", got, want)
	}
	if d.pos != len(buf) {
		t.Fatalf("decoder.pos = %d, want %d", d.pos, len(buf))
	}
}

func TestDecoder_ReadName_Root(t *testing.T) {
	d := newDecoder([]byte{0})
	got, err := d.readName()
	if err != nil {
		t.Fatalf("readName() error = %v", err)
	}
	if got != Name(".") {
		t.Fatalf("readName() = %q, want %q", got, ".")
	}
}

// TestDecoder_ReadName_CompressionPointer は、
// "www.example.com." を先頭に置き、続く名前がそれを圧縮ポインタで参照するケースを検証する。
func TestDecoder_ReadName_CompressionPointer(t *testing.T) {
	buf := []byte{
		3, 'w', 'w', 'w',
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,
		// オフセット17から: "mail" + ポインタ(オフセット0を指す)
		4, 'm', 'a', 'i', 'l',
		0xC0, 0x00,
	}

	d := &decoder{buf: buf, pos: 17}
	got, err := d.readName()
	if err != nil {
		t.Fatalf("readName() error = %v", err)
	}

	want := Name("mail.www.example.com.")
	if got != want {
		t.Fatalf("readName() = %q, want %q", got, want)
	}
	// ポインタジャンプ後もd.posはポインタ直後(バッファ末尾)を指す。
	if d.pos != len(buf) {
		t.Fatalf("decoder.pos = %d, want %d", d.pos, len(buf))
	}
}

func TestDecoder_ReadName_CompressionPointerLoop(t *testing.T) {
	// オフセット0のポインタが自分自身を指しているため、前方参照違反として弾かれる。
	buf := []byte{0xC0, 0x00}

	d := newDecoder(buf)
	if _, err := d.readName(); err == nil {
		t.Fatal("readName() error = nil, want error for self-referencing pointer")
	}
}

func TestDecoder_ReadName_TruncatedLabel(t *testing.T) {
	buf := []byte{5, 'a', 'b'}
	d := newDecoder(buf)
	if _, err := d.readName(); err == nil {
		t.Fatal("readName() error = nil, want error for truncated label")
	}
}
