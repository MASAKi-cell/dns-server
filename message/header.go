package message

import (
	"encoding/binary"
	"fmt"
)

// headerSize はHeaderのワイヤーフォーマット長(byte)。
const headerSize = 12

// Header はDNSメッセージ先頭の12byte固定長ヘッダを表す(RFC1035 4.1.1)。
type Header struct {
	ID uint16 // クライアントが発行する16bitの識別子

	QR     bool   // Query(0)かResponse(1)かを示す1bit
	Opcode Opcode // クエリの種類(標準クエリ=0、旧IQUERY=1、サーバーステータス要求=2)
	AA     bool   // Authoritative Answer。応答者がそのゾーンの権威サーバーかどうか
	TC     bool   // TrunCation。UDPの512byte制限などでメッセージが切り詰められたことを示す。これが立っていたらクライアントはTCPで再送すべき
	RD     bool   // クライアントがサーバーに再帰的な名前解決を要求するかどうか
	RA     bool   // Recursion Available。サーバーが再帰問い合わせに対応しているかどうか
	Z      uint8  // 3bit, reserved (must be zero)
	RCode  RCode  // 応答結果のエラーコード

	QDCount uint16 // Question section のエントリ数
	ANCount uint16 // Answer section のリソースレコード数
	NSCount uint16 // Authority section のリソースレコード数
	ARCount uint16 // Additional section のリソースレコード数
}

// marshal はHeaderをエンコードし、bufに追記して返す。
func (h Header) marshal(buf []byte) []byte {
	buf = binary.BigEndian.AppendUint16(buf, h.ID)
	buf = binary.BigEndian.AppendUint16(buf, h.flags())
	buf = binary.BigEndian.AppendUint16(buf, h.QDCount)
	buf = binary.BigEndian.AppendUint16(buf, h.ANCount)
	buf = binary.BigEndian.AppendUint16(buf, h.NSCount)
	buf = binary.BigEndian.AppendUint16(buf, h.ARCount)
	return buf
}

func (h Header) flags() uint16 {
	var flags uint16
	if h.QR {
		flags |= 1 << 15
	}
	flags |= uint16(h.Opcode&0xF) << 11
	if h.AA {
		flags |= 1 << 10
	}
	if h.TC {
		flags |= 1 << 9
	}
	if h.RD {
		flags |= 1 << 8
	}
	if h.RA {
		flags |= 1 << 7
	}
	flags |= uint16(h.Z&0x7) << 4
	flags |= uint16(h.RCode & 0xF)
	return flags
}

// readHeader は先頭12byteをデコードしてHeaderを返す。
func (d *decoder) readHeader() (Header, error) {
	if len(d.buf) < headerSize {
		return Header{}, fmt.Errorf("header: buffer too short: got %d bytes, want at least %d", len(d.buf), headerSize)
	}

	id, err := d.readUint16()
	if err != nil {
		return Header{}, fmt.Errorf("header: read id: %w", err)
	}

	flags, err := d.readUint16()
	if err != nil {
		return Header{}, fmt.Errorf("header: read flags: %w", err)
	}

	qdCount, err := d.readUint16()
	if err != nil {
		return Header{}, fmt.Errorf("header: read qdcount: %w", err)
	}

	anCount, err := d.readUint16()
	if err != nil {
		return Header{}, fmt.Errorf("header: read ancount: %w", err)
	}

	nsCount, err := d.readUint16()
	if err != nil {
		return Header{}, fmt.Errorf("header: read nscount: %w", err)
	}

	arCount, err := d.readUint16()
	if err != nil {
		return Header{}, fmt.Errorf("header: read arcount: %w", err)
	}

	return Header{
		ID:      id,
		QR:      flags&(1<<15) != 0,
		Opcode:  Opcode(flags>>11) & 0xF,
		AA:      flags&(1<<10) != 0,
		TC:      flags&(1<<9) != 0,
		RD:      flags&(1<<8) != 0,
		RA:      flags&(1<<7) != 0,
		Z:       uint8(flags>>4) & 0x7,
		RCode:   RCode(flags & 0xF),
		QDCount: qdCount,
		ANCount: anCount,
		NSCount: nsCount,
		ARCount: arCount,
	}, nil
}
