# DNSプロトコル仕様

本ドキュメントは、本リポジトリが実装するDNSメッセージのワイヤーフォーマットについてまとめたものです。

## 1. メッセージの全体構造

DNSメッセージは、クエリ・レスポンスを問わず同一フォーマットで、5つのセクションから構成されます。DNSがQRビット1つで済ませることができるのは、「質問も回答も同じ語彙(名前+タイプ+クラス)で表現できる」というドメイン特性を活かした設計になります。
一方で、HTTPの場合は、クエストはメソッド + パス + バージョン(例: GET /index.html HTTP/1.1)、レスポンスはバージョン + ステータスコード + 理由句(例: HTTP/1.1 200 OK)で、意味は全く別物になります。

| セクション | 内容 | 件数を示すHeaderフィールド |
|---|---|---|
| Header | メッセージ全体の制御情報。12byte固定長 | - |
| Question | 問い合わせ内容(QNAME/QTYPE/QCLASS) | QDCOUNT |
| Answer | 質問に対する回答のResource Record群 | ANCOUNT |
| Authority | 権威サーバーを示すResource Record群 | NSCOUNT |
| Additional | 追加情報のResource Record群(グルーレコード等) | ARCOUNT |

クエリでは通常Questionのみが埋まりANCOUNT等は0となり、レスポンスではHeaderの各カウントに応じてAnswer/Authority/Additionalが続きます。
カウントフィールドは実データの件数と必ず一致していなければならず、送信側はセクションの実長から算出して書き込みます。

## 2. Headerセクション

Headerセクションは以下の通り構成されます。

| フィールド | サイズ | 意味 |
|---|---|---|
| ID | 16bit | クライアントが発行する識別子。レスポンスはクエリと同じIDを返します |
| QR | 1bit | 0は問い合わせ、1はレスポンス |
| Opcode | 4bit | クエリ種別。0=標準クエリ(正常)、1=IQUERY(廃止)、2=サーバーステータス要求 |
| AA | 1bit | Authoritative Answer。応答者がそのゾーンの権威サーバーかどうかを示します |
| TC | 1bit | TrunCation。UDPの512byte制限等でメッセージが切り詰められたことを示します。立っていればクライアントはTCPで再送する必要があります |
| RD | 1bit | Recursion Desired。クライアントが再帰的な名前解決を要求するかどうかを示します |
| RA | 1bit | Recursion Available。サーバーが再帰問い合わせに対応しているかどうかを示します |
| Z | 3bit | 予約領域。将来の拡張用で常に0でなければなりません |
| RCODE | 4bit | 応答結果のエラーコード(下表) |
| QDCOUNT | 16bit | Questionセクションのエントリ数 |
| ANCOUNT | 16bit | Answerセクションのリソースレコード数 |
| NSCOUNT | 16bit | Authorityセクションのリソースレコード数 |
| ARCOUNT | 16bit | Additionalセクションのリソースレコード数 |

全フィールドを合わせて12byte(96bit)です。

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

上段の「1 1 1 1 1 1」と下段の「0〜9, 0〜5」を組み合わせて読むと、bit位置は0, 1, 2, ..., 9, 10, 11, 12, 13, 14, 15の16個(0-indexedで0〜15)になります。つまりこのルーラーは「この1行は16bit=2byte幅ですよ」という目盛りで、Headerの各行(ID行、flags行、QDCOUNT行、ANCOUNT行、NSCOUNT行、ARCOUNT行)の上に共通して使用されます。12byteという合計は、16bit(2byte)の行が6段(ID/flags/QDCOUNT/ANCOUNT/NSCOUNT/ARCOUNT)あることから来ています。flags行はQR/Opcode/AA/TC/RD/RA/Z/RCODEをまとめて1行(2byte)に収めたものです。

#### RD/RAの「再帰的」の意味について

DNSの名前空間は階層構造になっており、1つのサーバーがすべての答えを知っている訳ではありません。
www.example.comを解決するには、下記階層を上から順にたどる必要があります。
```
.(ルート) → .com(TLD) → example.com(権威サーバー)
```

