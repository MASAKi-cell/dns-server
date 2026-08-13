package message

import (
	"encoding/binary"
	"fmt"
	"slices"
	"strings"
)

const (
	maxLabelLength         = 63   // RFC1035 3.1: ラベルは63byteまで
	maxNameLength          = 255  // RFC1035 3.1: 名前全体は255byteまで
	compressionPointerMask = 0xC0 // 圧縮ポインタを示す先頭2bit(0b11)のマスク(RFC1035 4.1.4)。
	maxCompressionJumps    = 128  // 圧縮ポインタの追従回数の上限。255byteの名前でもラベル数は127個程度に収まるため、十分な余裕を持たせている。
)

// Name はDNSドメイン名を表す。ラベルをドットで連結した完全修飾形式
// (例: "www.example.com.")で保持し、ルートは "." とする。
// RFC1035の慣習に従ってドット付きの形式で保持される設計。
type Name string

// labels は名前をラベルのスライスに分解する（ラベルをドット区切りで保持）
// "www.example.com."をラベルのスライス（["www", "example", "com"]）に分解する処理
func (n Name) labels() ([]string, error) {
	trimmed := strings.TrimSuffix(string(n), ".")
	if trimmed == "" {
		return []string{}, nil
	}

	labels := strings.Split(trimmed, ".")
	if slices.Contains(labels, "") {
		return nil, fmt.Errorf("name %q contains an empty label", n)
	}

	return labels, nil
}

// marshal はラベル長プレフィックス方式でエンコードし、bufに追記して返す。
// 名前圧縮ポインタの書き込みは対象外(常にフルスペルで書き込む)。
func (n Name) marshal(buf []byte) ([]byte, error) {
	labels, err := n.labels()
	if err != nil {
		return nil, err
	}

	total := 1 // 終端の0byte
	for _, label := range labels {
		total += 1 + len(label)
	}
	if total > maxNameLength {
		return nil, fmt.Errorf("name %q exceeds %d bytes", n, maxNameLength)
	}

	for _, label := range labels {
		if len(label) > maxLabelLength {
			return nil, fmt.Errorf("name %q: label %q exceeds %d bytes", n, label, maxLabelLength)
		}
		buf = append(buf, byte(len(label)))
		buf = append(buf, label...)
	}
	buf = append(buf, 0)

	return buf, nil
}

// readName はラベル長プレフィックス方式の名前をデコードする。
// 圧縮ポインタ(先頭2bitが0b11)を追従するが、dの現在位置(d.pos)は
// 「圧縮ポインタも含めて名前を読み終えた直後」まで進める。ポインタの飛び先の読み取りは
// d.posを動かさずローカルのカーソルで行う。
func (d *decoder) readName() (Name, error) {
	labels := []string{}
	cursor := d.pos
	jumped := false
	jumps := 0

	for {
		if cursor >= len(d.buf) {
			return "", fmt.Errorf("name: unexpected end of buffer at offset %d", cursor)
		}

		length := d.buf[cursor]

		switch {
		case length == 0:
			cursor++
			if !jumped {
				d.pos = cursor
			}
			return joinLabels(labels), nil

		case length&compressionPointerMask == compressionPointerMask:
			if cursor+2 > len(d.buf) {
				return "", fmt.Errorf("name: truncated compression pointer at offset %d", cursor)
			}

			jumps++
			if jumps > maxCompressionJumps {
				return "", fmt.Errorf("name: too many compression pointer jumps (possible loop)")
			}

			ptr := int(binary.BigEndian.Uint16(d.buf[cursor:cursor+2]) &^ (compressionPointerMask << 8))
			if ptr >= cursor {
				return "", fmt.Errorf("name: compression pointer at offset %d does not point backward", cursor)
			}

			if !jumped {
				d.pos = cursor + 2
				jumped = true
			}
			cursor = ptr

		case length&compressionPointerMask != 0:
			return "", fmt.Errorf("name: invalid label length byte 0x%02x at offset %d", length, cursor)

		default:
			cursor++
			if cursor+int(length) > len(d.buf) {
				return "", fmt.Errorf("name: label extends past end of buffer at offset %d", cursor)
			}
			labels = append(labels, string(d.buf[cursor:cursor+int(length)]))
			cursor += int(length)
		}
	}
}

func joinLabels(labels []string) Name {
	if len(labels) == 0 {
		return "."
	}
	return Name(strings.Join(labels, ".") + ".")
}
