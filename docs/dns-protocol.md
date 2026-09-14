# DNSプロトコル仕様

本章ではDNSプロトコルの仕様について解説していきます。

## digコマンドの内容を確認する

まずは、digコマンドの内容を確認してみましょう。digコマンドはDNSサーバーにクエリを送り、返ってきたメッセージを人間が読みやすい形式で表示するだけの読み取り専用ツールです。例えば、`dig +noedns google.com A`を打ち込みと以下のような結果が返ってきます。

```bash
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
※ `;`で始まる行はコメント(digが付与した注釈)であり、DNSメッセージの一部ではありません。

- `opcode: QUERY`：標準的な問い合わせであることを示す(Opcode=0)
- `status: NOERROR`： エラーなく正常に処理されたことを示す(RCODE=0)。ドメインが存在しない場合は`NXDOMAIN`になる
`id: 26748`： クライアントとサーバーを紐付けるためのランダムなID
- `flags: qr rd ra`： Headerの1bitフラグのうち**立っているものだけ**が略称で列挙される(立っていないフラグは表示されない)。
  -  `qr`： QR(このメッセージがResponseであることを示す)
  -  `aa`： AA(応答者がそのゾーンの権威サーバーかどうか)
  -  `tc`： TC(メッセージが切り詰められたかどうか)
  -  `rd`： RD(再帰的な名前解決を要求かどうか)
  -  `ra`： RA(サーバーが再帰問い合わせに対応するかどうか)
- `QUERY: 1, ANSWER: 6, AUTHORITY: 0, ADDITIONAL: 0`： 質問1件・回答6件・権威情報0件・追加情報0件、という後続セクションの件数予告
- `;; QUESTION SECTION:`： Questionセクション。「google.comのAレコードを、クラスINで教えて」という、こちらが送った質問そのものを表している。
- `;; ANSWER SECTION:`： Answerセクション。NAME TTL CLASS TYPE RDATAの順で実際の回答が並ぶ。
- `;; AUTHORITY SECTION:`： 権威サーバーの情報が入る欄。件数が0のときは、そもそもこの行自体が出力されない
- `;; ADDITIONAL SECTION:`： Additionalセクション。補足的なレコードが入る欄。こちらも0件のときは非表示になる
- `;; MSG SIZE rcvd: 124`： 受信したDNSメッセージが124byteだったことを示す
- `ANSWER SECTION`： 回答セクション。`google.com. 83 IN A 172.217.209.139`は以下の意味になります。
  - NAME: `google.com.`
  - TTL: `83`(秒。このレコードをキャッシュしてよい残り秒数)
  - CLASS: `IN`
  - TYPE: `A`
  - RDATA: `172.217.209.139`

**`ANSWER SECTION:`で複数行返信が返ってきている理由**
1回の問い合わせに対して、`ANSWER SECTION:`で複数行、返信が返ってきています。

```bash
;; ANSWER SECTION:
google.com.		83	IN	A	172.217.209.139
google.com.		83	IN	A	172.217.209.102
google.com.		83	IN	A	172.217.209.101
google.com.		83	IN	A	172.217.209.138
google.com.		83	IN	A	172.217.209.100
google.com.		83	IN	A	172.217.209.113
```
これは複数のサーバーに問い合わせて回答を集めたのではなく、権威サーバー側が、自分のゾーンファイルに`google.com`のAレコードとしてもともと6つのIPアドレスを登録しており、1つの応答メッセージの中にまとめて返却していることを表しています(Answerセクションに6件のRRが並ぶ)。クライアント側(ブラウザなど)は、返ってきた6つのIPの中からどれか1つを選んで接続します。Googleのように大量のアクセスを捌く必要があるサービスは、権威サーバーが同じドメイン名に対して複数台のサーバー(のIP)を用意しておくことで、その後の接続先選択で複数台に負荷分散させることができます(これを**DNSラウンドロビン**と呼びます)。実際にどれを選んで接続するかはクライアント側(ブラウザやOS)に委ねられます。

```bash
[実際のインフラ]              [DNSゾーンファイルへの登録]
サーバー1: 172.217.209.139  →  google.com. 300 IN A 172.217.209.139
サーバー2: 172.217.209.102  →  google.com. 300 IN A 172.217.209.102
サーバー3: 172.217.209.101  →  google.com. 300 IN A 172.217.209.101
サーバー4: 172.217.209.138  →  google.com. 300 IN A 172.217.209.138
サーバー5: 172.217.209.100  →  google.com. 300 IN A 172.217.209.100
サーバー6: 172.217.209.113  →  google.com. 300 IN A 172.217.209.113
```

## メッセージの全体構造
DNSメッセージは、クエリ・レスポンスを問わず同一フォーマットで、5つのセクションから構成されます。DNSがQRビット1つで済ませることができるのは、「質問も回答も同じ語彙(名前+タイプ+クラス)で表現できる」というドメイン特性を活かした設計になります。
HTTPの場合は、クエストはメソッド + パス + バージョン(例: `GET /index.html HTTP/1.1`)、レスポンスはバージョン + ステータスコード + 理由句(例: `HTTP/1.1 200 OK`)で、別の意味になります。

| セクション | 内容 | 件数を示すHeaderフィールド |
|---|---|---|
| Header | メッセージ全体の制御情報。12byte固定長 | - |
| Question | 問い合わせ内容(QNAME/QTYPE/QCLASS) | QDCOUNT |
| Answer | 質問に対する回答のResource Record群 | ANCOUNT |
| Authority | 権威サーバーを示すResource Record群 | NSCOUNT |
| Additional | 追加情報のResource Record群(グルーレコード等) | ARCOUNT |

レスポンスではHeaderの各カウントに応じてAnswer/Authority/Additionalが続きます。

## Headerセクション

Headerの部分は以下の通り構成されます。

| フィールド | サイズ | 意味 |
|---|---|---|
| ID | 16bit | クライアントが発行する識別子。レスポンスはクエリと同じIDを返却する |
| QR | 1bit | 0は問い合わせ、1はレスポンス |
| Opcode | 4bit | クエリ種別。0=標準クエリ(正常)、1=IQUERY(廃止)、2=サーバーステータス要求 |
| AA | 1bit | Authoritative Answer。応答者がそのゾーンの権威サーバーかどうかを示す |
| TC | 1bit | TrunCation。UDPの512byte制限等でメッセージが切り詰められたことを示しす（HeaderのTCビットが立てられる）。TCビットが立っていればレスポンスを受け取ったクライアントはTCPで再送する必要がある |
| RD | 1bit | Recursion Desired。クライアントが再帰的な名前解決を要求するかどうかを示す |
| RA | 1bit | Recursion Available。サーバーが再帰問い合わせに対応しているかどうかを示す |
| Z | 3bit | 予約領域。将来の拡張用で常に0でなければならない |
| RCODE | 4bit | 応答結果のエラーコード(下表) |
| QDCOUNT | 16bit | Questionセクションのエントリ数 |
| ANCOUNT | 16bit | Answerセクションのリソースレコード数 |
| NSCOUNT | 16bit | Authorityセクションのリソースレコード数 |
| ARCOUNT | 16bit | Additionalセクションのリソースレコード数 |

Headerを合わせると12byte(96bit)になります。

```bash
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

