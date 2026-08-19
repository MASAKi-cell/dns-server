# DNSプロトコル仕様

本ドキュメントは、本リポジトリが実装するDNSメッセージのワイヤーフォーマットについてまとめたものである。

## 参照RFC

| RFC | 内容 |
|---|---|
| RFC1034 | DOMAIN NAMES - CONCEPTS AND FACILITIES(概念定義) |
| RFC1035 | DOMAIN NAMES - IMPLEMENTATION AND SPECIFICATION(ワイヤーフォーマットの基本仕様。本ドキュメントの主な参照元) |
| RFC3596 | DNS Extensions to Support IP Version 6(AAAAレコード) |
| RFC2181 | Clarifications to the DNS Specification |

## 1. メッセージの全体構造

DNSメッセージは、クエリ・レスポンスを問わず同一フォーマットで、5つのセクションから構成される(RFC1035 4節)。

| セクション | 内容 | 件数を示すHeaderフィールド |
|---|---|---|
| Header | メッセージ全体の制御情報。12byte固定長 | - |
| Question | 問い合わせ内容(QNAME/QTYPE/QCLASS) | QDCOUNT |
| Answer | 質問に対する回答のResource Record群 | ANCOUNT |
| Authority | 権威サーバーを示すResource Record群 | NSCOUNT |
| Additional | 追加情報のResource Record群(グルーレコード等) | ARCOUNT |

クエリでは通常Questionのみが埋まりANCOUNT等は0、レスポンスではHeaderの各カウントに応じてAnswer/Authority/Additionalが続く。
カウントフィールドは実データの件数と必ず一致していなければならず、送信側はセクションの実長から算出して書き込む。

## 2. Headerセクション(12byte固定長)

RFC1035 4.1.1。全フィールドを合わせて12byte(96bit)。

```
                                    1  1  1  1  1  1
      0  1  2  3  4  5  6  7  8  9  0  1  2  3  4  5
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                      ID                       |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |QR|   Opcode  |AA|TC|RD|RA|   Z    |   RCODE   |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                    QDCOUNT                    |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                    ANCOUNT                    |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                    NSCOUNT                    |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                    ARCOUNT                    |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
```

上段の「1 1 1 1 1 1」と下段の「0〜9, 0〜5」を組み合わせて読むと、bit位置は0, 1, 2, ..., 9, 10, 11, 12, 13, 14, 15の16個(0-indexedで0〜15)になります。つまりこのルーラーは「この1行は16bit=2byte幅ですよ」という目盛りで、Headerの各行(ID行、flags行、QDCOUNT行、ANCOUNT行、NSCOUNT行、ARCOUNT行)の上に共通して使用されます。
12byteという合計は、16bit(2byte)の行が6段あることから来ています。

ID       → 2byte
flags     → 2byte  (QR/Opcode/AA/TC/RD/RA/Z/RCODEをまとめて1行)
QDCOUNT  → 2byte
ANCOUNT  → 2byte
NSCOUNT  → 2byte
ARCOUNT  → 2byte
-----------------
合計       12byte


| フィールド | サイズ | 意味 |
|---|---|---|
| ID | 16bit | クライアントが発行する識別子。レスポンスはクエリと同じIDを返す |
| QR | 1bit | 0=Query, 1=Response |
| Opcode | 4bit | クエリ種別。0=標準クエリ、1=IQUERY(廃止)、2=サーバーステータス要求 |
| AA | 1bit | Authoritative Answer。応答者がそのゾーンの権威サーバーか |
| TC | 1bit | TrunCation。UDPの512byte制限等でメッセージが切り詰められたことを示す。立っていればクライアントはTCPで再送すべき |
| RD | 1bit | Recursion Desired。クライアントが再帰的な名前解決を要求するか |
| RA | 1bit | Recursion Available。サーバーが再帰問い合わせに対応しているか |
| Z | 3bit | 予約領域。将来の拡張用で常に0でなければならない |
| RCODE | 4bit | 応答結果のエラーコード(下表) |
| QDCOUNT | 16bit | Questionセクションのエントリ数 |
| ANCOUNT | 16bit | Answerセクションのリソースレコード数 |
| NSCOUNT | 16bit | Authorityセクションのリソースレコード数 |
| ARCOUNT | 16bit | Additionalセクションのリソースレコード数 |

### RCODEの主な値

| 値 | 名称 | 意味 |
|---|---|---|
| 0 | NOERROR | 正常終了 |
| 1 | FORMERR | クエリの形式エラー |
| 2 | SERVFAIL | サーバー内部エラー |
| 3 | NXDOMAIN | 問い合わせたドメイン名が存在しない |
| 4 | NOTIMP | サーバーが未実装の機能を要求された |
| 5 | REFUSED | ポリシーにより応答を拒否 |

