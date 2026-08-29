package zone

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/MASAKi-cell/dns/message"
)

// Parse はゾーンファイルをパースしてZoneを返す
func Parse(r io.Reader) (*Zone, error) {
	p := &parser{
		scanner: bufio.NewScanner(r),
	}
	return p.parse()
}

// parser はゾーンファイルのパーサー
type parser struct {
	scanner      *bufio.Scanner
	origin       string
	defaultTTL   uint32
	lastName     string // 前の行の名前（名前省略時に使用）
	lineNum      int
	soaCount     int
	pendingLines []string // 複数行にまたがるレコード用
}

func (p *parser) parse() (*Zone, error) {
	var records []message.ResourceRecord

	for p.scanner.Scan() {
		p.lineNum++
		line := p.scanner.Text()

		// 複数行にまたがるレコード（括弧）の処理
		line = p.handleMultiline(line)
		if line == "" {
			continue
		}

		// コメントを除去
		line = p.stripComment(line)
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		// ディレクティブの処理
		if strings.HasPrefix(line, "$") {
			if err := p.parseDirective(line); err != nil {
				return nil, fmt.Errorf("line %d: %w", p.lineNum, err)
			}
			continue
		}

		// レコード行のパース
		rr, err := p.parseRecord(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", p.lineNum, err)
		}
		if rr != nil {
			if rr.Type == message.TypeSOA {
				p.soaCount++
			}
			records = append(records, *rr)
		}
	}

	if err := p.scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan error: %w", err)
	}

	// 検証
	if p.origin == "" {
		return nil, ErrMissingOrigin
	}
	if p.soaCount == 0 {
		return nil, ErrMissingSOA
	}
	if p.soaCount > 1 {
		return nil, ErrMultipleSOA
	}

	// Zone構築
	zone := NewZone(p.origin, p.defaultTTL)
	for _, rr := range records {
		zone.AddRecord(rr)
	}

	return zone, nil
}

// handleMultiline は複数行にまたがるレコード（括弧）を処理する
func (p *parser) handleMultiline(line string) string {
	// 各行のコメントを先に除去
	line = p.stripComment(line)

	// 既存の保留行があれば追加
	if len(p.pendingLines) > 0 {
		p.pendingLines = append(p.pendingLines, strings.TrimSpace(line))

		// 閉じ括弧があるか確認
		combined := strings.Join(p.pendingLines, " ")
		if strings.Contains(combined, ")") {
			p.pendingLines = nil
			return combined
		}
		return ""
	}

	// 開き括弧があるが閉じ括弧がない場合
	if strings.Contains(line, "(") && !strings.Contains(line, ")") {
		p.pendingLines = append(p.pendingLines, strings.TrimSpace(line))
		return ""
	}

	return line
}

// stripComment はコメント（;以降）を除去する
// ただしクォート内のセミコロンは除去しない
func (p *parser) stripComment(line string) string {
	inQuote := false
	for i, c := range line {
		if c == '"' {
			inQuote = !inQuote
		} else if c == ';' && !inQuote {
			return line[:i]
		}
	}
	return line
}

// parseDirective はディレクティブを処理する
func (p *parser) parseDirective(line string) error {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return fmt.Errorf("invalid directive: %s", line)
	}

	switch strings.ToUpper(fields[0]) {
	case "$ORIGIN":
		origin := fields[1]
		if !strings.HasSuffix(origin, ".") {
			origin = origin + "."
		}
		p.origin = origin
	case "$TTL":
		ttl, err := p.parseTTL(fields[1])
		if err != nil {
			return fmt.Errorf("invalid $TTL: %w", err)
		}
		p.defaultTTL = ttl
	default:
		// 未対応のディレクティブは無視
	}

	return nil
}

