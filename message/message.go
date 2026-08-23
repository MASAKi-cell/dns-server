package message

import "fmt"

// Message はDNSメッセージ全体を表す。
type Message struct {
	Header      Header
	Questions   []Question
	Answers     []ResourceRecord
	Authorities []ResourceRecord
	Additionals []ResourceRecord
}

// Marshal はMessageをワイヤーフォーマットのバイト列にエンコードする。
// Header.QDCount等のカウント系フィールドは各セクションの長さから上書きされる。
//
// 例：Questionが1件("www.example.com."のAレコード問い合わせ)のみで
// Answer/Authority/Additionalが0件の場合、返り値のbufはHeader(12byte)に
// Question分を連結した次のようなバイト列になる。
//
//	[ID(2)] [flags(2)] [QDCOUNT=1] [ANCOUNT=0] [NSCOUNT=0] [ARCOUNT=0]        … Header 12byte
//	[QNAME: 3 w w w 7 e x a m p l e 3 c o m 0] [QTYPE=1(A)] [QCLASS=1(IN)]    … Question
func (m Message) Marshal() ([]byte, error) {
	h := m.Header
	h.QDCount = uint16(len(m.Questions))
	h.ANCount = uint16(len(m.Answers))
	h.NSCount = uint16(len(m.Authorities))
	h.ARCount = uint16(len(m.Additionals))

	// DNSメッセージは、内容に関わらずheaderは必ず12byteでありその分を確保しておく
	buf := make([]byte, 0, headerSize)
	buf = h.marshal(buf)

	// Questions
	buf, err := marshalQuestions(buf, m.Questions)
	if err != nil {
		return nil, err
	}

	// Answers
	buf, err = marshalResourceRecords(buf, "answer", m.Answers)
	if err != nil {
		return nil, err
	}

	// Authorities
	buf, err = marshalResourceRecords(buf, "authority", m.Authorities)
	if err != nil {
		return nil, err
	}

	// Additionals
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
//
// 例：Marshalの例と対になる入力、つまりQuestion1件("www.example.com."の
// Aレコード問い合わせ)のみでAnswer/Authority/Additionalが0件のバイト列
//
//	[ID(2)] [flags(2)] [QDCOUNT=1] [ANCOUNT=0] [NSCOUNT=0] [ARCOUNT=0]        … Header 12byte
//	[QNAME: 3 w w w 7 e x a m p l e 3 c o m 0] [QTYPE=1(A)] [QCLASS=1(IN)]    … Question
//
// を渡すと、次のようなMessageが返る。
//
//	&Message{
//	    Header:    Header{QDCount: 1, ANCount: 0, NSCount: 0, ARCount: 0, ...},
//	    Questions: []Question{{Name: "www.example.com.", Type: TypeA, Class: ClassIN}},
//	    // Answers/Authorities/Additionalsは空スライス
//	}
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

	// 各セクションをmessageにまとめて返却する。
	return &Message{
		Header:      header,
		Questions:   questions,
		Answers:     answers,
		Authorities: authorities,
		Additionals: additionals,
	}, nil
}
