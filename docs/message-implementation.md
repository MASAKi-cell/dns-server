# messageパッケージの実装

[dns-protocol.md](./dns-protocol.md)で説明したワイヤーフォーマットを、`message`パッケージが
Goの型とエンコード/デコード処理としてどう表現しているかをまとめます。

## 1. パッケージの責務

`message`パッケージが担うのは、DNSメッセージのバイト列 ⇔ Go構造体の相互変換のみです。
ソケットの読み書き、ゾーンデータの管理、名前解決アルゴリズムはこのパッケージの範囲外で、
それぞれ`client`/`zone`/`server`/`resolver`が担う想定です(いずれも未実装)。

## 2. ファイル構成

```
message/
├── types.go    # Type/Class/Opcode/RCodeなどプロトコル上の列挙値とString()
├── header.go   # Header構造体とそのmarshal/読み取り
├── question.go # Question構造体とそのmarshal/読み取り
├── name.go     # Name型(ドメイン名)のラベル分解・エンコード・名前圧縮のデコード
├── rr.go       # ResourceRecord構造体とそのmarshal/読み取り、RDLENGTHの算出・検証
├── rdata.go    # RDataインターフェースとTYPE別実装(AData, NSData, SOADataなど)
├── codec.go    # decoder(バイト列を読み進めるカーソル)とプリミティブな読み取りヘルパー
└── message.go  # Message構造体。全セクションを束ねたMarshal/Unmarshalのエントリポイント
```

## 3. 型とDNS仕様の対応

| Go型 | 対応するDNS仕様 |
|---|---|
| `Message` | メッセージ全体(Header + Question×N + Answer/Authority/Additional×N) |
| `Header` | 12byte固定ヘッダー。フラグ群は`QR`/`AA`等の`bool`フィールドに分解して保持し、ワイヤー上は1つの16bit `flags`値にパックします |
| `Question` | Questionセクションの1エントリ |
| `ResourceRecord` | Answer/Authority/Additionalセクションの1エントリ。RDLENGTHはフィールドとして持たず、marshal時に`RData.marshal`の結果から都度算出します |
| `RData` | RDATA部分のインターフェース。TYPEごとに`AData`/`NSData`/`CNAMEData`/`MXData`/`TXTData`/`SOAData`が実装し、未対応のTYPEは生バイト列を保持する`RawData`にフォールバックします |
| `Name` | `string`を基底型とし、`"www.example.com."`のようなドット区切りのFQDN形式で保持します |

## 4. Marshal(エンコード)の流れ

エントリポイントは`Message.Marshal()`です(`message.go`)。

1. `Header`のQDCOUNT/ANCOUNT/NSCOUNT/ARCOUNTを、実際の各スライスの長さで上書きします
   (呼び出し側が件数を手で合わせる必要はありません)
2. `Header.marshal`で12byte分を`buf`に追記します
3. `Questions`を先頭から順に`Question.marshal`で追記します
4. `Answers` → `Authorities` → `Additionals`の順に、それぞれ`ResourceRecord.marshal`で追記します

いずれの層も「`[]byte`を受け取り、追記した`[]byte`を返す」というシグネチャで統一されており、
`append`によるバッファの使い回しを前提にしています。

### RDLENGTHの算出(`rr.go`)

`ResourceRecord.marshal`はRDLENGTHを事前に計算せず、次の手順で書き戻します。

1. RDLENGTHの位置に0のプレースホルダーを書き込み、その位置(`rdlengthPos`)を覚えておきます
2. `RData.marshal`でRDATA本体を追記します
3. 追記後の長さとRDATA開始位置の差分からRDLENGTHの実値を求め、`rdlengthPos`に上書きします

RDATAの内容(特に`Name`を含む場合)を書いてみるまで正確な長さが分からないため、
この「後から書き戻す」方式になっています。

## 5. Unmarshal(デコード)の流れ

エントリポイントは`Unmarshal(data []byte)`です(`message.go`)。

1. `newDecoder(data)`でカーソルを初期化します
2. `d.readHeader()`でHeaderを読み取ります
3. `Header.QDCount`件、`d.readQuestion()`をループしてQuestionsを読みます
4. `Header.ANCount`/`NSCount`/`ARCount`件ずつ、`d.readResourceRecords(n)`で
   Answer/Authority/Additionalを読みます

各ステップのエラーは`fmt.Errorf("message: read xxx: %w", err)`の形でラップされ、
どのセクションで失敗したかが呼び出し元まで伝わるようになっています。

## 6. decoderの設計(`codec.go`)

```go
type decoder struct {
    buf []byte
    pos int
}
```

単純な「今どこまで読んだか」のカーソルに見えますが、`buf`全体を保持しているのが重要な設計判断です。
名前圧縮ポインタ(6節参照)はメッセージ内の任意の**過去の位置**を後方参照するため、
すでに読み終えた領域にも自由にアクセスできる必要があり、そのために全体バッファを
手放さない構造になっています。

プリミティブな読み取りヘルパーは次のとおりです。

| メソッド | 用途 |
|---|---|
| `readUint8` / `readUint16` / `readUint32` | ビッグエンディアンで固定長整数を読みます |
| `readBytes(n)` | 次のnbyteを返します(内部バッファを直接指すスライスなので、呼び出し側が保持するならコピーが必要です) |
| `readCharacterString` | 1byte長プレフィックス付き文字列(TXTのRDATAなどで使用)を読みます |