上段の「1 1 1 1 1 1」と下段の「0〜9, 0〜5」を組み合わせて読むと、bit位置は0, 1, 2, ..., 9, 10, 11, 12, 13, 14, 15の16個(0-indexedで0〜15)になります。この1行は16bit=2byte幅ですよという目盛りで、Headerの各行(ID行、flags行、QDCOUNT行、ANCOUNT行、NSCOUNT行、ARCOUNT行)の上に共通して使用されます。12byteという合計は、16bit(2byte)の行が6段(`ID/flags/QDCOUNT/ANCOUNT/NSCOUNT/ARCOUNT`)あることから来ています。flags行は`QR/Opcode/AA/TC/RD/RA/Z/RCODE`をまとめて1行(2byte)に収めたものです。


### RCODEの主な値

RCODEはDNSクエリの応答結果を表示します。

| 値 | 名称 | 意味 |
|---|---|---|
| 0 | NOERROR | 正常終了 |
| 1 | FORMERR | クエリの形式エラー |
| 2 | SERVFAIL | サーバー内部エラー |
| 3 | NXDOMAIN | 問い合わせたドメイン名が存在しない |
| 4 | NOTIMP | サーバーが未実装の機能を要求された |
| 5 | REFUSED | ポリシーにより応答を拒否 |

## Questionセクション

QuestionセクションはQNAME、QTYPE、QCLASSの項目が存在します。

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
| QNAME | 問い合わせ対象のドメイン名(可変長)が格納されている |
| QTYPE | 問い合わせるレコード種別(A、AAAAなど) |
| QCLASS | 問い合わせるクラス(通常はIN) |