// parseRecord はレコード行をパースする
func (p *parser) parseRecord(line string) (*message.ResourceRecord, error) {
	tokens := p.tokenize(line)
	if len(tokens) == 0 {
		return nil, nil
	}

	var (
		name  string
		ttl   uint32
		class = message.ClassIN
		typ   message.Type
		rdata []string
	)

	idx := 0

	// 名前の解析（空白で始まる場合は前の名前を継続）
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		name = p.lastName
	} else {
		name = tokens[idx]
		idx++
	}

	// 名前の正規化
	name = p.normalizeName(name)
	p.lastName = name

	// TTL/CLASS/TYPEの解析（順不同で出現しうる）
	hasTTL := false
	hasClass := false

	for idx < len(tokens) {
		token := strings.ToUpper(tokens[idx])

		// TTLかどうか
		if !hasTTL {
			if ttlVal, err := p.parseTTL(tokens[idx]); err == nil {
				ttl = ttlVal
				hasTTL = true
				idx++
				continue
			}
		}

		// CLASSかどうか
		if !hasClass {
			if c, ok := parseClass(token); ok {
				class = c
				hasClass = true
				idx++
				continue
			}
		}

		// TYPEかどうか
		if t, ok := parseType(token); ok {
			typ = t
			idx++
			rdata = tokens[idx:]
			break
		}

		idx++
	}

	// デフォルトTTL
	if !hasTTL {
		ttl = p.defaultTTL
	}

	// TYPEが見つからない場合
	if typ == 0 {
		return nil, fmt.Errorf("missing record type")
	}

	// RDATAのパース
	rdataVal, err := p.parseRData(typ, rdata)
	if err != nil {
		return nil, fmt.Errorf("invalid RDATA for %s: %w", typ, err)
	}

	return &message.ResourceRecord{
		Name:  message.Name(name),
		Type:  typ,
		Class: class,
		TTL:   ttl,
		RData: rdataVal,
	}, nil
}

// tokenize は行をトークンに分割する（クォートを考慮）
func (p *parser) tokenize(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false

	for _, c := range line {
		switch {
		case c == '"':
			inQuote = !inQuote
			current.WriteRune(c)
		case (c == ' ' || c == '\t') && !inQuote:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		case c == '(' || c == ')':
			// 括弧は無視
			continue
		default:
			current.WriteRune(c)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// normalizeName は名前を正規化する
func (p *parser) normalizeName(name string) string {
	// @はオリジンに置換
	if name == "@" {
		return p.origin
	}

	// FQDNでなければオリジンを付加
	if !strings.HasSuffix(name, ".") {
		return name + "." + p.origin
	}

	return name
}

// parseTTL はTTL文字列をパースする
func (p *parser) parseTTL(s string) (uint32, error) {
	// 数値のみの場合
	if val, err := strconv.ParseUint(s, 10, 32); err == nil {
		return uint32(val), nil
	}

	// 単位付き（1h, 1d など）の場合
	s = strings.ToLower(s)
	var total uint64
	var current uint64

	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			current = current*10 + uint64(c-'0')
		case c == 's':
			total += current
			current = 0
		case c == 'm':
			total += current * 60
			current = 0
		case c == 'h':
			total += current * 3600
			current = 0
		case c == 'd':
			total += current * 86400
			current = 0
		case c == 'w':
			total += current * 604800
			current = 0
		default:
			return 0, fmt.Errorf("invalid TTL character: %c", c)
		}
	}
	total += current

	if total > 0xFFFFFFFF {
		return 0, fmt.Errorf("TTL overflow")
	}

	return uint32(total), nil
}

// parseClass はCLASS文字列をパースする
func parseClass(s string) (message.Class, bool) {
	switch s {
	case "IN":
		return message.ClassIN, true
	case "CS":
		return message.ClassCS, true
	case "CH":
		return message.ClassCH, true
	case "HS":
		return message.ClassHS, true
	default:
		return 0, false
	}
}

// parseType はTYPE文字列をパースする
func parseType(s string) (message.Type, bool) {
	switch s {
	case "A":
		return message.TypeA, true
	case "AAAA":
		return message.TypeAAAA, true
	case "NS":
		return message.TypeNS, true
	case "CNAME":
		return message.TypeCNAME, true
	case "SOA":
		return message.TypeSOA, true
	case "MX":
		return message.TypeMX, true
	case "TXT":
		return message.TypeTXT, true
	default:
		return 0, false
	}
}

// parseRData はRDATAをパースする
func (p *parser) parseRData(typ message.Type, tokens []string) (message.RData, error) {
	switch typ {
	case message.TypeA:
		return p.parseAData(tokens)
	case message.TypeAAAA:
		return p.parseAAAAData(tokens)
	case message.TypeNS:
		return p.parseNSData(tokens)
	case message.TypeCNAME:
		return p.parseCNAMEData(tokens)
	case message.TypeSOA:
		return p.parseSOAData(tokens)
	case message.TypeMX:
		return p.parseMXData(tokens)
	case message.TypeTXT:
		return p.parseTXTData(tokens)
	default:
		return nil, fmt.Errorf("unsupported record type: %s", typ)
	}
}

func (p *parser) parseAData(tokens []string) (message.RData, error) {
	if len(tokens) < 1 {
		return nil, fmt.Errorf("missing IPv4 address")
	}

	ip := net.ParseIP(tokens[0])
	if ip == nil {
		return nil, fmt.Errorf("invalid IPv4 address: %s", tokens[0])
	}

	ip4 := ip.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("not an IPv4 address: %s", tokens[0])
	}

	return message.AData{Address: [4]byte(ip4)}, nil
}

