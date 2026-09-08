# Yoroshiku EPUB Agent

![Yoroshiku EPUB Agent](assets/hero.png)

原稿と表紙を置いて、Codexに「よろしく」。Markdown原稿・表紙・挿絵から、Amazon KDPへアップロードできるEPUBと出版準備資料を作るエージェントです。

Python、Node.js、Java、Goなどの開発環境は利用者側に不要です。Windows / macOS / Linux用の単体Builderを同梱し、素材の判断はCodex、EPUB生成と検証は決定論的なコードが担当します。

## いちばん簡単な使い方

1. GitHub Releasesから完成済みZIPをダウンロードして解凍する。
2. `book-info.md` の分かる項目だけ入力する。
3. Markdown原稿を `manuscript/`、表紙・挿絵を `images/` へ入れる。
4. Codexでこのフォルダを開く。
5. 「KDP出版用によろしく」と依頼する。
6. 完成した `release/book.epub` をKindle Previewerで確認する。
7. KDPへ `book.epub` と `release/cover.jpg` をアップロードする。

ZIPには次の入力場所が最初から入っています。

```text
book-info.md       ← EPUBに入れるタイトル・著者名など
manuscript/        ← Markdown原稿
images/            ← cover.jpgと挿絵
```

`book-info.md` はEPUB用の書誌情報だけを扱います。タイトル、サブタイトル、シリーズ、著者名、言語を入力でき、空欄があっても構いません。価格、キーワード、カテゴリー、権利確認、AI生成コンテンツ申告など、KDP画面で確認・入力する項目は含めません。

この配置は分かりやすくするための初期形です。素材はルート直下でも、原稿と画像が同じ場所でも構いません。`manuscript.md` 1ファイル、`chapter01.md` のような命名にも対応します。曖昧さはCodexが内容まで見て判断し、安全に決められない場合だけ確認します。

## 作品情報の入力

最小限、次の2項目を `book-info.md` へ入力すると確実です。

```text
- タイトル: 作品タイトル
- 著者名: 著者名またはペンネーム
```

それ以外のKDP登録情報は、Codexが原稿から候補を作って `release/` の整理資料へ出力します。権利やAI申告など推測できないものは、登録前の確認事項として残します。

## 出力

```text
release/
  book.epub
  cover.jpg
  kdp-metadata.md
  description.txt
  keywords.txt
  categories.md
  validation-report.md
  publishing-checklist.md
  build-config.json
```

`validation-report.md` には成功項目、自動修正、警告、ユーザー確認事項、エラーが分かれて記録されます。

## 何をするエージェントか

```text
素材を理解する
  → 原稿・表紙・挿絵・章順を判断する
  → Builder設定を作る
  → EPUB 3を決定論的に生成する
  → 内部構造と参照を検証する
  → KDP登録情報とチェックリストを用意する
```

- Agent: 素材の理解、曖昧さの解消、文案、設定補正、エラー対応
- Builder: MarkdownからXHTML、CSS、目次、OPF、OCF ZIPを生成
- Validator: EPUB必須構造、metadata、manifest、spine、navigation、XHTML、CSS、画像、MIME type、リンク切れを検査
- Kindle Previewer: Amazon変換環境での最終表示確認

## 現行仕様についての前提（2026-09-02確認）

- 現行のW3C勧告はEPUB 3.3です。パッケージ文書の `version` 属性は仕様どおり `3.0` であり、`3.3` と書くものではありません。EPUBは既定でリフロー型で、Navigation Documentが必須です。[W3C EPUB 3.3](https://www.w3.org/TR/epub-33/)
- KDPはガイドラインに適合したEPUBを電子書籍原稿として受け付け、アップロード前のKindle Previewer検証を推奨しています。[KDP対応ファイル形式](https://kdp.amazon.com/en_US/help/topic/G200634390)
- 販売ページ用表紙はEPUBとは別に必要で、推奨は1600×2560 px、RGB、JPEGです。EPUB内にも `cover-image` を指定しますが、重複表示を避けるため別のHTML表紙ページは作りません。[KDP Cover Image Guidelines](https://kdp.amazon.com/en_US/help/topic/G6GTK3T3NUHKLEFX)
- KDPはAI生成の本文・画像・翻訳について申告を求め、AI支援のみの場合は申告不要と区別しています。本ツールは区分を推測せず整理票へ残します。[KDP Content Guidelines](https://kdp.amazon.com/en_US/help/topic/G200672390)
- 現在の登録上限はキーワード候補7件、カテゴリー3件、商品説明4000文字です。[キーワード](https://kdp.amazon.com/en_US/help/topic/G201743260)・[カテゴリー](https://kdp.amazon.com/en_US/help/topic/G200652170)・[商品説明](https://kdp.amazon.com/en_US/help/topic/G201189630)
- Kindle Previewerは日本語EPUBを表示できます。ただしAmazonの現行Enhanced Typesetting対応言語一覧に日本語は含まれていないため、本ツールはその機能に依存しません。[Kindle Previewer](https://kdp.amazon.com/en_US/help/topic/G202131170)

詳しい設計上の採否は [`docs/spec-assumptions.md`](docs/spec-assumptions.md) にまとめています。

## 対応範囲

v1は日本語横書き小説のReflowable EPUBを対象にします。

- 見出し、本文、段落、章区切り、目次
- 表紙、挿絵、代替テキスト
- 青空文庫風ルビ `｜漢字《かんじ》`、簡易ルビ `{漢字|かんじ}`
- 強調、斜体、引用、リスト、リンク
- JPEG、PNG、GIF、原稿から参照されるSVG

漫画、Fixed Layout EPUB、複雑な技術書、音声・動画、高度な縦書き、DRM、Amazonへの自動投稿はv1対象外です。

## セキュリティとプライバシー

Builderはネットワークへ接続せず、指定した素材フォルダ外のファイルをEPUBへ取り込みません。元原稿と元画像は変更しません。Amazonアカウント情報も扱いません。

## 開発者向け

利用者には不要です。ソースからの開発にはGo 1.22以降を使います。

```text
go test ./...
go build ./cmd/book-builder
```

全OS向けバイナリは `scripts/build-all.ps1` または `scripts/build-all.sh` で `bin/` へ出力します。設定JSONは [`docs/config-reference.md`](docs/config-reference.md) を参照してください。

## ライセンス

MIT License
