package message

import (
	"encoding/binary"
	"fmt"
)

// decoder はバイト列を先頭から読み進めるカーソルを保持する。
// 名前圧縮ポインタ(RFC1035 4.1.4)がバッファ内の任意の位置を後方参照するため、
// 全体のバッファと現在位置を保持する構造にしている。
type decoder struct {
	buf []byte
	pos int
}

func newDecoder(buf []byte) *decoder {
	return &decoder{buf: buf}
}

func (d *decoder) readUint8() (uint8, error) {
	if d.pos+1 > len(d.buf) {
		return 0, fmt.Errorf("unexpected end of buffer reading uint8 at offset %d", d.pos)
	}
	v := d.buf[d.pos]
	d.pos++
	return v, nil
}

func (d *decoder) readUint16() (uint16, error) {
	if d.pos+2 > len(d.buf) {
		return 0, fmt.Errorf("unexpected end of buffer reading uint16 at offset %d", d.pos)
	}
	v := binary.BigEndian.Uint16(d.buf[d.pos:])
	d.pos += 2
	return v, nil
}

func (d *decoder) readUint32() (uint32, error) {
	if d.pos+4 > len(d.buf) {
		return 0, fmt.Errorf("unexpected end of buffer reading uint32 at offset %d", d.pos)
	}
	v := binary.BigEndian.Uint32(d.buf[d.pos:])
	d.pos += 4
	return v, nil
}

// readBytes は次のn byteを返す。返されるスライスはdの内部バッファを直接指すため、
// 呼び出し側が保持する場合はコピーすること。
func (d *decoder) readBytes(n int) ([]byte, error) {
	if n < 0 || d.pos+n > len(d.buf) {
		return nil, fmt.Errorf("unexpected end of buffer reading %d bytes at offset %d", n, d.pos)
	}
	v := d.buf[d.pos : d.pos+n]
	d.pos += n
	return v, nil
}

// readCharacterString はPascal文字列形式(1byte長プレフィックス)を読む(RFC1035 3.3)。
func (d *decoder) readCharacterString() (string, error) {
	length, err := d.readUint8()
	if err != nil {
		return "", fmt.Errorf("character-string: read length: %w", err)
	}

	b, err := d.readBytes(int(length))
	if err != nil {
		return "", fmt.Errorf("character-string: read data: %w", err)
	}

	return string(b), nil
}
