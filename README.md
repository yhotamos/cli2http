# CLI2HTTP

既存 CLI を localhost の HTTP API として公開するツールです。引数を JSON で渡し、終了コード・標準出力・標準エラーを受け取れます。

## インストール

Go 1.26 以降と、公開したい CLI が必要です。

```sh
go install github.com/yhotamos/cli2http@latest
```

インストール先の Go の bin ディレクトリを PATH に追加してください。更新も同じコマンドで行います。

## 使い方

ImageMagick で画像をリサイズする例です。`input.jpg` のあるディレクトリで起動します。

```sh
cd images
cli2http "magick"
```

起動すると URL と Token が表示されます（以下は例です）。

```text
URL     : http://127.0.0.1:48321
Token   : 4e3360d0...
/exec : magick
```

アクセスごとにメソッド・パス・HTTP ステータス・処理時間が表示されます。

```text
POST /exec 200 in 498ms
```

画像をリサイズするリクエスト例です。URL と `<Token>` は、表示された実際の値に置き換えてください。

Node.js の例：

```js
const response = await fetch("http://127.0.0.1:48321/exec", {
  method: "POST",
  headers: {
    Authorization: "Bearer <Token>",
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    args: ["input.jpg", "-resize", "800x600", "output.jpg"],
  }),
});
console.log(await response.json());
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

コマンドと固定引数は、1つのコマンド文字列で指定します。

```sh
cli2http --port 8080 "npm install"
```

HTTP の `args` は固定引数の後ろに追加されます。上の例に `{"args":["express","--save"]}` を送ると、`npm install express --save` を実行します。

`--path <パス> <コマンド文字列>` で、CLI を公開するパスを指定できます。複数指定すると、同じポートで複数の CLI を公開できます。Token とタイムアウトも共有します。

```sh
cli2http --port 8080 --path /image "magick" --path /json "jq -r"
cli2http "magick" --path /json "jq"
```

パス未指定のコマンドは `/exec` で公開します。コマンド文字列内の引用符・エスケープの規則は OS ごとに異なります。[空白を含むパスや引用符の指定方法](docs/command-strings.md)を参照してください。

`cmd`・`powershell`・`pwsh`・`sh`・`bash` など主要なシェルと `cli2http` 自身は指定できません。対象 CLI の機能や権限を制限するものではありません。

CLI の非ゼロ終了も HTTP 200 で返します。成否は `exitCode` で確認してください。
サーバの終了は Ctrl+C です。

## オプション

ポート・Token・タイムアウト・作業ディレクトリは最初の公開指定より前に指定します。

```sh
cli2http --port 8080 --timeout 30s "magick"
```

| オプション | 説明 | 既定 |
| ---------- | ---- | ---- |
| `--dir <ディレクトリ>` | すべての CLI の作業ディレクトリ | 起動時の作業ディレクトリ |
| `--port <番号>` | 待ち受けポート（`0` は自動割り当て） | `0` |
| `--path <パス> <コマンド文字列>` | 指定パスで CLI を公開（複数指定可） | 未指定のコマンドは `/exec` |
| `--token <Token>` | 再起動後も使える固定 Token | 起動ごとに自動生成 |
| `--timeout <時間>` | CLI の実行時間制限（例：`30s`・`2m`、`0` は無制限） | `0` |
| `--help` | ヘルプを表示 | — |
| `--version` | バージョンを表示 | — |

固定 Token は16文字以上の半角英数字・`-`・`_` を指定します。ポートを自動割り当てにする場合は、起動時に表示される URL を使ってください。

## API

| メソッド・パス | 用途                                   | Token |
| -------------- | -------------------------------------- | ----- |
| `POST /exec`   | `args` の文字列配列を CLI に渡して実行 | 必要  |
| `GET /info`    | 公開パス・コマンド名の一覧とサーバ PID を取得        | 必要  |
| `GET /health`  | 稼働確認                               | 不要  |

`POST /exec` はパス未指定の場合です。`--path` を指定した CLI は、そのパスで実行します。

引数なしの場合は `{"args":[]}` を送ります。CLI の実行場所は `cli2http --dir ./images "magick"` のように指定できます。省略時と相対パスの基準は起動時の作業ディレクトリです。
stdout・stderr はそれぞれ最大 10 MiB で、超過すると実行を中断して HTTP 422 を返します。タイムアウトも実行を中断して HTTP 422 を返します（`{"error":"command execution timed out"}`）。停止処理のため、応答までに指定時間を超える場合があります。

## ライセンス

[MIT License](LICENSE)

## 作者

yhotta240 [https://github.com/yhotta240](https://github.com/yhotta240)
