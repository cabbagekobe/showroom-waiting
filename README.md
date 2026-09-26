# showroom-waiting

**English** | [日本語](#showroom-waiting-日本語)

A CLI that puts a specific SHOWROOM room into a **recording-standby** state and, once the broadcast starts, automatically records the HLS stream. It records a single broadcast and then exits.

## Disclaimer

- This is an **unofficial** tool and is **not affiliated with, endorsed by, or connected to SHOWROOM** in any way.
- It relies on SHOWROOM's public API. That API may change or become unavailable at any time, which can break this tool without notice.
- The software is provided **"as is", without any warranty**. The author accepts **no responsibility or liability** for any damage, loss, or trouble arising from its use.
- **Use at your own risk.** You are responsible for reviewing and complying with SHOWROOM's Terms of Service and any applicable laws.

## Requirements

- Go 1.26 or later (to build)
- [ffmpeg](https://ffmpeg.org/) (at runtime, must be on `PATH`)

## Build

```sh
go build -o showroom-waiting .
```

## Usage

```sh
showroom-waiting <room URL or url_key> [options]
```

Examples:

```sh
# Specify by room URL (wait until the broadcast starts, then record)
showroom-waiting https://www.showroom-live.com/nba_b_hinaho0303

# Specify url_key directly, with output directory and quality
showroom-waiting nba_b_hinaho0303 -o ~/recordings -q best

# Wait for 30 minutes only (exit if it doesn't start by then)
showroom-waiting nba_b_hinaho0303 -timeout 30
```

### Options

| Flag | Default | Description |
|------|---------|-------------|
| `-o`   | (config file, otherwise current directory) | Base directory for recordings |
| `-q`   | `best` | Quality `best` / `medium` / `low` (falls back to the highest quality if the requested one is unavailable) |
| `-i`   | `30`   | Polling interval (seconds) for checking whether the broadcast has started |
| `-timeout` | `0` | Maximum wait time (minutes). `0` means unlimited |

The positional argument (URL/url_key) may appear before or after the flags.
Pressing `Ctrl+C` during recording keeps the recording captured so far and exits.

### Output location

Recordings are **saved into a `url_key` subdirectory created under the base directory**.

```
<base directory>/<url_key>/<url_key>_<broadcast start time>.ts
```

Example: specifying `https://www.showroom-live.com/r/46_tomisatonao` saves to
`<base directory>/46_tomisatonao/46_tomisatonao_1784887247.ts`.

A verbose ffmpeg log is written next to the recording as `<recording>.ts.log` (segment requests, byte counts, fetch failures), so gaps in a recording can be diagnosed afterwards.

The base directory is determined in the following order of priority:

1. `-o` flag
2. `output_dir` in the config file (see below)
3. Current directory (`.`)

### Config file

You can set the base output directory in a config file so you don't have to pass `-o` every time.

- Path: `~/.config/showroom-waiting/config.json`
  (if `XDG_CONFIG_HOME` is set, `showroom-waiting/config.json` under that directory)
- Format: JSON. Configuration is optional; if the file is absent, the default behavior applies.

```json
{
  "output_dir": "~/recordings"
}
```

A leading `~` in `output_dir` is expanded to the home directory.

## How it works

It uses only SHOWROOM's public API (no login required).

1. Extract `room_url_key` from the input URL
2. Check the room's existence and `is_live` via `GET /api/room/status?room_url_key=KEY`
3. Poll every `-i` seconds until `is_live` becomes `true` (i.e. recording standby)
4. When the broadcast starts, get the HLS (`.m3u8`) URL via `GET /api/live/streaming_url?room_id=RID`
   (the list can be empty right after the broadcast starts, so it retries a few times)
5. Record to `.ts` with `ffmpeg -i <m3u8> -c copy`. When the broadcast ends, ffmpeg exits on its own and the program terminates

> SHOWROOM streams over both WebRTC and HLS. This tool records the HLS that ffmpeg can handle.
> For the rare room where HLS is not provided, recording is not possible and the tool exits with an error to that effect.

## Test

```sh
go test ./...
```

It verifies pure logic (URL parsing, HLS selection) and API response parsing (stubbed with `httptest`).

## License

MIT License. See [LICENSE](LICENSE).

---

# showroom-waiting (日本語)

[English](#showroom-waiting) | **日本語**

SHOWROOM の特定ルーム URL を指定すると **録画待機状態** に入り、配信が始まったら自動で HLS を録画する CLI。1 配信を録画したら終了します。

## 免責事項

- 本ツールは **非公式** ツールであり、**SHOWROOM 運営とは一切関係ありません**（提携・承認・関連いずれもありません）。
- SHOWROOM の公開 API に依存しています。この API は予告なく変更・提供停止される可能性があり、その場合ツールが動作しなくなることがあります。
- 本ソフトウェアは **無保証（"as is"）** で提供されます。利用により生じたいかなる損害・損失・トラブルについても、作者は **一切の責任を負いません**。
- **利用は自己責任** でお願いします。SHOWROOM の利用規約および関連法令の確認・遵守は各自の責任で行ってください。

## 必要環境

- Go 1.26 以上（ビルド時）
- [ffmpeg](https://ffmpeg.org/)（実行時、`PATH` に必要）

## ビルド

```sh
go build -o showroom-waiting .
```

## 使い方

```sh
showroom-waiting <ルームURL または url_key> [オプション]
```

例:

```sh
# ルームURLで指定（配信開始まで待機し、始まったら録画）
showroom-waiting https://www.showroom-live.com/nba_b_hinaho0303

# url_key 直接指定 + 出力先と画質を指定
showroom-waiting nba_b_hinaho0303 -o ~/recordings -q best

# 30分だけ待機（それまでに始まらなければ終了）
showroom-waiting nba_b_hinaho0303 -timeout 30
```

### オプション

| フラグ | 既定値 | 説明 |
|--------|--------|------|
| `-o`   | （設定ファイル、無ければカレント） | 録画の保存先ベースディレクトリ |
| `-q`   | `best` | 画質 `best` / `medium` / `low`（指定画質が無ければ最高画質にフォールバック） |
| `-i`   | `30`   | 配信開始を確認するポーリング間隔（秒） |
| `-timeout` | `0` | 待機の最大時間（分）。`0` は無制限 |

位置引数（URL/url_key）はフラグの前後どちらに置いても動作します。
録画中に `Ctrl+C` すると、そこまでの録画を残して終了します。

### 保存先

録画は **ベースディレクトリ配下に `url_key` のサブディレクトリを作り、その中に保存**します。

```
<ベースディレクトリ>/<url_key>/<url_key>_<配信開始時刻>.ts
```

例: `https://www.showroom-live.com/r/46_tomisatonao` を指定すると
`<ベースディレクトリ>/46_tomisatonao/46_tomisatonao_1784887247.ts` に保存されます。

録画ファイルの隣に ffmpeg の詳細ログが `<録画ファイル>.ts.log` として保存されます（セグメントごとの取得記録・バイト数・取得失敗）。録画に欠落があった場合の原因調査に使えます。

ベースディレクトリは次の優先度で決まります。

1. `-o` フラグ
2. 設定ファイルの `output_dir`（下記）
3. カレントディレクトリ（`.`）

### 設定ファイル

保存先ベースディレクトリを設定ファイルで指定でき、毎回 `-o` を打たずに済みます。

- パス: `~/.config/showroom-waiting/config.json`
  （環境変数 `XDG_CONFIG_HOME` が設定されていればそちら配下の `showroom-waiting/config.json`）
- 形式: JSON。設定は任意で、ファイルが無ければ既定動作になります。

```json
{
  "output_dir": "~/recordings"
}
```

`output_dir` の先頭 `~` はホームディレクトリに展開されます。

## 動作の仕組み

SHOWROOM の公開 API のみを使用します（ログイン不要）。

1. 入力 URL から `room_url_key` を抽出
2. `GET /api/room/status?room_url_key=KEY` でルームの存在と `is_live` を確認
3. `is_live` が `true` になるまで `-i` 間隔でポーリング（＝録画待機）
4. 配信開始を検知したら `GET /api/live/streaming_url?room_id=RID` で HLS(`.m3u8`) URL を取得
   （配信直後は一覧が空のことがあるため数回リトライ）
5. `ffmpeg -i <m3u8> -c copy` で `.ts` に録画。配信終了で ffmpeg が自然終了し、プログラムも終了

> SHOWROOM は WebRTC と HLS の両方を配信します。本ツールは ffmpeg で扱える HLS を録画します。
> 稀に HLS が提供されないルームでは録画できず、その旨のエラーで終了します。

## テスト

```sh
go test ./...
```

純粋ロジック（URL 解析・HLS 選択）と API レスポンスのパース（`httptest` によるスタブ）を検証します。

## ライセンス

MIT ライセンス。[LICENSE](LICENSE) を参照してください。
