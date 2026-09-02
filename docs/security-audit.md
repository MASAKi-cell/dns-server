# セキュリティ監査レポート

本プロジェクトのセキュリティ監査結果と、DNS特有の攻撃手法、本番運用時に必要な対策をまとめる。

## 1. 監査サマリー

| 重要度 | 件数 | 対応 |
|--------|------|------|
| Critical | 0 | - |
| High | 4 | 本番運用時に対応 |
| Medium | 5 | 本番運用時に検討 |
| Low | 4 | 必要に応じて |

## 2. 実装できているセキュリティ対策

### 暗号論的乱数によるID生成

```go
// client/client.go:160-167
func generateID() (uint16, error) {
    var b [2]byte
    if _, err := rand.Read(b[:]); err != nil {
        return 0, err
    }
    return uint16(b[0])<<8 | uint16(b[1]), nil
}
```

`math/rand`ではなく`crypto/rand`を使用。Transaction IDの予測を困難にし、Kaminsky攻撃への基本的な耐性を確保。

### 圧縮ポインタ攻撃の防御

```go
// message/name.go:99-107
if jumps > maxCompressionJumps {
    return "", fmt.Errorf("too many compression pointer jumps")
}
if ptr >= cursor {
    return "", fmt.Errorf("compression pointer does not point backward")
}
```

- ジャンプ回数を128回に制限（無限ループ防止）
- 後方参照のみ許可（バッファ外読み取り防止）

### バッファ境界チェック

```go
// message/codec.go
func (d *decoder) readBytes(n int) ([]byte, error) {
    if d.pos+n > len(d.buf) {
        return nil, fmt.Errorf("unexpected end of buffer")
    }
    // ...
}
```

全ての読み取り操作でバッファ境界を検証。

### 再帰深度制限

```go
// resolver/resolver.go:16,60-62
const maxDepth = 16

if depth > maxDepth {
    return nil, fmt.Errorf("max recursion depth exceeded")
}
```

無限再帰によるスタックオーバーフローを防止。

### その他

- Transaction ID検証（レスポンスのID一致確認）
- グレースフルシャットダウン（WaitGroupで処理中リクエストを待機）
- 外部依存ゼロ（サプライチェーン攻撃リスクなし）
- CNAME追跡の深度制限（maxDepth=8）

## 3. 検出された脆弱性

### High（本番運用時は対応必須）

#### レート制限なし

**場所:** `server/server.go:67-87`

```go
for {
    n, addr, err := s.conn.ReadFrom(buf)
    // ...
    s.wg.Add(1)
    go s.handleRequest(req, addr)  // 無制限にgoroutine生成
}
```

**リスク:** 攻撃者が大量のDNSクエリを送信すると、goroutineが無制限に生成されメモリ枯渇。

**対策例:**
```go
// semaphoreによるgoroutine数制限
sem := make(chan struct{}, 1000)

select {
case sem <- struct{}{}:
    go func() {
        defer func() { <-sem }()
        s.handleRequest(req, addr)
    }()
default:
    // 制限超過時はドロップ
}
```

#### キャッシュサイズ無制限

**場所:** `resolver/cache.go:19-29`

```go
type Cache struct {
    mu      sync.RWMutex
    entries map[string]cacheEntry  // サイズ上限なし
}
```

**リスク:** 大量の異なるドメイン名をクエリされると、キャッシュエントリが無限に増加。

**対策例:**
```go
const maxCacheEntries = 10000

func (c *Cache) Set(records []message.ResourceRecord) {
    if len(c.entries) >= maxCacheEntries {
        c.evictOldest()  // LRUで古いエントリを削除
    }
    // ...
}
```

#### オープンリゾルバ

**場所:** `cmd/resolved/main.go`

**リスク:** 任意のクライアントからのクエリを無制限に処理。DNS増幅攻撃の踏み台として悪用される。

**対策例:**
```go
// 許可IPリストによるアクセス制御
type Server struct {
    AllowedIPs []net.IPNet
}

func (s *Server) isAllowed(addr net.Addr) bool {
    udpAddr := addr.(*net.UDPAddr)
    for _, allowed := range s.AllowedIPs {
        if allowed.Contains(udpAddr.IP) {
            return true
        }
    }
    return false
}
```

#### Zone構造体の並行アクセス

**場所:** `zone/zone.go`

**リスク:** ゾーンのホットリロード機能を追加した場合、データ競合が発生。

**対策例:**
```go
type Zone struct {
    mu      sync.RWMutex
    records map[string][]message.ResourceRecord
}

func (z *Zone) Lookup(...) {
    z.mu.RLock()
    defer z.mu.RUnlock()
    // ...
}
```

### Medium