## Resource Record

DNSメッセージのAnswer・Authority・Additionalの3つのセクションは、いずれも「Resource Record（RR）」というレコードの配列で構成されています。A、NS、CNAME、MX、TXT、SOAなど、レコードの種類(TYPE)によって中身の意味は異なりますが、フォーマットはすべて共通になっています。

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
| NAME | 可変長 | このレコードが対応するドメイン名 |
| TYPE | 16bit | レコード種別(A=1, NS=2, CNAME=5, SOA=6, MX=15, TXT=16, AAAA=28など) |
| CLASS | 16bit | クラス(通常はIN=1) |
| TTL | 32bit(符号あり扱いだが負値は不可) | このレコードをキャッシュしてよい秒数。0の場合はキャッシュ不可を意味する |
| RDLENGTH | 16bit | 後続するRDATAのbyte長 |
| RDATA | RDLENGTH byte | TYPEとCLASSに応じて解釈される可変長データ。例えばTYPE=AならIPv4アドレス4byte、TYPE=CNAMEならドメイン名、TYPE=MXならpreference値+メールサーバー名、といった具合に中身の形式が変わる |

`NAME/TYPE/CLASS/TTL/RDLENGTH`の5つは、どのTYPEのレコードであっても同じ順序・同じ形式で読み取れます。まず共通フォーマット部分を読み切り、その後`RDLENGTH`（実データ長から算出する値）分のバイト列を切り出して、TYPEに応じたデコーダーを行う、二段階の処理になります。実際にそのデコーダーが読み進めたbyte数が事前に分かっていた`RDLENGTH`の値と一致しているかを検証し、不一致の場合はメッセージが壊れているか、パーサにバグがあることを意味します。

## ドメイン名のエンコーディング

ドメイン名はラベルの列であり、各ラベルは「長さ(1byte) + ラベル本体」の形式で連続してエンコードされ、末尾は長さ0の1byte(ルートラベル)で終わります。
```
例）www.example.com. →
  3 'w' 'w' 'w'  7 'e' 'x' 'a' 'm' 'p' 'l' 'e'  3 'c' 'o' 'm'  0
```

- 名前全体は長さbyte・終端の0byteを含めて最大255byteです
- ルート(`.`)は長さ0の1byteのみで表現されます

## 名前圧縮(Message Compression)

DNSメッセージでは同じドメイン名が何度も検索されることがあります(QuestionのQNAMEと、それに対応するAnswerのNAME)。そのたびにドメイン名をエンコーディングして全体を書き直すとメッセージが膨らみ、特にUDPの場合、512byte制限に引っかかることがあります。そのため、既出のドメイン名(の一部)を**ポインタ**2byteで参照する圧縮を行います。

### ポインタの形式

ポインタは、通常のラベル長bit(先頭2bitが`00`)と区別できるよう先頭2bitを`11`に固定した2byteです。残り14bitが「メッセージ先頭(Headerの先頭byte)からのオフセット」を表し、デコーダはそのoffset位置（メッセージの先頭から数えて何byte目かを示す位置の数値）から名前(の続き)を読み直します。

```
     1  1
     5  4  13 12 11 10 9  8  7  6  5  4  3  2  1  0
   +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
   | 1  1|                OFFSET                   |
   +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
```

デコーダはラベル列を1byteずつ読み進めながら、各byteの先頭2bitで「これはラベル長かポインタか」を判定します。

