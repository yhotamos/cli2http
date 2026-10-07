# CLI2HTTP

既存 CLI を localhost の HTTP API として公開するツールです。引数を JSON で渡し、終了コード・標準出力・標準エラーを受け取れます。

## インストール

Go 1.25 以降と、公開したい CLI が必要です。

```sh
go install github.com/yhotamos/cli2http@latest
```

インストール先の Go の bin ディレクトリを PATH に追加してください。更新も同じコマンドで行います。

## 使い方

ImageMagick で画像をリサイズする例です。`input.jpg` のあるディレクトリで起動します。

```sh
cd images
cli2http magick
```

ポートを固定する場合は `cli2http --port 8080 magick` を使います。未指定または `--port 0` は自動割り当てです。

`--token <Token>` で固定 Token を指定できます。16文字以上の半角英数字・`-`・`_` が使用でき、同じ値を指定すれば再起動後も使えます。未指定なら起動ごとに自動生成します。

起動すると URL と Token が表示されます（以下は例です）。

```text
Command : magick
URL     : http://127.0.0.1:48321
Token   : 4e3360d0...
```

アクセスごとにメソッド・パス・HTTP ステータス・処理時間が表示されます。

```text
POST /exec 200 in 498ms
```

別のターミナルから `input.jpg` を 800×600 にリサイズして `output.jpg` として保存します。URL と `<Token>` は、表示された実際の値に置き換えてください。

```sh
curl -X POST 'http://127.0.0.1:48321/exec' \
  -H 'Authorization: Bearer <Token>' \
  -H 'Content-Type: application/json' \
  -d '{"args":["input.jpg","-resize","800x600","output.jpg"]}'
```

レスポンス例：

```json
{
  "exitCode": 0,
  "stdout": "",
  "stderr": ""
}
```

起動したディレクトリに `output.jpg` が作成されます。

ほかの CLI も `cli2http <command>` で公開できます。
`cmd`・`powershell`・`pwsh`・`sh`・`bash` など主要なシェルと `cli2http` 自身は指定できません。対象 CLI の機能や権限を制限するものではありません。

CLI の非ゼロ終了も HTTP 200 で返します。成否は `exitCode` で確認してください。
サーバの終了は Ctrl+C です。Token を自動生成した場合は、再起動すると値が変わります。ポートが自動割り当ての場合は URL も変わります。

## API

| メソッド・パス | 用途                                   | Token |
| -------------- | -------------------------------------- | ----- |
| `POST /exec`   | `args` の文字列配列を CLI に渡して実行 | 必要  |
| `GET /info`    | 対象コマンドとサーバ PID を取得        | 必要  |
| `GET /health`  | 稼働確認                               | 不要  |

引数なしの場合は `{"args":[]}` を送ります。CLI は cli2http の作業ディレクトリで実行されます。
stdout・stderr はそれぞれ最大 10 MiB で、超過すると実行を中断して HTTP 422 を返します。

バージョン確認は `cli2http --version`、ヘルプは `cli2http --help` です。

## ライセンス

[MIT License](LICENSE)

## 作者

yhotta240 [https://github.com/yhotta240](https://github.com/yhotta240)
