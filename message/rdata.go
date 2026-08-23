package message

import (
	"encoding/binary"
	"fmt"
)

// RData はResourceRecordのRDATA部分を表す。
// TYPEごとに構造化された実装(AData, MXDataなど)か、TYPEに対するフォールバックとしてRawDataが使われる。
type RData interface {
	// rdataType はこのRDATAに対応するTYPEを返す。
	rdataType() Type
	// marshal はRDATA本体(RDLENGTHを含まない)をbufに追記して返す。
	marshal(buf []byte) ([]byte, error)
}

// AData はTYPE=A(IPv4アドレス)のRDATAを表す。
type AData struct {
	Address [4]byte
}

func (r AData) rdataType() Type { return TypeA }

func (r AData) marshal(buf []byte) ([]byte, error) {
	return append(buf, r.Address[:]...), nil
}

// AAAAData はTYPE=AAAA(IPv6アドレス)のRDATAを表す。
type AAAAData struct {
	Address [16]byte
}

func (r AAAAData) rdataType() Type { return TypeAAAA }

func (r AAAAData) marshal(buf []byte) ([]byte, error) {
	return append(buf, r.Address[:]...), nil
}

// NSData はTYPE=NSのRDATAを表す。
type NSData struct {
	NSDName Name
}

func (r NSData) rdataType() Type { return TypeNS }

func (r NSData) marshal(buf []byte) ([]byte, error) {
	buf, err := r.NSDName.marshal(buf)
	if err != nil {
		return nil, fmt.Errorf("ns rdata: marshal nsdname: %w", err)
	}
	return buf, nil
}

// CNAMEData はTYPE=CNAMEのRDATAを表す。
type CNAMEData struct {
	CName Name
}

func (r CNAMEData) rdataType() Type { return TypeCNAME }

func (r CNAMEData) marshal(buf []byte) ([]byte, error) {
	buf, err := r.CName.marshal(buf)
	if err != nil {
		return nil, fmt.Errorf("cname rdata: marshal cname: %w", err)
	}
	return buf, nil
}

// MXData はTYPE=MXのRDATAを表す。
type MXData struct {
	Preference uint16
	Exchange   Name
}

func (r MXData) rdataType() Type { return TypeMX }

func (r MXData) marshal(buf []byte) ([]byte, error) {
	buf = binary.BigEndian.AppendUint16(buf, r.Preference)
	buf, err := r.Exchange.marshal(buf)
	if err != nil {
		return nil, fmt.Errorf("mx rdata: marshal exchange: %w", err)
	}
	return buf, nil
}

// TXTData はTYPE=TXTのRDATAを表す。
// 1つ以上のcharacter-string(1byte長プレフィックス)を保持する。
type TXTData struct {
	Txt []string
}

func (r TXTData) rdataType() Type { return TypeTXT }

func (r TXTData) marshal(buf []byte) ([]byte, error) {
	for _, s := range r.Txt {
		if len(s) > 255 {
			return nil, fmt.Errorf("txt rdata: character-string %q exceeds 255 bytes", s)
		}
		buf = append(buf, byte(len(s)))
		buf = append(buf, s...)
	}
	return buf, nil
}

// SOAData はTYPE=SOAのRDATAを表す。
type SOAData struct {
	MName   Name
	RName   Name
	Serial  uint32
	Refresh uint32
	Retry   uint32
	Expire  uint32
	Minimum uint32
}

func (r SOAData) rdataType() Type { return TypeSOA }

func (r SOAData) marshal(buf []byte) ([]byte, error) {
	buf, err := r.MName.marshal(buf)
	if err != nil {
		return nil, fmt.Errorf("soa rdata: marshal mname: %w", err)
	}

	buf, err = r.RName.marshal(buf)
	if err != nil {
		return nil, fmt.Errorf("soa rdata: marshal rname: %w", err)
	}

	buf = binary.BigEndian.AppendUint32(buf, r.Serial)
	buf = binary.BigEndian.AppendUint32(buf, r.Refresh)
	buf = binary.BigEndian.AppendUint32(buf, r.Retry)
	buf = binary.BigEndian.AppendUint32(buf, r.Expire)
	buf = binary.BigEndian.AppendUint32(buf, r.Minimum)

	return buf, nil
}

// RawData は未対応のTYPEに対するRDATAを、生バイト列のまま保持する。
type RawData struct {
	Type Type
	Data []byte
}