いずれも読み取り前にバッファ長を検証し、不足時は`d.pos`の情報を含むエラーを返します。

## 7. Nameのエンコード/デコード(`name.go`)

### エンコード

`Name.marshal`は次の順で処理します。

1. `labels()`でドット区切りのFQDNをラベルのスライスに分解します。空ラベル
   (`"www..com."`のような入力)はエラーにします
2. 終端の0byteを含めた合計長が255byteを超えないか事前に検証します
3. 各ラベルについて63byteを超えないか検証しつつ、「長さ(1byte) + 本体」を追記します
4. 最後に終端の0byteを追記します

名前圧縮ポインタの**書き込み**は行いません(常にフルスペルでエンコードします)。圧縮による
サイズ削減は現状のスコープ外です。

### デコード(名前圧縮への対応)

`decoder.readName`が本パッケージで最も複雑なロジックで、圧縮ポインタの追従を行います。
実装上のポイントは次のとおりです。

- 読み取り位置として、外から見える`d.pos`とは別にローカルの`cursor`を使います。ポインタを
  追従して過去の位置に飛んでも、`d.pos`(呼び出し元から見た「名前を読み終えた位置」)は
  巻き戻さないようにするためです
- 初めてポインタに遭遇した時点(`jumped`が`false`)でのみ`d.pos`を
  「ポインタの直後」に確定させ、以降何度ポインタを辿っても`d.pos`は変化させません。
  これにより、ポインタが指す先でさらに別のポインタを辿るような多段参照でも、
  呼び出し元は正しく「ポインタ2byte分だけ読み進んだ」状態になります
- ラベル長byteの上位2bitで3パターンに分岐します
  - `00`: 通常のラベル(長さ0〜63)
  - `11`: 圧縮ポインタ(残り14bitがオフセット)
  - `01`/`10`: RFC1035で未定義のため不正な入力としてエラーにします
- ポインタが指す先(`ptr`)が現在位置(`cursor`)より前であることを毎回検証し
  (`ptr >= cursor`を拒否)、かつ`maxCompressionJumps`(128回)で追従回数の上限を設けています。
  この2つの防御によって、自己参照や循環参照によるデコーダの無限ループを防いでいます

## 8. RDATAのディスパッチ(`rdata.go`)

`decoder.readRData(typ Type, rdataEnd int)`がTYPEに応じて専用のreadメソッドへ振り分ける
`switch`文になっています。未対応のTYPEは`readRawData`に落ち、RDLENGTH分のバイト列を
そのまま`RawData`として保持します(パース不能なTYPEでもメッセージ全体の読み取りを継続できます)。

`rdataEnd`(RDATAの終端の絶対オフセット)を引数で渡しているのは、TXTのように内部に複数の
character-stringを含む可変長RDATAを読み切るためです。`readTXTData`は`d.pos < rdataEnd`の間
character-stringを読み続けるループになっています。固定長のRDATA(A/AAAA/NS/CNAME/MX/SOA)は
`rdataEnd`を使わず、フォーマット通りの固定バイト数を読むだけで済みます。

### RDLENGTHの整合性検証(`rr.go`)

`readResourceRecord`は、RDATAを読み終えた後の`d.pos`が事前に計算しておいた`rdataEnd`と
一致するかを検証します。ここが一致しない場合、そのTYPE用パーサがRDLENGTHの示す長さと
異なるバイト数を消費したことを意味し、メッセージの破損かパーサ側のバグを示すため
エラーとして扱います。

## 9. エラーハンドリング方針

独自のエラー型は定義せず、`fmt.Errorf("...: %w", err)`によるラップを各レイヤーで積み重ねる
方式で統一されています。例えば`Message`→`ResourceRecord`→`RData`と処理が降りていく過程で、
それぞれの層が「どのセクション/フィールドの処理で失敗したか」を一言添えてラップするため、
最終的なエラーメッセージだけで失敗箇所をおおよそ特定できます。

## 10. テストの構成

各ファイルに対応する`*_test.go`があり、主に次の観点で書かれています。

- **既知バイト列との一致**: 手で組み立てたバイト列を期待値としたテーブル駆動テストで、
  `marshal`の出力や`readXxx`の結果を検証します(例: `name_test.go`の
  `TestName_MarshalKnownBytes`)
- **異常系**: ラベル長超過、空ラベル、圧縮ポインタの自己参照、バッファ不足による
  途中切れなど、RFC違反やメッセージ破損のケースがエラーになることを確認します

## 11. 今後の実装範囲との関係

`message`パッケージはワイヤーフォーマットの変換のみを提供する土台であり、今後
`docs/`配下には以下のような、他パッケージが実装され次第のドキュメント追加を想定しています。

- `client`: `message.Marshal`/`Unmarshal`を使ってUDP/TCP越しにクエリを送受信する処理
- `zone`: ゾーンファイルをパースし`message.ResourceRecord`相当のレコード群を構築する処理
- `server`: 受信したクエリ(`message.Unmarshal`)をハンドラにディスパッチし、
  応答(`message.Marshal`)を返す処理
- `resolver`: ルートヒントからの反復的な名前解決とキャッシュ