| 先頭2bit | 意味 |
|---|---|
| `00` | 通常のラベル長(残り6bitがラベル長0〜63) |
| `11` | ポインタ(残り14bitがオフセット) |
| `01`, `10` | 未使用[[RFC1035]](#参考文献) |

### 具体例

Header(12byte)の直後、offset 12からQuestionセクションのQNAMEとして`example.com.`が書き込まれているとします。

```
offset:  12  13  14  15  16  17  18  19  20  21  22  23  24
byte  :   7  'e' 'x' 'a' 'm' 'p' 'l' 'e'  3  'c' 'o' 'm'  0
```

この後、Additionalセクションのグルーレコードで`ns1.example.com.`という名前を書きたい場合、`example.com.`の部分は上のoffset 12を指すポインタで済ませられます。

```
  03 'n' 's' '1'  C0 0C
```

- `03 'n' 's' '1'` … 通常のラベル(長さ3の"ns1")
- `C0 0C` … ポインタ。2byteをbit列にすると`1100000000001100`で、先頭2bit`11`(ポインタ判別)+残り14bit`00000000001100`(10進で12)。つまり「offset 12から読み直せ」という指示

デコーダはこの6byteを「"ns1"」+「offset 12から読んだ"example.com."」として`ns1.example.com.`を復元します。圧縮しない場合17byte必要な名前が、6byteで済んでいます。

## RDATAのフォーマット(TYPE別)

RDATAに格納される内容は、TYPEの種類によって異なります。
TYPE=Aなら4byteのIPv4アドレスがそのまま格納され、TYPE=SOAならMNAME/RNAME(ドメイン名2つ)に続けて32bitの数値が5つ並ぶ仕様となっています。

| TYPE | 値 | RDATAの内容 |
|---|---|---|
| A | 1 | IPv4アドレス。4byte |
| NS | 2 | ネームサーバー名(NSDNAME)。ドメイン名1つ |
| CNAME | 5 | 正規名(CNAME)。ドメイン名1つ |
| SOA | 6 | MNAME, RNAME(ドメイン名) + SERIAL, REFRESH, RETRY, EXPIRE, MINIMUM(各32bit) |
| MX | 15 | PREFERENCE(16bit) + EXCHANGE(ドメイン名) |
| TXT | 16 | 1つ以上のcharacter-string |
| AAAA | 28 | IPv6アドレス。16byte[[RFC3596]](#参考文献) |

**character-string**はTXTレコードなどで使われる、長さ1byteプレフィックス付きの可変長文字列(Pascal文字列形式)のことです。
「長さ(1byte, 0〜255) + 本体」の形式で、ドメイン名のラベルとは異なり圧縮ポインタの対象にはなりません（character-stringの長さbyteは0〜255まるごと使用することができ、ドメイン名のラベル長は上位2bitを予約するせいで63までしか使用できない状況とは異なるため）。例えば、helloは以下の形式となります。
```
"hello" → 5 'h' 'e' 'l' 'l' 'o'
```
TXTのRDATAはRDLENGTHが尽きるまでcharacter-stringを繰り返し読むことで、複数文字列を1レコードに格納できます。
例えばRDLENGTH=12のRDATAが5 'h''e''l''l''o' 5 'w''o''r''l''d'なら、"hello"と"world"という2つのcharacter-stringが1レコードに入っていることになります。


## CLASSの値

| CLASS名 | 値 | 備考 |
|---|---|---|
| IN | 1 | インターネット。実運用で使うのはほぼこれ |
| CS | 2 | CSNETクラス。現在は廃止 |
| CH | 3 | Chaosnetクラス |
| HS | 4 | Hesiod。MIT Project Athenaのネームサービス用 |

### なぜTYPEとCLASSが別れているのか
何のデータかとどこのデータかを別々に表現しています。

- **TYPE**: そのレコードが「何の種類のデータか」(A=IPv4アドレス、NS=ネームサーバー、MX=メール交換先など)を表します
- **CLASS**: そのレコードが「どのネットワーク・プロトコル体系(名前空間)に属するデータか」を表します

DNSが設計された1980年代当時はTCP/IPだけがネットワークプロトコルではなく、Xerox NSやChaosnetなど複数のプロトコル体系が併存していました。そのためDNSは「TCP/IP専用の名前解決システム」ではなく、同じ階層構造・同じメッセージフォーマットを異なるネットワーク体系でも使い回せる汎用の名前解決基盤として設計され、CLASSはその拡張軸として用意されました。同じ`TYPE=A`でもCLASSが違えばRDATAの意味・フォーマットが変わりうる、という想定です。実際にはTCP/IPが多く使用されるようになり、CLASSはほぼ`IN`固定となっています。

---

## 参考文献

- [RFC1034] Mockapetris, P., "Domain Names - Concepts and Facilities", STD 13, RFC 1034, November 1987.
  https://www.rfc-editor.org/rfc/rfc1034
- [RFC1035] Mockapetris, P., "Domain Names - Implementation and Specification", STD 13, RFC 1035, November 1987.
  https://www.rfc-editor.org/rfc/rfc1035
- [RFC3596] Thomson, S., Huitema, C., Ksinant, V., and M. Souissi, "DNS Extensions to Support IP Version 6", STD 88, RFC 3596, October 2003.
  https://www.rfc-editor.org/rfc/rfc3596