func (r RawData) rdataType() Type { return r.Type }

func (r RawData) marshal(buf []byte) ([]byte, error) {
	return append(buf, r.Data...), nil
}

// readRData はTYPEに応じたRDATAをデコードする。rdataEnd はRDATAの終端の絶対オフセット
// (d.pos基準)で、可変長のRDATA(TXTなど)を読み切るために使う。
func (d *decoder) readRData(typ Type, rdataEnd int) (RData, error) {
	switch typ {
	case TypeA:
		return d.readAData()
	case TypeAAAA:
		return d.readAAAAData()
	case TypeNS:
		return d.readNSData()
	case TypeCNAME:
		return d.readCNAMEData()
	case TypeMX:
		return d.readMXData()
	case TypeTXT:
		return d.readTXTData(rdataEnd)
	case TypeSOA:
		return d.readSOAData()
	default:
		return d.readRawData(typ, rdataEnd)
	}
}

func (d *decoder) readAData() (RData, error) {
	b, err := d.readBytes(4)
	if err != nil {
		return nil, fmt.Errorf("a rdata: %w", err)
	}
	return AData{Address: [4]byte(b)}, nil
}

func (d *decoder) readAAAAData() (RData, error) {
	b, err := d.readBytes(16)
	if err != nil {
		return nil, fmt.Errorf("aaaa rdata: %w", err)
	}
	return AAAAData{Address: [16]byte(b)}, nil
}

func (d *decoder) readNSData() (RData, error) {
	name, err := d.readName()
	if err != nil {
		return nil, fmt.Errorf("ns rdata: read nsdname: %w", err)
	}
	return NSData{NSDName: name}, nil
}

func (d *decoder) readCNAMEData() (RData, error) {
	name, err := d.readName()
	if err != nil {
		return nil, fmt.Errorf("cname rdata: read cname: %w", err)
	}
	return CNAMEData{CName: name}, nil
}

func (d *decoder) readMXData() (RData, error) {
	preference, err := d.readUint16()
	if err != nil {
		return nil, fmt.Errorf("mx rdata: read preference: %w", err)
	}

	exchange, err := d.readName()
	if err != nil {
		return nil, fmt.Errorf("mx rdata: read exchange: %w", err)
	}

	return MXData{Preference: preference, Exchange: exchange}, nil
}

func (d *decoder) readTXTData(rdataEnd int) (RData, error) {
	txt := []string{}
	for d.pos < rdataEnd {
		s, err := d.readCharacterString()
		if err != nil {
			return nil, fmt.Errorf("txt rdata: %w", err)
		}
		txt = append(txt, s)
	}
	return TXTData{Txt: txt}, nil
}

func (d *decoder) readSOAData() (RData, error) {
	mname, err := d.readName()
	if err != nil {
		return nil, fmt.Errorf("soa rdata: read mname: %w", err)
	}

	rname, err := d.readName()
	if err != nil {
		return nil, fmt.Errorf("soa rdata: read rname: %w", err)
	}

	serial, err := d.readUint32()
	if err != nil {
		return nil, fmt.Errorf("soa rdata: read serial: %w", err)
	}

	refresh, err := d.readUint32()
	if err != nil {
		return nil, fmt.Errorf("soa rdata: read refresh: %w", err)
	}

	retry, err := d.readUint32()
	if err != nil {
		return nil, fmt.Errorf("soa rdata: read retry: %w", err)
	}

	expire, err := d.readUint32()
	if err != nil {
		return nil, fmt.Errorf("soa rdata: read expire: %w", err)
	}

	minimum, err := d.readUint32()
	if err != nil {
		return nil, fmt.Errorf("soa rdata: read minimum: %w", err)
	}

	return SOAData{
		MName:   mname,
		RName:   rname,
		Serial:  serial,
		Refresh: refresh,
		Retry:   retry,
		Expire:  expire,
		Minimum: minimum,
	}, nil
}

func (d *decoder) readRawData(typ Type, rdataEnd int) (RData, error) {
	raw, err := d.readBytes(rdataEnd - d.pos)
	if err != nil {
		return nil, fmt.Errorf("raw rdata: %w", err)
	}

	data := make([]byte, len(raw))
	copy(data, raw)

	return RawData{Type: typ, Data: data}, nil
}