問い合わせた側は1つのサーバーにだけ聞き、「最終的な答えが出るまであなたが代わりに調べてきて」と依頼します。そのサーバーが下記のように問い合わせを裏側で実施し、最終結果だけをクライアントに返します。

1. ルートサーバーに聞く → 「.comは知らないが、.comの担当サーバーを教えてあげる」と紹介される
2. .comのTLDサーバーに聞く → 「example.comは知らないが、担当サーバーを教えてあげる」と紹介される
3. example.comの権威サーバーに聞く → やっとwww.example.comのAレコードが返る

- RD(Recursion Desired): クライアントが送るクエリのフラグ。「このサーバーに再帰的な代行を頼みたい」という意思表示です。
- RA(Recursion Available): サーバーが返すレスポンスのフラグ。「うちは再帰的な代行に対応しています」という表明のことを指します。

問い合わせを裏側で受け持ってくれるものを一般的にはフルサービスリゾルバと呼び、クライアントはRD=1でクエリを送信します。フルサービスリゾルバは内部でルート→TLD→権威サーバーへの反復的な問い合わせを代行し、最終結果だけをクライアントに返します。逆にルートサーバーやTLDサーバー自体は通常RD=0(再帰非対応)で、紹介を返すだけです。


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

## 3. Questionセクション

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

## 4. Resource Record共通フォーマット

DNSメッセージのAnswer・Authority・Additionalの3つのセクションは、いずれも「Resource Record（RR）」というレコードの配列で構成されています。A、NS、CNAME、MX、TXT、SOAなど、レコードの種類(TYPE)によって中身の意味は異なりますが、フォーマットはすべて共通になっています。

RFC1035 4.1.3では以下のように定義されています。
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

NAME/TYPE/CLASS/TTL/RDLENGTH は、どんなレコードでも同じ形式でパース可能です。まず共通フォーマット部分を読み切り、その後RDLENGTH（実データ長から算出する値）分のバイト列を切り出して、TYPEに応じたデコーダーを行う、二段階の処理になります。受信側はRDLENGTHぶんだけ読み進めた位置が、そのTYPE用パーサが実際に消費したbyte数と一致することを検証して、不一致はメッセージが壊れているか、パーサにバグがあることを意味します。

## 5. ドメイン名のエンコーディング

ドメイン名はラベルの列であり、各ラベルは「長さ(1byte) + ラベル本体」の形式で連続してエンコードされ、末尾は長さ0の1byte(ルートラベル)で終端します。
RFC1035 3.1では以下のように定義されています。
```
www.example.com. →
  3 'w' 'w' 'w'  7 'e' 'x' 'a' 'm' 'p' 'l' 'e'  3 'c' 'o' 'm'  0
```

- 1ラベルは最大63byteです(長さbyteの上位2bitは圧縮ポインタ用に予約されているため、ラベル長として表現できるのは0〜63です)
- 名前全体は長さbyte・終端の0byteを含めて最大255byteです
- ルート(`.`)は長さ0の1byteのみで表現されます

## 6. 名前圧縮(Message Compression)

DNSメッセージでは同じドメイン名が何度も検索されることがあります(QuestionのQNAMEと、それに対応するAnswerのNAME)。そのたびにドメイン名をエンコーディングして全体を書き直すとメッセージが膨らみ、特にUDPの512byte制限を圧迫します。既出のドメイン名(の一部)を**ポインタ**2byteで参照する圧縮方式を定義しています。

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
| `01`, `10` | RFC1035では未使用 |

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
- `C0 0C` … ポインタ。2byteをbit列にすると`1100000000001100`で、先頭2bit`11`(ポインタ判別)+残り14bit`00000000001100`(10進で12)。つまり「offset 12から読み直せ」という指示です

デコーダはこの6byteを「"ns1"」+「offset 12から読んだ"example.com."」として`ns1.example.com.`を復元します。圧縮しない場合17byte必要な名前が、6byteで済んでいます。

### 注意点

- ポインタは必ず**後方(より小さいオフセット)** を指します。前方参照や自己参照は無限ループを防ぐため、不正なメッセージとして拒否しなければなりません。
- デコーダはポインタを何回でも追従しうるため、最大追従回数などでループを防御する必要があります。