## 3. Questionセクション

RFC1035 4.1.2。QDCOUNT件分繰り返す。

```
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    /                     QNAME                     /
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                     QTYPE                     |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                     QCLASS                     |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
```

| フィールド | 意味 |
|---|---|
| QNAME | 問い合わせ対象のドメイン名(可変長。エンコード方式は4節を参照) |
| QTYPE | 問い合わせるレコード種別(TYPEと同じ値域に、AXFR等のQTYPE専用値を加えたもの) |
| QCLASS | 問い合わせるクラス(通常はIN) |

## 4. Resource Record共通フォーマット

RFC1035 4.1.3。Answer/Authority/Additionalの各セクションはこのフォーマットの繰り返し。

```
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    /                      NAME                     /
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                      TYPE                     |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                     CLASS                     |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                      TTL                       |
    |                                                |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                   RDLENGTH                    |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    /                     RDATA                     /
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
```

| フィールド | サイズ | 意味 |
|---|---|---|
| NAME | 可変長 | このレコードが属する所有者名 |
| TYPE | 16bit | レコード種別(下表) |
| CLASS | 16bit | クラス(通常はIN=1) |
| TTL | 32bit(符号あり扱いだが負値は不可) | このレコードをキャッシュしてよい秒数 |
| RDLENGTH | 16bit | 続くRDATAのbyte長 |
| RDATA | RDLENGTH byte | TYPEに応じた可変長データ(次節) |

RDLENGTHは実データ長から算出する値。受信側はRDLENGTHぶんだけ読み進めた位置が、そのTYPE用パーサが実際に消費したbyte数と一致することを検証すべきである(不一致はメッセージが壊れているか、パーサにバグがあることを意味する)。

## 5. ドメイン名のエンコーディング

RFC1035 3.1。ドメイン名はラベルの列であり、各ラベルは「長さ(1byte) + ラベル本体」の形式で連続してエンコードされ、末尾は長さ0の1byte(ルートラベル)で終端する。

```
www.example.com. →
  3 'w' 'w' 'w'  7 'e' 'x' 'a' 'm' 'p' 'l' 'e'  3 'c' 'o' 'm'  0
```

- 1ラベルは最大63byte(長さbyteの上位2bitは圧縮ポインタ用に予約されているため、ラベル長として表現できるのは0〜63)
- 名前全体は長さbyte・終端の0byteを含めて最大255byte
- ルート(`.`)は長さ0の1byteのみで表現される

## 6. 名前圧縮(Message Compression)

RFC1035 4.1.4。同じドメイン名がメッセージ中に繰り返し現れる場合(例: QuestionのQNAMEと
AnswerのNAME)、2回目以降は全体を書かず、既出の名前(の一部)を指す**ポインタ**で置き換えて
メッセージサイズを削減できる。

ポインタは2byteで、先頭2bitが`11`であることで通常のラベル長(先頭2bitは`00`)と区別する。
残り14bitがメッセージ先頭からのオフセットを表す。

```
     1  1
     5  4  13 12 11 10 9  8  7  6  5  4  3  2  1  0
   +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
   | 1  1|                OFFSET                   |
   +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
```

- ポインタは必ず**後方(より小さいオフセット)** を指す。前方参照や自己参照は不正なメッセージとして拒否しなければならない(無限ループを防ぐため)
- ラベル列の途中からポインタに切り替わってもよい(例:`4 'm' 'a' 'i' 'l' <ポインタ>` = "mail." + ポインタ先の名前)
- デコーダはポインタを何回でも追従しうるため、実装は最大追従回数などでループを防御する必要がある

## 7. RDATAのフォーマット(TYPE別)

RFC1035 3.3、AAAAのみRFC3596。

| TYPE | 値 | RDATAの内容 |
|---|---|---|
| A | 1 | IPv4アドレス。4byte |
| NS | 2 | ネームサーバー名(NSDNAME)。ドメイン名1つ |
| CNAME | 5 | 正規名(CNAME)。ドメイン名1つ |
| SOA | 6 | MNAME, RNAME(ドメイン名) + SERIAL, REFRESH, RETRY, EXPIRE, MINIMUM(各32bit) |
| MX | 15 | PREFERENCE(16bit) + EXCHANGE(ドメイン名) |
| TXT | 16 | 1つ以上のcharacter-string(下記) |
| AAAA | 28 | IPv6アドレス。16byte(RFC3596) |

### character-string