func (p *parser) parseAAAAData(tokens []string) (message.RData, error) {
	if len(tokens) < 1 {
		return nil, fmt.Errorf("missing IPv6 address")
	}

	ip := net.ParseIP(tokens[0])
	if ip == nil {
		return nil, fmt.Errorf("invalid IPv6 address: %s", tokens[0])
	}

	ip6 := ip.To16()
	if ip6 == nil {
		return nil, fmt.Errorf("not an IPv6 address: %s", tokens[0])
	}

	return message.AAAAData{Address: [16]byte(ip6)}, nil
}

func (p *parser) parseNSData(tokens []string) (message.RData, error) {
	if len(tokens) < 1 {
		return nil, fmt.Errorf("missing nameserver")
	}

	name := p.normalizeName(tokens[0])
	return message.NSData{NSDName: message.Name(name)}, nil
}

func (p *parser) parseCNAMEData(tokens []string) (message.RData, error) {
	if len(tokens) < 1 {
		return nil, fmt.Errorf("missing canonical name")
	}

	name := p.normalizeName(tokens[0])
	return message.CNAMEData{CName: message.Name(name)}, nil
}

func (p *parser) parseSOAData(tokens []string) (message.RData, error) {
	if len(tokens) < 7 {
		return nil, fmt.Errorf("SOA requires 7 fields, got %d", len(tokens))
	}

	mname := p.normalizeName(tokens[0])
	rname := p.normalizeName(tokens[1])

	serial, err := strconv.ParseUint(tokens[2], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid serial: %w", err)
	}

	refresh, err := p.parseTTL(tokens[3])
	if err != nil {
		return nil, fmt.Errorf("invalid refresh: %w", err)
	}

	retry, err := p.parseTTL(tokens[4])
	if err != nil {
		return nil, fmt.Errorf("invalid retry: %w", err)
	}

	expire, err := p.parseTTL(tokens[5])
	if err != nil {
		return nil, fmt.Errorf("invalid expire: %w", err)
	}

	minimum, err := p.parseTTL(tokens[6])
	if err != nil {
		return nil, fmt.Errorf("invalid minimum: %w", err)
	}

	return message.SOAData{
		MName:   message.Name(mname),
		RName:   message.Name(rname),
		Serial:  uint32(serial),
		Refresh: refresh,
		Retry:   retry,
		Expire:  expire,
		Minimum: minimum,
	}, nil
}

func (p *parser) parseMXData(tokens []string) (message.RData, error) {
	if len(tokens) < 2 {
		return nil, fmt.Errorf("MX requires preference and exchange")
	}

	preference, err := strconv.ParseUint(tokens[0], 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid preference: %w", err)
	}

	exchange := p.normalizeName(tokens[1])

	return message.MXData{
		Preference: uint16(preference),
		Exchange:   message.Name(exchange),
	}, nil
}

func (p *parser) parseTXTData(tokens []string) (message.RData, error) {
	var texts []string

	for _, token := range tokens {
		// クォートを除去
		text := strings.Trim(token, "\"")
		texts = append(texts, text)
	}

	return message.TXTData{Txt: texts}, nil
}
