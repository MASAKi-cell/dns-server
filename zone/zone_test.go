package zone

import (
	"strings"
	"testing"

	"github.com/MASAKi-cell/dns/message"
)

func TestParse_BasicZone(t *testing.T) {
	zoneFile := `
$ORIGIN example.com.
$TTL 3600

@       IN  SOA   ns1.example.com. admin.example.com. (
                  2024010101  ; Serial
                  3600        ; Refresh
                  900         ; Retry
                  604800      ; Expire
                  86400       ; Minimum TTL
)

@       IN  NS    ns1.example.com.
@       IN  NS    ns2.example.com.
@       IN  A     192.0.2.1
www     IN  A     192.0.2.2
ns1     IN  A     192.0.2.10
ns2     IN  A     192.0.2.11
`

	z, err := Parse(strings.NewReader(zoneFile))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if z.Origin != "example.com." {
		t.Errorf("Origin = %q, want %q", z.Origin, "example.com.")
	}

	if z.TTL != 3600 {
		t.Errorf("TTL = %d, want %d", z.TTL, 3600)
	}

	// SOAレコードの確認
	soa := z.SOA()
	if soa == nil {
		t.Fatal("SOA record not found")
	}
	if soa.Type != message.TypeSOA {
		t.Errorf("SOA type = %v, want %v", soa.Type, message.TypeSOA)
	}

	// NSレコードの確認
	ns := z.NS()
	if len(ns) != 2 {
		t.Errorf("NS count = %d, want %d", len(ns), 2)
	}

	// Aレコードの確認
	a := z.LookupExact("example.com.", message.TypeA)
	if len(a) != 1 {
		t.Errorf("A record count for example.com = %d, want %d", len(a), 1)
	}

	www := z.LookupExact("www.example.com.", message.TypeA)
	if len(www) != 1 {
		t.Errorf("A record count for www.example.com = %d, want %d", len(www), 1)
	}
}

func TestParse_MXRecord(t *testing.T) {
	zoneFile := `
$ORIGIN example.com.
$TTL 3600

@   IN  SOA   ns1 admin 2024010101 3600 900 604800 86400
@   IN  MX    10 mail1
@   IN  MX    20 mail2.example.com.
`

	z, err := Parse(strings.NewReader(zoneFile))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	mx := z.LookupExact("example.com.", message.TypeMX)
	if len(mx) != 2 {
		t.Fatalf("MX record count = %d, want %d", len(mx), 2)
	}

	// 最初のMXレコード
	mxData, ok := mx[0].RData.(message.MXData)
	if !ok {
		t.Fatal("First MX RData is not MXData")
	}
	if mxData.Preference != 10 {
		t.Errorf("First MX preference = %d, want %d", mxData.Preference, 10)
	}
	if string(mxData.Exchange) != "mail1.example.com." {
		t.Errorf("First MX exchange = %q, want %q", mxData.Exchange, "mail1.example.com.")
	}
}

func TestParse_CNAMERecord(t *testing.T) {
	zoneFile := `
$ORIGIN example.com.
$TTL 3600

@     IN  SOA   ns1 admin 2024010101 3600 900 604800 86400
www   IN  A     192.0.2.1
ftp   IN  CNAME www
`

	z, err := Parse(strings.NewReader(zoneFile))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// CNAME追跡なしの検索
	cname := z.LookupExact("ftp.example.com.", message.TypeCNAME)
	if len(cname) != 1 {
		t.Fatalf("CNAME record count = %d, want %d", len(cname), 1)
	}

	// CNAME追跡ありの検索
	a := z.Lookup("ftp.example.com.", message.TypeA)
	if len(a) != 2 { // CNAME + A
		t.Errorf("Lookup with CNAME following returned %d records, want %d", len(a), 2)
	}
}