RFC1035 3.3。TXTレコードなどで使われる、長さ1byteプレフィックス付きの可変長文字列(Pascal文字列形式)。
「長さ(1byte, 0〜255) + 本体」の形式で、ドメイン名のラベルとは異なり圧縮ポインタの対象にはならない。
TXTのRDATAはRDLENGTHが尽きるまでcharacter-stringを繰り返し読むことで、複数文字列を1レコードに格納できる。

### SOAレコードの各フィールド

| フィールド | 意味 |
|---|---|
| MNAME | このゾーンのマスターサーバー名 |
| RNAME | ゾーン管理者のメールアドレス(`@`を`.`に置き換えた形式) |
| SERIAL | ゾーンのバージョン番号。セカンダリはこれで更新を検知する |
| REFRESH | セカンダリがマスターに更新確認しに行く間隔(秒) |
| RETRY | REFRESHに失敗した際の再試行間隔(秒) |
| EXPIRE | この期間更新できなければセカンダリはゾーンを権威なしとみなす(秒) |
| MINIMUM | ネガティブキャッシュ(RFC2308)のTTLとして扱われる(RFC2181以降の解釈) |

## 8. TYPE/CLASSの値

| TYPE名 | 値 |
|---|---|
| A | 1 |
| NS | 2 |
| CNAME | 5 |
| SOA | 6 |
| MX | 15 |
| TXT | 16 |
| AAAA | 28 |

| CLASS名 | 値 | 備考 |
|---|---|---|
| IN | 1 | インターネット。実運用で使うのはほぼこれのみ |
| CS | 2 | CSNETクラス。現在は廃止 |
| CH | 3 | Chaosnetクラス |
| HS | 4 | Hesiod。MIT Project Athenaのネームサービス用 |

## 9. トランスポートとメッセージサイズ

- **UDP**: 512byteを超えるメッセージは切り詰められ、HeaderのTCビットが立てられる
  (EDNS0(RFC6891)により実際にはより大きなUDPペイロードが使われることも多いが、
  本リポジトリでは未対応)
- **TCP**: メッセージの前に2byteのビッグエンディアン長プレフィックスを付与して送る
  (RFC1035 4.2.2)。TCビットが立ったレスポンスを受け取ったクライアントはTCPで再送する

## 10. digコマンドで実際のDNSメッセージを見る

ここまでの仕様が実際のDNSメッセージでどう現れるかは、`dig`コマンドで手元から確認できる。
`dig`はDNSサーバーにクエリを送り、返ってきたメッセージを人間が読みやすい形式で表示する
だけの読み取り専用ツールで、これまでの節で説明したHeader/Question/Answer/Authority/
Additionalの各セクションが、ほぼそのままの構成で出力に現れる。

```
$ dig +noedns google.com A

; <<>> DiG 9.10.6 <<>> +noedns google.com A
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 26748
;; flags: qr rd ra; QUERY: 1, ANSWER: 6, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;google.com.			IN	A

;; ANSWER SECTION:
google.com.		83	IN	A	172.217.209.139
google.com.		83	IN	A	172.217.209.102
google.com.		83	IN	A	172.217.209.101
google.com.		83	IN	A	172.217.209.138
google.com.		83	IN	A	172.217.209.100
google.com.		83	IN	A	172.217.209.113

;; Query time: 7 msec
;; SERVER: ...#53(...)
;; WHEN: Wed Aug 19 09:39:24 JST 2026
;; MSG SIZE  rcvd: 124
```

### 出力とワイヤーフォーマットの対応

| dig出力の行 | 対応するワイヤーフォーマット | 参照節 |
|---|---|---|
| `->>HEADER<<- opcode: QUERY, status: NOERROR, id: 26748` | HeaderのOpcodeフィールド・RCODEフィールド・ID | 2節 |
| `flags: qr rd ra` | Headerのフラグ群(`QR`/`RD`/`RA`が立っている。立っていないフラグは表示されない) | 2節 |
| `QUERY: 1, ANSWER: 6, AUTHORITY: 0, ADDITIONAL: 0` | QDCOUNT/ANCOUNT/NSCOUNT/ARCOUNT | 2節 |
| `;; QUESTION SECTION:` 以下 | Questionセクション(QNAME, QCLASS, QTYPE の順で表示) | 3節 |
| `;; ANSWER SECTION:` 以下 | Answerセクション(NAME, TTL, CLASS, TYPE, RDATA の順で表示) | 4, 7節 |
| `;; AUTHORITY SECTION:` (該当時) | Authorityセクション | 1, 4節 |
| `;; ADDITIONAL SECTION:` (該当時) | Additionalセクション | 1, 4節 |
| `;; MSG SIZE  rcvd:` | 受信したメッセージ全体のbyte数 | 1節 |

