## 概要

GoによるDNSプロトコルの実装。（DNSメッセージのパース/エンコード、DNSクライアント、再帰リゾルバ、権威サーバー）

> **⚠️ 注意**: 本リポジトリは**学習目的**で作成されています。本番環境やインターネットに公開するサーバーとしての使用は想定していません。セキュリティ対策（レートリミット、アクセス制御、DoS対策等）は実装されていないため、ローカル環境でのテスト・学習用途に限定してご利用ください。

### 主な機能

- **DNSメッセージ処理** - RFC 1035準拠のワイヤーフォーマットのエンコード/デコード
- **DNSクライアント** - UDPベースのDNSクエリ送信
- **再帰リゾルバ** - ルートサーバーから権威サーバーを辿る反復解決
- **権威サーバー** - ゾーンファイルを読み込んでDNSクエリに応答
- **ゾーンファイルパーサー** - RFC 1035形式のゾーンファイル読み込み

### サポートするレコードタイプ

- A (IPv4アドレス)
- AAAA (IPv6アドレス)
- NS (ネームサーバー)
- CNAME (正規名)
- MX (メール交換)
- TXT (テキスト)
- SOA (権威の開始)

## 使い方

### selfdig - DNS問い合わせツール

```bash
# デフォルト（Google DNS 8.8.8.8）でAレコードを問い合わせ
selfdig example.com

# サーバーを指定
selfdig @1.1.1.1 example.com

# レコードタイプを指定
selfdig example.com AAAA
selfdig example.com MX
selfdig example.com NS
```

### resolved - 再帰DNSリゾルバ

ルートサーバーから権威サーバーを辿って名前解決を行う再帰リゾルバ。

```bash
# デフォルトポート（5353）で起動
resolved

# ポートを指定
resolved -addr :5353

# selfdigで問い合わせ
selfdig @127.0.0.1:5353 example.com
```

### authd - 権威DNSサーバー

ゾーンファイルを読み込んで権威応答を返すDNSサーバー。

```bash
# ゾーンファイルを指定して起動
authd -zone example.zone -addr :5353
```

ゾーンファイルの例:

```
$ORIGIN example.com.
$TTL 3600

@       IN  SOA   ns1.example.com. admin.example.com. (
                  2024010101  ; Serial
                  3600        ; Refresh
                  1800        ; Retry
                  604800      ; Expire
                  86400       ; Minimum TTL
                  )

@       IN  NS    ns1.example.com.
@       IN  A     192.0.2.1
www     IN  A     192.0.2.2
mail    IN  MX    10 mail.example.com.
mail    IN  A     192.0.2.3
```

## プロジェクト構成

```
dns/
├── message/    # DNSメッセージのエンコード/デコード
├── client/     # DNSクライアント
├── server/     # UDPベースのDNSサーバー
├── resolver/   # 再帰リゾルバ（キャッシュ、ルートヒント含む）
├── zone/       # ゾーンファイルパーサー
├── cmd/
│   ├── selfdig/   # DNS問い合わせツール
│   ├── resolved/  # 再帰リゾルバデーモン
│   └── authd/     # 権威サーバーデーモン
└── docs/       # ドキュメント
```

## 参考文献

- [RFC 1035 - Domain Names - Implementation and Specification](https://datatracker.ietf.org/doc/html/rfc1035)
- [RFC 3596 - DNS Extensions to Support IP Version 6](https://datatracker.ietf.org/doc/html/rfc3596)
