// ゾーンファイルのパースとレコードの保持・検索を行う。
// 権威DNSサーバーの中核として、クエリに応じたレコードの検索を担う。
package zone

import (
	"strings"

	"github.com/MASAKi-cell/dns/message"
)

// ゾーン構造体
// records格納例：
//
//	records = {
//	    "example.com.": [
//	       {Type: SOA, RData: SOAData{...}},
//	       {Type: NS,  RData: NSData{NSDName: "ns1.example.com."}}
//	}
type Zone struct {
	Origin  string                              // ゾーンのオリジン（例: "example.com."）
	TTL     uint32                              // デフォルトTTL
	records map[string][]message.ResourceRecord //ドメイン名のレコード情報
}

// 新しいZoneを作成する
func NewZone(origin string, ttl uint32) *Zone {
	// 末尾に.がなければ、オリジンをFQDN形式に正規化
	if !strings.HasSuffix(origin, ".") {
		origin = origin + "."
	}
	return &Zone{
		Origin:  origin,
		TTL:     ttl,
		records: make(map[string][]message.ResourceRecord),
	}
}

// ゾーンにレコードを追加する
func (z *Zone) AddRecord(rr message.ResourceRecord) {
	name := strings.ToLower(string(rr.Name))
	z.records[name] = append(z.records[name], rr)
}

// 名前とタイプでレコードを検索する（CNAMEがあればその先も追跡する）
func (z *Zone) Lookup(name string, typ message.Type) []message.ResourceRecord {
	name = z.normalizeName(name)
	return z.lookupWithCNAME(name, typ, 0)
}

// CNAME追跡付きの検索
func (z *Zone) lookupWithCNAME(name string, typ message.Type, depth int) []message.ResourceRecord {
	const maxDepth = 8 // CNAME追跡の上限

	if depth > maxDepth {
		return nil
	}

	// 完全一致で検索
	records := z.LookupExact(name, typ)
	if len(records) > 0 {
		return records
	}

	// CNAMEを探す（要求されたタイプがCNAME以外の場合）
	if typ != message.TypeCNAME {
		cnames := z.LookupExact(name, message.TypeCNAME)
		if len(cnames) > 0 {
			// CNAMEの先を追跡
			result := make([]message.ResourceRecord, 0, len(cnames))
			result = append(result, cnames...)

			for _, cname := range cnames {
				if cnameData, ok := cname.RData.(message.CNAMEData); ok {
					target := string(cnameData.CName)
					result = append(result, z.lookupWithCNAME(target, typ, depth+1)...)
				}
			}
			return result
		}
	}

	return nil
}

// 完全一致でレコードを検索する（CNAME追跡なし）
func (z *Zone) LookupExact(name string, typ message.Type) []message.ResourceRecord {
	name = z.normalizeName(name)
	allRecords := z.records[name]

	var result []message.ResourceRecord
	for _, rr := range allRecords {
		if rr.Type == typ {
			result = append(result, rr)
		}
	}
	return result
}

// 指定した名前の全レコードを返す
func (z *Zone) LookupAll(name string) []message.ResourceRecord {
	name = z.normalizeName(name)
	return z.records[name]
}

// ゾーンのSOAレコードを返す
func (z *Zone) SOA() *message.ResourceRecord {
	records := z.LookupExact(z.Origin, message.TypeSOA)
	if len(records) > 0 {
		return &records[0]
	}
	return nil
}

// ゾーンのNSレコード群を返す
func (z *Zone) NS() []message.ResourceRecord {
	return z.LookupExact(z.Origin, message.TypeNS)
}

// 名前を正規化する（小文字化、FQDN化）
func (z *Zone) normalizeName(name string) string {
	name = strings.ToLower(name)
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}
	return name
}

// 指定した名前がこのゾーンの管轄かどうかを返す
func (z *Zone) IsAuthoritative(name string) bool {
	name = z.normalizeName(name)
	return name == z.Origin || strings.HasSuffix(name, "."+z.Origin)
}
