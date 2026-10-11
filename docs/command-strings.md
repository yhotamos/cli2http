# コマンド文字列の指定

実行ファイル名と固定引数は、1つの文字列にまとめます。起動シェルは外側の引用符を取り除くため、空白を含むパスや引数を囲む内側の引用符は、以下のように保持して渡してください。

## 実行ファイルの指定

### cmd

```bat
cli2http "\"C:\Program Files\ImageMagick\magick.exe\" -resize 800x600"
```

### pwsh

```powershell
cli2http '"C:\Program Files\ImageMagick\magick.exe" -resize 800x600'
```

PowerShell 7.3 以降で、`$PSNativeCommandArgumentPassing` が `Windows` または `Standard` の場合の例です。

### bash・zsh（Linux・macOS）

```sh
cli2http '/usr/local/bin/magick -resize 800x600'
```

## 引数の引用符

空白を含む引数は、コマンド文字列の中で二重引用符で囲みます。pwsh・bash・zsh の例：

```sh
cli2http 'magick "My Pictures/input.jpg" -resize 800x600 output.jpg'
```

`My Pictures/input.jpg` が1つの引数になります。

Windows では `\` をそのまま使えます。ただし、二重引用符の直前にある末尾の `\` は2つにします。`C:\Pictures\` を引用符で囲む場合は `"C:\Pictures\\"` と書きます。

## 実行と展開

対象 CLI はシェル経由ではなく直接実行し、固定引数の後ろに HTTP の `args` を追加します。

cli2http は変数展開・コマンド置換を行わず、`|`・`>`・`&&` なども通常の引数として渡します。ただし、起動シェルが cli2http に渡す前に展開する場合があります。

引数の妥当性は対象 CLI に任せます。対象 CLI の機能や権限は制限しません。
