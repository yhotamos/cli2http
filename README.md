# CLI2HTTP

既存 CLI を localhost の HTTP API として公開するツールです。
指定した CLI を解決し、localhost の空きポートで HTTP サーバを起動します。
認証 Token は起動ごとに自動生成し、ファイルには保存しません。

## 実行

Go 1.25 以降が必要です。

```sh
go run . git
```

出力例（ポートと Token は起動ごとに変わります）:

```text
Command : git
Address : http://127.0.0.1:48321
Token   : 4e3360d0...

POST /exec
```

サーバは Ctrl+C で終了します。
コマンド未指定、複数指定、実行ファイルが見つからない場合はエラーで終了します。
`POST /exec` と `GET /info` は Bearer Token が必要です。`GET /health` は認証不要です。
ブラウザでは Address に `/health` を付けると稼働確認できます。`GET /` は API がないため 404 を返します。

```sh
go run . --help
go build -o cli2http.exe .
./cli2http.exe git
```

## リクエスト例

起動時に表示された Address と Token を使い、別のターミナルから呼び出します。
例のポート `48321` と `<起動時のToken>` は実際の値に置き換えてください。

### POST /exec

`go run . git` で起動したサーバに、Git のバージョン表示を要求する例です。

```http
POST /exec HTTP/1.1
Host: 127.0.0.1:48321
Authorization: Bearer <起動時のToken>
Content-Type: application/json

{"args":["--version"]}
```

curl の例:

```bash
curl -X POST 'http://127.0.0.1:48321/exec' \
  -H 'Authorization: Bearer <起動時のToken>' \
  -H 'Content-Type: application/json' \
  -d '{"args":["--version"]}'
```

レスポンス例（バージョンは環境によって異なります）:

```json
{
  "exitCode": 0,
  "stdout": "git version ...\n",
  "stderr": ""
}
```

CLI ごとに別プロセスで起動する場合、URL のポートと Token で接続先を区別します。

### API の動作

`/info` の `pid` は cli2http サーバのプロセス ID です。
`/exec` は `args` の文字列配列を対象 CLI に直接渡し、終了するまで待って `exitCode`・`stdout`・`stderr` を返します。引数なしの場合は `{"args":[]}` を指定します。
CLI の非ゼロ終了も HTTP 200 として返します。不正な JSON・引数・Content-Type は 400、認証エラーは 401、起動失敗は 500 です。存在しない API は 404、対象 API の非対応メソッドは 405 になります。
リクエスト本文は最大 1 MiB です。stdout・stderr もそれぞれ最大 10 MiB とし、超過した場合は実行を停止して HTTP 422 と出力上限エラーを返します。
接続切断・サーバ終了・出力上限超過時には、Windows は Job Object、Unix はプロセスグループを停止します。親の終了後に出力パイプが残る場合の待機は最大1秒です。これらはプロセスの後始末であり、サンドボックスではありません。

## 検証

```sh
go test ./...
go vet ./...
```

## コード構成

```text
main.go                プロセスの終了コード
cmd/root.go            Cobra の定義・引数検証・起動処理・表示・割り込み通知
internal/runner/       コマンド解決・直接実行・実行結果
internal/server/       HTTP サーバ・API ハンドラ・Token 認証
```

`cmd` が CLI の入出力を担当し、`internal` は Cobra に依存しません。
各責務のテストは同じディレクトリの `*_test.go` に配置しています。