## 7. RDATAのフォーマット(TYPE別)

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
| AAAA | 28 | IPv6アドレス。16byte(RFC3596) |

### character-string

character-stringはTXTレコードなどで使われる、長さ1byteプレフィックス付きの可変長文字列(Pascal文字列形式)のことです。
「長さ(1byte, 0〜255) + 本体」の形式で、ドメイン名のラベルとは異なり圧縮ポインタの対象にはなりません（character-stringの長さbyteは0〜255まるごと使用することができ、ドメイン名のラベル長は上位2bitを予約するせいで63までしか使用できない状況とは異なるため）。例えば、helloは以下の形式となります。
```
"hello" → 5 'h' 'e' 'l' 'l' 'o'
```
TXTのRDATAはRDLENGTHが尽きるまでcharacter-stringを繰り返し読むことで、複数文字列を1レコードに格納できます。
例えばRDLENGTH=12のRDATAが5 'h''e''l''l''o' 5 'w''o''r''l''d'なら、"hello"と"world"という2つのcharacter-stringが1レコードに入っていることになります。

### SOAレコードの各フィールド

SOA(Start Of Authority)はゾーンの管理情報そのものを表すレコードで、プライマリ(マスター)とセカンダリ間のゾーン同期を成立させるためのフィールドです。