| 脆弱性 | 場所 | 概要 |
|--------|------|------|
| Bailiwickチェックなし | resolver/resolver.go | 応答レコードがクエリ対象の権威範囲内か未検証 |
| SSRF対策なし | resolver/resolver.go | プライベートIPへのクエリ可能 |
| Headerカウント上限なし | message/message.go | QDCount=65535で大量メモリ確保 |
| CNAME再帰の別パス | resolver/resolver.go | extractNextServers内のResolve呼び出し |
| キャッシュCleanup未実行 | resolver/cache.go | 期限切れエントリが残り続ける |

## 4. DNS特有の攻撃手法

### DNS増幅攻撃（DDoS Amplification）

攻撃者が送信元IPを偽装してDNSクエリを送信し、応答を被害者に送りつける。

```
攻撃者 → [偽装IP: 被害者] → DNSサーバー
                              ↓
                           被害者 ← 大きな応答
```

- クエリ: 数十バイト
- 応答: 数百〜数千バイト（増幅率: 10〜100倍）

**対策:**
- Response Rate Limiting (RRL)
- オープンリゾルバとして公開しない
- ACLによるアクセス制限

### DNSキャッシュポイズニング

攻撃者がリゾルバのキャッシュに偽のレコードを注入。

```
1. 被害者がexample.comをクエリ
2. リゾルバが権威サーバーに問い合わせ
3. 攻撃者が権威サーバーより先に偽応答を送信
4. IDが一致すれば偽応答がキャッシュされる
```

**防御要素:**
- ランダムなTransaction ID（crypto/rand使用）
- ランダムなソースポート
- QNAME/QTYPE/QCLASSの検証
- DNSSEC（署名検証）

### Kaminsky攻撃

キャッシュポイズニングの発展形。存在しないサブドメインを大量にクエリし、NSレコードを汚染。

```
1. 攻撃者がrandom1.example.comをクエリ（存在しない）
2. 攻撃者が大量の偽応答を送信
   - Answer: random1.example.com → なし
   - Authority: example.com NS → evil.attacker.com
3. 成功すればexample.com全体を乗っ取り
```

**対策:**
- Bailiwickチェック（応答が権威範囲内か検証）
- DNSSEC

### SSRF via DNS

リゾルバを経由して内部ネットワークにアクセス。

```
1. 攻撃者が悪意のあるドメインをクエリ
2. 攻撃者の権威サーバーが内部IP（192.168.1.1）を返す
3. リゾルバが内部IPに接続を試みる
```

**対策:**
```go
func isPrivateIP(ip net.IP) bool {
    privateRanges := []string{
        "10.0.0.0/8",
        "172.16.0.0/12",
        "192.168.0.0/16",
        "127.0.0.0/8",
    }
    for _, cidr := range privateRanges {
        _, block, _ := net.ParseCIDR(cidr)
        if block.Contains(ip) {
            return true
        }
    }
    return false
}
```

### 圧縮ポインタ攻撃

DNSメッセージの名前圧縮機能を悪用し、無限ループやバッファオーバーリードを引き起こす。

```
悪意のあるパケット:
- 圧縮ポインタが自分自身を指す → 無限ループ
- 圧縮ポインタがバッファ外を指す → メモリ読み取り
```

**本プロジェクトの対策:**
- ジャンプ回数制限（maxCompressionJumps=128）
- 後方参照のみ許可（ptr < cursor）

## 5. 本番運用時のチェックリスト

### 必須

- [ ] レート制限の実装
- [ ] キャッシュサイズ制限
- [ ] ACL（アクセス制御リスト）
- [ ] プライベートIPフィルタリング
- [ ] Headerカウントの上限チェック
- [ ] バインドアドレスの明示的指定（0.0.0.0を避ける）

### 推奨

- [ ] Response Rate Limiting (RRL)
- [ ] DNSSEC検証
- [ ] DNS Cookies (RFC 7873)
- [ ] 構造化ログ
- [ ] メトリクス/監視（キャッシュヒット率、エラー率）
- [ ] TCPフォールバック

### 運用

- [ ] ファイアウォールでポート53を制限
- [ ] ログローテーション
- [ ] 定期的なセキュリティアップデート
- [ ] `go test -race` をCIに追加

## 6. 参考資料

- [RFC 5452 - DNS Resilience against Forged Answers](https://www.rfc-editor.org/rfc/rfc5452)
- [RFC 7873 - DNS Cookies](https://www.rfc-editor.org/rfc/rfc7873)
- [OWASP - DNS Security](https://cheatsheetseries.owasp.org/cheatsheets/DNS_Security_Cheat_Sheet.html)
- [Kaminsky Attack (2008)](https://en.wikipedia.org/wiki/DNS_spoofing#Kaminsky_attack)
