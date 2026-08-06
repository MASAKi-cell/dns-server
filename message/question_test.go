package message

import (
	"reflect"
	"testing"
)

func TestQuestion_MarshalKnownBytes(t *testing.T) {
	q := Question{
		Name:  "example.com.",
		Type:  TypeA,
		Class: ClassIN,
	}

	want := []byte{
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,
		0x00, 0x01, // TYPE=A
		0x00, 0x01, // CLASS=IN
	}

	got, err := q.marshal(nil)
	if err != nil {
		t.Fatalf("marshal() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("marshal() = % x, want % x", got, want)
	}
}

func TestQuestion_RoundTrip(t *testing.T) {
	q := Question{
		Name:  "www.example.com.",
		Type:  TypeMX,
		Class: ClassIN,
	}

	buf, err := q.marshal(nil)
	if err != nil {
		t.Fatalf("marshal() error = %v", err)
	}

	d := newDecoder(buf)
	got, err := d.readQuestion()
	if err != nil {
		t.Fatalf("readQuestion() error = %v", err)
	}

	if got != q {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, q)
	}
	if d.pos != len(buf) {
		t.Fatalf("decoder.pos = %d, want %d", d.pos, len(buf))
	}
}
