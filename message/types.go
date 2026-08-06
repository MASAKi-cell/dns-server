// Package message はRFC1035 4節で定義されるDNSメッセージのワイヤーフォーマットを
// Goの型として表現し、そのエンコード/デコードを提供する。
package message

import "fmt"

// Type はリソースレコードのTYPE/QTYPEフィールドを表す(RFC1035 3.2.2, 3.2.3)。
type Type uint16

const (
	TypeA     Type = 1
	TypeNS    Type = 2
	TypeCNAME Type = 5
	TypeSOA   Type = 6
	TypeMX    Type = 15
	TypeTXT   Type = 16
	TypeAAAA  Type = 28 // RFC3596
)

func (t Type) String() string {
	switch t {
	case TypeA:
		return "A"
	case TypeNS:
		return "NS"
	case TypeCNAME:
		return "CNAME"
	case TypeSOA:
		return "SOA"
	case TypeMX:
		return "MX"
	case TypeTXT:
		return "TXT"
	case TypeAAAA:
		return "AAAA"
	default:
		return fmt.Sprintf("TYPE%d", uint16(t))
	}
}

// Class はリソースレコードのCLASS/QCLASSフィールドを表す(RFC1035 3.2.4, 3.2.5)。
type Class uint16

const (
	ClassIN Class = 1
	ClassCS Class = 2
	ClassCH Class = 3
	ClassHS Class = 4
)

func (c Class) String() string {
	switch c {
	case ClassIN:
		return "IN"
	case ClassCS:
		return "CS"
	case ClassCH:
		return "CH"
	case ClassHS:
		return "HS"
	default:
		return fmt.Sprintf("CLASS%d", uint16(c))
	}
}

// Opcode はHeaderのOPCODEフィールド(4bit)を表す(RFC1035 4.1.1)。
type Opcode uint8

const (
	OpcodeQuery  Opcode = 0
	OpcodeIQuery Opcode = 1
	OpcodeStatus Opcode = 2
)

func (o Opcode) String() string {
	switch o {
	case OpcodeQuery:
		return "QUERY"
	case OpcodeIQuery:
		return "IQUERY"
	case OpcodeStatus:
		return "STATUS"
	default:
		return fmt.Sprintf("OPCODE%d", uint8(o))
	}
}

// RCode はHeaderのRCODEフィールド(4bit)を表す(RFC1035 4.1.1)。
type RCode uint8

const (
	RCodeSuccess        RCode = 0
	RCodeFormatError    RCode = 1
	RCodeServerFailure  RCode = 2
	RCodeNameError      RCode = 3
	RCodeNotImplemented RCode = 4
	RCodeRefused        RCode = 5
)

func (r RCode) String() string {
	switch r {
	case RCodeSuccess:
		return "NOERROR"
	case RCodeFormatError:
		return "FORMERR"
	case RCodeServerFailure:
		return "SERVFAIL"
	case RCodeNameError:
		return "NXDOMAIN"
	case RCodeNotImplemented:
		return "NOTIMP"
	case RCodeRefused:
		return "REFUSED"
	default:
		return fmt.Sprintf("RCODE%d", uint8(r))
	}
}
