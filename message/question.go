package message

import (
	"encoding/binary"
	"fmt"
)

// Question はDNSメッセージのQuestionセクションの1エントリを表す(RFC1035 4.1.2)。
type Question struct {
	Name  Name
	Type  Type
	Class Class
}

// marshal はQuestionをエンコードし、bufに追記して返す。
func (q Question) marshal(buf []byte) ([]byte, error) {
	buf, err := q.Name.marshal(buf)
	if err != nil {
		return nil, fmt.Errorf("question: marshal name: %w", err)
	}

	buf = binary.BigEndian.AppendUint16(buf, uint16(q.Type))
	buf = binary.BigEndian.AppendUint16(buf, uint16(q.Class))

	return buf, nil
}

// readQuestion はQuestionセクションの1エントリをデコードする。
func (d *decoder) readQuestion() (Question, error) {
	name, err := d.readName()
	if err != nil {
		return Question{}, fmt.Errorf("question: read name: %w", err)
	}

	typ, err := d.readUint16()
	if err != nil {
		return Question{}, fmt.Errorf("question: read type: %w", err)
	}

	class, err := d.readUint16()
	if err != nil {
		return Question{}, fmt.Errorf("question: read class: %w", err)
	}

	return Question{Name: name, Type: Type(typ), Class: Class(class)}, nil
}