| フィールド | 意味 |
|---|---|
| MNAME | このゾーンのマスターサーバー名 |
| RNAME | ゾーン管理者のメールアドレス(`@`を`.`に置き換えた形式 (例:
dns-admin.google.com.はdns-admin@google.com) |
| SERIAL | ゾーンのバージョン番号。セカンダリはこれで更新を検知します |
| REFRESH | セカンダリがマスターに更新確認しに行く間隔(秒) |
| RETRY | REFRESHに失敗した際の再試行間隔(秒) |
| EXPIRE | この期間更新できなければセカンダリはゾーンを権威なしとみなします(秒) |
| MINIMUM | ネガティブキャッシュ(RFC2308)のTTLとして扱われます(RFC2181以降の解釈) |

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

### なぜTYPEとCLASSが別れているのか

- **TYPE**: そのレコードが「何の種類のデータか」(A=IPv4アドレス、NS=ネームサーバー、MX=メール交換先など)を表します
- **CLASS**: そのレコードが「どのネットワーク・プロトコル体系(名前空間)に属するデータか」を表します

つまり「何のデータか」と「どの世界のデータか」という別々の表現をしています。

DNSが設計された1980年代当時はTCP/IPだけがネットワークプロトコルではなく、Xerox NSやChaosnetなど複数のプロトコル体系が併存していました。そのためDNSは「TCP/IP専用の名前解決システム」ではなく、同じ階層構造・同じメッセージフォーマットを異なるネットワーク体系でも使い回せる汎用の名前解決基盤として設計され、CLASSはその拡張軸として用意されました。同じ`TYPE=A`でもCLASSが違えばRDATAの意味・フォーマットが変わりうる、という想定です。実際にはTCP/IPが多く使用されるようになり、CLASSはほぼ`IN`固定となっていますが、ワイヤーフォーマット上は今も必須フィールドとして残っており、CHクラスは今でもBINDのバージョン確認(`dig CH TXT version.bind`)などで実用されています。

## 9. トランスポートとメッセージサイズ

- **UDP**: 512byteを超えるメッセージは切り詰められ、HeaderのTCビットが立てられます。
- **TCP**: メッセージの前に2byteのビッグエンディアン長プレフィックスを付与して送ります。TCビットが立ったレスポンスを受け取ったクライアントはTCPで再送します

## 10. digコマンドで実際のDNSメッセージを見る

ここまでの仕様が実際のDNSメッセージでどう現れるかは、`dig`コマンドで手元から確認できます。
`dig`はDNSサーバーにクエリを送り、返ってきたメッセージを人間が読みやすい形式で表示するだけの読み取り専用ツールで、これまでの節で説明したHeader/Question/Answer/Authority/Additionalの各セクションが、ほぼそのままの構成で出力に現れます。

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
| `flags: qr rd ra` | Headerのフラグ群(`QR`/`RD`/`RA`が立っています。立っていないフラグは表示されません) | 2節 |
| `QUERY: 1, ANSWER: 6, AUTHORITY: 0, ADDITIONAL: 0` | QDCOUNT/ANCOUNT/NSCOUNT/ARCOUNT | 2節 |
| `;; QUESTION SECTION:` 以下 | Questionセクション(QNAME, QCLASS, QTYPE の順で表示) | 3節 |
| `;; ANSWER SECTION:` 以下 | Answerセクション(NAME, TTL, CLASS, TYPE, RDATA の順で表示) | 4, 7節 |
| `;; AUTHORITY SECTION:` (該当時) | Authorityセクション | 1, 4節 |
| `;; ADDITIONAL SECTION:` (該当時) | Additionalセクション | 1, 4節 |
| `;; MSG SIZE  rcvd:` | 受信したメッセージ全体のbyte数 | 1節 |

`;`で始まる行はコメント(dig自身が付与した注釈)であり、DNSメッセージそのものの一部ではありません。
実際のリソースレコードの行は`NAME TTL CLASS TYPE RDATA`の順で1レコード1行に整形されています。
これは4節のResource Record共通フォーマットの各フィールドがそのまま列挙されたもので、`google.com. 83 IN A 172.217.209.139`であれば

- NAME: `google.com.`
- TTL: `83`(秒。このレコードをキャッシュしてよい残り秒数)
- CLASS: `IN`
- TYPE: `A`
- RDATA: `172.217.209.139`

と読み替えられます。

### flagsの読み方

`flags:`の行には、Headerの1bitフラグのうち**立っているものだけ**が略称で列挙されます
(立っていないフラグは表示されません)。

| 略称 | 対応するHeaderフィールド |
|---|---|
| `qr` | QR(このメッセージがResponseであることを示します。クエリ側の表示には出ません) |
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

NSレコードのRDATA(`ns1.google.com.`)はネームサーバーの名前でありIPアドレスではないため、そのままでは名前解決の役に立ちません。
そこでサーバーは、そのNSレコードに対応するA/AAAAレコードをAdditionalセクションに**グルーレコード**として付加しています。

**SOAレコード**：

```
$ dig +noedns google.com SOA
;; ANSWER SECTION:
google.com.	45	IN	SOA	ns1.google.com. dns-admin.google.com. 965863404 900 900 1800 60
```

RDATAの並びはMNAME, RNAME, SERIAL, REFRESH, RETRY, EXPIRE, MINIMUMの順です(7節参照)。
`dns-admin.google.com.`はRNAME、つまりゾーン管理者のメールアドレス
(`dns-admin@google.com`の`@`を`.`に置き換えた表記)です。

**存在しないドメイン(NXDOMAIN)の例**:

```
$ dig +noedns thisdomaindoesnotexist12345.com A
;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: 21583
;; flags: qr rd ra; QUERY: 1, ANSWER: 0, AUTHORITY: 1, ADDITIONAL: 0

;; AUTHORITY SECTION:
com.	900	IN	SOA	a.gtld-servers.net. nstld.verisign-grs.com. 1787099934 1800 900 604800 900
```

`status: NXDOMAIN`はHeaderのRCODEが3であることを示します(2節のRCODE表を参照)。ANSWERは0件ですが、
AUTHORITYセクションに親ゾーン(`com.`)のSOAレコードが1件返っています。これはネガティブキャッシュ
(RFC2308)のためで、SOAのMINIMUMフィールド(5節)がこの否定応答をキャッシュしてよい秒数として
使われます。

### よく使うオプション

| オプション | 効果 |
|---|---|
| `+noedns` | EDNS0のOPT疑似レコードを付けずにクエリを送ります。ADDITIONALセクションにOPTが混ざらないため、本リポジトリの`message`パッケージが扱う範囲(EDNS0未対応)とメッセージ構造を素直に対応させやすくなります |
| `+short` | RDATAだけを簡潔に表示します |
| `+tcp` | UDPではなくTCP経由で問い合わせます(9節の2byte長プレフィックスの動作を伴います) |
| `+trace` | ルートサーバーから反復的に権威サーバーを辿る過程を表示します(`resolver`パッケージが将来行う処理のイメージに近いです) |