`;`で始まる行はコメント(dig自身が付与した注釈)であり、DNSメッセージそのものの一部ではない。
実際のリソースレコードの行は`NAME TTL CLASS TYPE RDATA`の順で1レコード1行に整形されている。
これは4節のResource Record共通フォーマットの各フィールドがそのまま列挙されたもので、
`google.com. 83 IN A 172.217.209.139`であれば

- NAME: `google.com.`
- TTL: `83`(秒。このレコードをキャッシュしてよい残り秒数)
- CLASS: `IN`
- TYPE: `A`
- RDATA: `172.217.209.139`

と読み替えられる。

### flagsの読み方

`flags:`の行には、Headerの1bitフラグのうち**立っているものだけ**が略称で列挙される
(立っていないフラグは表示されない)。

| 略称 | 対応するHeaderフィールド |
|---|---|
| `qr` | QR(このメッセージがResponseであることを示す。クエリ側の表示には出ない) |
| `aa` | AA(応答者がそのゾーンの権威サーバー) |
| `tc` | TC(メッセージが切り詰められた) |
| `rd` | RD(再帰的な名前解決を要求) |
| `ra` | RA(サーバーが再帰問い合わせに対応) |

### 他のTYPEやRCODEの例

**NSレコード**(Additionalセクションにグルーレコードが付く例):

```
$ dig +noedns google.com NS
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 15653
;; flags: qr rd ra; QUERY: 1, ANSWER: 4, AUTHORITY: 0, ADDITIONAL: 8

;; ANSWER SECTION:
google.com.		21906	IN	NS	ns1.google.com.
...
;; ADDITIONAL SECTION:
ns1.google.com.		40589	IN	A	216.239.32.10
ns1.google.com.		40589	IN	AAAA	2001:4860:4802:32::a
...
```

NSレコードのRDATA(`ns1.google.com.`)はネームサーバーの名前でありIPアドレスではないため、
そのままでは名前解決の役に立たない(いわゆる鶏と卵の問題)。そこでサーバーは、そのNSレコードに
対応するA/AAAAレコードをAdditionalセクションに**グルーレコード**として付加している。

**SOAレコード**(5節の各フィールドがそのまま並ぶ):

```
$ dig +noedns google.com SOA
;; ANSWER SECTION:
google.com.	45	IN	SOA	ns1.google.com. dns-admin.google.com. 965863404 900 900 1800 60
```

RDATAの並びはMNAME, RNAME, SERIAL, REFRESH, RETRY, EXPIRE, MINIMUMの順(7節参照)。
`dns-admin.google.com.`はRNAME、つまりゾーン管理者のメールアドレス
(`dns-admin@google.com`の`@`を`.`に置き換えた表記)である。

**存在しないドメイン(NXDOMAIN)の例**:

```
$ dig +noedns thisdomaindoesnotexist12345.com A
;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: 21583
;; flags: qr rd ra; QUERY: 1, ANSWER: 0, AUTHORITY: 1, ADDITIONAL: 0

;; AUTHORITY SECTION:
com.	900	IN	SOA	a.gtld-servers.net. nstld.verisign-grs.com. 1787099934 1800 900 604800 900
```

`status: NXDOMAIN`はHeaderのRCODEが3であることを示す(2節のRCODE表を参照)。ANSWERは0件だが、
AUTHORITYセクションに親ゾーン(`com.`)のSOAレコードが1件返っている。これはネガティブキャッシュ
(RFC2308)のためで、SOAのMINIMUMフィールド(5節)がこの否定応答をキャッシュしてよい秒数として
使われる。

### よく使う便利なオプション

| オプション | 効果 |
|---|---|
| `+noedns` | EDNS0のOPT疑似レコードを付けずにクエリを送る。ADDITIONALセクションにOPTが混ざらないため、本リポジトリの`message`パッケージが扱う範囲(EDNS0未対応)とメッセージ構造を素直に対応させやすい |
| `+short` | RDATAだけを簡潔に表示する |
| `+tcp` | UDPではなくTCP経由で問い合わせる(9節の2byte長プレフィックスの動作を伴う) |
| `+trace` | ルートサーバーから反復的に権威サーバーを辿る過程を表示する(`resolver`パッケージが将来行う処理のイメージに近い) |

## 11. 本リポジトリでの実装範囲

現時点で実装済みなのは、上記フォーマットのGoによる表現とエンコード/デコード
(`message`パッケージ)のみ。UDP/TCPでの送受信、ゾーンファイルの読み込み、再帰解決といった
上位の処理は未実装(`client`/`zone`/`server`/`resolver`/`cmd`)。実装の詳細は
[message-implementation.md](./message-implementation.md)を参照。
