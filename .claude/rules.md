## Goスキル

Go関連のタスクでは、迷ったら以下のグローバルスキル（`~/.agents/skills/`）を積極的に呼び出すこと。

- `golang-code-style`: コーディングスタイル（行の長さ、変数宣言、制御フローの明瞭さ）を書く/レビューする時
- `golang-error-handling`: エラーの生成・ラップ(%w)・判定(errors.Is/As)・slogでのログ出力を書く/レビューする時
- `golang-security`: 入力値検証、暗号、シークレット管理、ネットワーク処理などセキュリティに関わるコードを書く/レビューする時。本プロジェクトはUDP/TCPで外部入力を受けるDNSサーバーのため特に重要
- `golang-design-patterns`: functional options、graceful shutdown、DIなどアーキテクチャ判断が必要な時
