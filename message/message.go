package message

import "fmt"

// Message はDNSメッセージ全体を表す(RFC1035 4節)。
type Message struct {
	Header      Header
	Questions   []Question
	Answers     []ResourceRecord
	Authorities []ResourceRecord
	Additionals []ResourceRecord
}

// Marshal はMessageをワイヤーフォーマットのバイト列にエンコードする。
// Header.QDCount等のカウント系フィールドは各セクションの長さから上書きされる。
func (m Message) Marshal() ([]byte, error) {
	h := m.Header
	h.QDCount = uint16(len(m.Questions))
	h.ANCount = uint16(len(m.Answers))
	h.NSCount = uint16(len(m.Authorities))
	h.ARCount = uint16(len(m.Additionals))

	buf := make([]byte, 0, headerSize)
	buf = h.marshal(buf)

	buf, err := marshalQuestions(buf, m.Questions)
	if err != nil {
		return nil, err
	}

	buf, err = marshalResourceRecords(buf, "answer", m.Answers)
	if err != nil {
		return nil, err
	}

	buf, err = marshalResourceRecords(buf, "authority", m.Authorities)
	if err != nil {
		return nil, err
	}

	buf, err = marshalResourceRecords(buf, "additional", m.Additionals)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func marshalQuestions(buf []byte, questions []Question) ([]byte, error) {
	for _, q := range questions {
		var err error
		buf, err = q.marshal(buf)
		if err != nil {
			return nil, fmt.Errorf("message: marshal question: %w", err)
		}
	}
	return buf, nil
}

func marshalResourceRecords(buf []byte, section string, rrs []ResourceRecord) ([]byte, error) {
	for _, rr := range rrs {
		var err error
		buf, err = rr.marshal(buf)
		if err != nil {
			return nil, fmt.Errorf("message: marshal %s: %w", section, err)
		}
	}
	return buf, nil
}

// Unmarshal はワイヤーフォーマットのバイト列をパースしてMessageを返す。
func Unmarshal(data []byte) (*Message, error) {
	d := newDecoder(data)

	header, err := d.readHeader()
	if err != nil {
		return nil, fmt.Errorf("message: read header: %w", err)
	}

	questions := make([]Question, 0, header.QDCount)
	for range header.QDCount {
		q, err := d.readQuestion()
		if err != nil {
			return nil, fmt.Errorf("message: read question: %w", err)
		}
		questions = append(questions, q)
	}

	answers, err := d.readResourceRecords(int(header.ANCount))
	if err != nil {
		return nil, fmt.Errorf("message: read answer: %w", err)
	}

	authorities, err := d.readResourceRecords(int(header.NSCount))
	if err != nil {
		return nil, fmt.Errorf("message: read authority: %w", err)
	}

	additionals, err := d.readResourceRecords(int(header.ARCount))
	if err != nil {
		return nil, fmt.Errorf("message: read additional: %w", err)
	}

	return &Message{
		Header:      header,
		Questions:   questions,
		Answers:     answers,
		Authorities: authorities,
		Additionals: additionals,
	}, nil
}