func TestParse_TXTRecord(t *testing.T) {
	zoneFile := `
$ORIGIN example.com.
$TTL 3600

@   IN  SOA   ns1 admin 2024010101 3600 900 604800 86400
@   IN  TXT   "v=spf1 include:_spf.example.com ~all"
`

	z, err := Parse(strings.NewReader(zoneFile))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	txt := z.LookupExact("example.com.", message.TypeTXT)
	if len(txt) != 1 {
		t.Fatalf("TXT record count = %d, want %d", len(txt), 1)
	}

	txtData, ok := txt[0].RData.(message.TXTData)
	if !ok {
		t.Fatal("TXT RData is not TXTData")
	}
	if len(txtData.Txt) != 1 {
		t.Errorf("TXT text count = %d, want %d", len(txtData.Txt), 1)
	}
	expected := "v=spf1 include:_spf.example.com ~all"
	if txtData.Txt[0] != expected {
		t.Errorf("TXT text = %q, want %q", txtData.Txt[0], expected)
	}
}

func TestParse_MissingOrigin(t *testing.T) {
	zoneFile := `
$TTL 3600
@   IN  SOA   ns1 admin 2024010101 3600 900 604800 86400
`

	_, err := Parse(strings.NewReader(zoneFile))
	if err == nil {
		t.Error("Expected error for missing $ORIGIN, got nil")
	}
}

func TestParse_MissingSOA(t *testing.T) {
	zoneFile := `
$ORIGIN example.com.
$TTL 3600
@   IN  A     192.0.2.1
`

	_, err := Parse(strings.NewReader(zoneFile))
	if err == nil {
		t.Error("Expected error for missing SOA, got nil")
	}
}

func TestParse_TTLWithUnits(t *testing.T) {
	zoneFile := `
$ORIGIN example.com.
$TTL 1h

@   IN  SOA   ns1 admin 2024010101 1h 15m 7d 1d
@   IN  A     192.0.2.1
`

	z, err := Parse(strings.NewReader(zoneFile))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if z.TTL != 3600 {
		t.Errorf("TTL = %d, want %d (1h = 3600)", z.TTL, 3600)
	}

	soa := z.SOA()
	if soa == nil {
		t.Fatal("SOA record not found")
	}

	soaData, ok := soa.RData.(message.SOAData)
	if !ok {
		t.Fatal("SOA RData is not SOAData")
	}

	if soaData.Refresh != 3600 {
		t.Errorf("SOA Refresh = %d, want %d", soaData.Refresh, 3600)
	}
	if soaData.Retry != 900 {
		t.Errorf("SOA Retry = %d, want %d", soaData.Retry, 900)
	}
	if soaData.Expire != 604800 {
		t.Errorf("SOA Expire = %d, want %d", soaData.Expire, 604800)
	}
}

func TestZone_IsAuthoritative(t *testing.T) {
	z := NewZone("example.com.", 3600)

	tests := []struct {
		name string
		want bool
	}{
		{"example.com.", true},
		{"www.example.com.", true},
		{"sub.www.example.com.", true},
		{"other.com.", false},
		{"notexample.com.", false},
	}

	for _, tt := range tests {
		got := z.IsAuthoritative(tt.name)
		if got != tt.want {
			t.Errorf("IsAuthoritative(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestZone_LookupCaseInsensitive(t *testing.T) {
	zoneFile := `
$ORIGIN example.com.
$TTL 3600

@   IN  SOA   ns1 admin 2024010101 3600 900 604800 86400
WWW IN  A     192.0.2.1
`

	z, err := Parse(strings.NewReader(zoneFile))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// 小文字で検索
	a := z.LookupExact("www.example.com.", message.TypeA)
	if len(a) != 1 {
		t.Errorf("Case insensitive lookup failed: got %d records, want 1", len(a))
	}

	// 大文字で検索
	a = z.LookupExact("WWW.EXAMPLE.COM.", message.TypeA)
	if len(a) != 1 {
		t.Errorf("Case insensitive lookup failed: got %d records, want 1", len(a))
	}
}
