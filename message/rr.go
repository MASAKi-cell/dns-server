package message

import (
	"encoding/binary"
	"fmt"
)

// ResourceRecord はAnswer/Authority/AdditionalセクションのRRを表す。
// RDLENGTHはRData.marshal/readRDataの結果から都度算出するため、フィールドとしては持たない。
type ResourceRecord struct {
	Name  Name
	Type  Type
	Class Class
	TTL   uint32
	RData RData
}

// marshal はResourceRecordをエンコードし、bufに追記して返す。
func (rr ResourceRecord) marshal(buf []byte) ([]byte, error) {
	buf, err := rr.Name.marshal(buf)
	if err != nil {
		return nil, fmt.Errorf("resource record: marshal name: %w", err)
	}

	buf = binary.BigEndian.AppendUint16(buf, uint16(rr.Type))
	buf = binary.BigEndian.AppendUint16(buf, uint16(rr.Class))
	buf = binary.BigEndian.AppendUint32(buf, rr.TTL)

	rdlengthPos := len(buf)
	buf = binary.BigEndian.AppendUint16(buf, 0) // 後で書き戻すプレースホルダー

	rdataStart := len(buf)
	buf, err = rr.RData.marshal(buf)
	if err != nil {
		return nil, fmt.Errorf("resource record: marshal rdata: %w", err)
	}

	rdlength := len(buf) - rdataStart
	if rdlength > 0xFFFF {
		return nil, fmt.Errorf("resource record: rdata length %d exceeds uint16 range", rdlength)
	}
	binary.BigEndian.PutUint16(buf[rdlengthPos:rdlengthPos+2], uint16(rdlength))

	return buf, nil
}

// readResourceRecord はAnswer/Authority/Additionalセクションの1エントリをデコードする。
func (d *decoder) readResourceRecord() (ResourceRecord, error) {
	name, err := d.readName()
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("resource record: read name: %w", err)
	}

	typ, err := d.readUint16()
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("resource record: read type: %w", err)
	}

	class, err := d.readUint16()
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("resource record: read class: %w", err)
	}

	ttl, err := d.readUint32()
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("resource record: read ttl: %w", err)
	}

	rdlength, err := d.readUint16()
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("resource record: read rdlength: %w", err)
	}

	rdataEnd := d.pos + int(rdlength)
	if rdataEnd > len(d.buf) {
		return ResourceRecord{}, fmt.Errorf("resource record: rdata length %d exceeds remaining buffer", rdlength)
	}

	rdata, err := d.readRData(Type(typ), rdataEnd)
	if err != nil {
		return ResourceRecord{}, fmt.Errorf("resource record: read rdata: %w", err)
	}

	if d.pos != rdataEnd {
		return ResourceRecord{}, fmt.Errorf(
			"resource record: rdata parser consumed %d bytes, want %d",
			d.pos-(rdataEnd-int(rdlength)), rdlength,
		)
	}

	return ResourceRecord{
		Name:  name,
		Type:  Type(typ),
		Class: Class(class),
		TTL:   ttl,
		RData: rdata,
	}, nil
}

// readResourceRecords はcount個のResourceRecordを連続してデコードする。
func (d *decoder) readResourceRecords(count int) ([]ResourceRecord, error) {
	rrs := make([]ResourceRecord, 0, count)
	for range count {
		rr, err := d.readResourceRecord()
		if err != nil {
			return nil, err
		}
		rrs = append(rrs, rr)
	}
	return rrs, nil
}
