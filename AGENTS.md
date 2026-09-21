# Yoroshiku EPUB Agent

このリポジトリを開いたCodexは、ユーザーが「KDP出版用によろしく」などと依頼したら、素材の理解・整理、Reflowable EPUB 3生成、機械検証、KDP登録情報の準備まで担当する。Amazonへのログイン、登録、公開操作はしない。

## 最優先のUX

- ユーザーへPython、Node.js、Java、Go、パッケージマネージャー等の導入を求めない。
- `bin/` のOS・CPU別単体実行ファイルを使う。開発用ソースやビルド手順は利用者向けフローに持ち込まない。
- 素材配置を厳格に要求しない。ファイル名、本文、画像内容、作品情報から合理的に判断する。
- 配布ZIPにある `book-info.md`、`manuscript/`、`images/` を標準の入力口として案内する。ただし利用を強制せず、ルート直下や別名の素材にも対応する。
- 小さな不整合では止まらない。安全な正規化、設定補正、再ビルドを行い、判断と修正はレポートへ残す。
- 権利保有、パブリックドメイン、AI生成コンテンツ区分は推測しない。EPUB生成を妨げなければ「要確認」としてレポートへ残す。著者名・表紙・原稿順など成果物を誤らせる曖昧さだけを、必要事項をまとめてユーザーへ確認する。
- 元原稿と元画像は上書きしない。正規化は生成物の中で行い、Agentの判断は `.kdp-work/build-config.json` に記録する。

## Builderの選択

現在のOSとCPUに合うものを選ぶ。

- Windows x64: `bin/book-builder-windows-amd64.exe`
- Windows ARM64: `bin/book-builder-windows-arm64.exe`
- macOS Apple Silicon: `bin/book-builder-macos-arm64`
- macOS Intel: `bin/book-builder-macos-amd64`
- Linux x64: `bin/book-builder-linux-amd64`
- Linux ARM64: `bin/book-builder-linux-arm64`

実行権限がないUnix系では `chmod +x` を行ってよい。対応バイナリが本当に欠損・破損している場合だけ状況を説明し、ユーザーに開発環境の導入を求めず、GitHub Releasesの完成済みZIPを案内する。

## 標準ワークフロー

1. フォルダ全体を列挙する。`bin/`, `cmd/`, `internal/`, `templates/`, `docs/`, `examples/`, `release/`, `.git/`, `.github/`, `.kdp-work/` は出版素材候補から除外する。標準入力口は `book-info.md`、`manuscript/`、`images/` だが、素材が他の場所にあっても探索する。
2. Markdown、JPEG/PNG/GIF、作品情報らしいファイルを読む。`book-info.md` はEPUB用のタイトル、サブタイトル、シリーズ、著者、言語だけを入力する簡潔な書誌情報ファイルとして扱い、その入力値を優先する。価格、キーワード、カテゴリー、権利確認、AI申告などKDP画面側の項目を `book-info.md` へ追加させない。空欄は未指定であり、文字列「空欄」や仮の値として扱わない。画像は可能なら視覚的にも確認し、表紙、挿絵、装飾画像を分類する。
3. Builderで下書き設定を作る。

   ```text
   <builder> scan --input . --config .kdp-work/build-config.json
   ```

4. `.kdp-work/build-config.json` と実際の素材を突き合わせ、必ずAgentが補正する。特に以下を確認する。

   - 原稿の採用範囲と自然な章順
   - 最上位見出しまたは内容に基づく各章タイトル
   - 表紙が1点に特定できること
   - 原稿内Markdown画像と追加挿絵が重複しないこと
   - ファイル番号、章内容、画像内容による挿絵対応
   - 画像ごとの具体的な代替テキスト
   - `book-info.md` と原稿の整合、およびタイトル、サブタイトル、シリーズ、著者、言語
   - OPF主タイトルが `<dc:title id="title">…</dc:title>` と、それを参照する `<meta refines="#title" property="title-type">main</meta>` の1組で一意に指定されること。サブタイトルがある場合は `id="subtitle"` と `title-type=subtitle` の組にすること
   - 商品説明文、最大7件の具体的なキーワード候補、最大3件の正確なカテゴリー候補
   - 権利・AI申告・KDP Select・価格に関する既知情報と要確認事項

5. 商品説明やキーワードは原稿を読んで内容に忠実な候補を作る。誇張、レビュー文、価格、連絡先、URL、無関係な検索語を混ぜない。カテゴリーは登録時に変わり得るため候補として扱う。
6. 設定を保存してビルドする。

   ```text
   <builder> build --config .kdp-work/build-config.json
   ```

7. `release/validation-report.md` を読み、`OPF-TITLE-MAIN` の成功項目を含めて主タイトルの一意性と `title-type=main` の参照を確認する。エラーまたは自動修正可能な警告があれば、設定を直して再ビルドする。Kindle Previewerで `E20006` または `E21011` が出た場合は、`package.opf` の主タイトルと `title-type=main` を修正し、Builderから再生成する。機械的な失敗に対して最大3回は原因を調べて修正を試みる。
   `epubcheck` が既に利用可能なら追加検証を実行して結果をレポートへ追記してよいが、JavaやEPUBCheckの導入をユーザーへ要求しない。
8. 成功時は `release/` の成果物を確認し、ユーザーへ次の3点を簡潔に伝える。

   - `release/book.epub` が生成・内蔵検証済みであること
   - 残る警告と人間が決める事項
   - Kindle Previewerで確認してからKDPへ `book.epub` と `cover.jpg` をアップロードすること

## 判断基準

- 原稿候補: `.md` / `.markdown`。README、AGENTS、変更履歴、作品情報ファイルは本文から除く。
- 作品情報: `book-info.md` の値を最優先する。未入力項目は原稿から補完候補を作るが、権利・パブリックドメイン・AI生成区分は推測しない。
- 章順: 明示的な作品情報、本文内の部・章番号、自然順ファイル名、物語上の連続性の順で根拠を重く見る。
- 表紙: `cover`, `hyoshi`, `表紙` の名前、縦長比率、タイトル・著者表記、解像度を合わせて判断する。表紙を本文挿絵として重複登録しない。
- 挿絵: 原稿内の明示リンクを最優先する。次に完全な章番号一致、内容一致、並び順を使う。安全に対応できない画像は勝手に章へ入れず、警告または確認事項にする。
- 画像が推奨寸法より小さい場合は引き伸ばさない。巨大画像もv1では非破壊で組み込み、警告する。
- MarkdownのBOM、改行コード、前付けメタデータ、空白、軽微な記法揺れは生成時に正規化する。
- サポート範囲と設定スキーマは `docs/config-reference.md` を参照する。

## 完了条件

- `release/book.epub` が存在し、内蔵Validatorのエラーが0件。
- `release/cover.jpg`, `kdp-metadata.md`, `description.txt`, `keywords.txt`, `categories.md`, `validation-report.md`, `publishing-checklist.md` が存在する。
- `book-info.md` に入力されたEPUB用のタイトル・著者などが `.kdp-work/build-config.json` と生成EPUBへ正しく反映されている。
- `package.opf` に空でない主タイトルが1件あり、その `dc:title` のIDを、ただ1件の `<meta property="title-type">main</meta>` が正しく参照している。サブタイトルがある場合は `title-type=subtitle` で主タイトルと区別されている。
- manifest、spine、navigation、metadata、XHTML、CSS、画像、MIME type、リンク、表紙の検査結果がレポートに記録されている。
- ユーザー確認事項を隠さない。内蔵検証の合格をKindle上の表示保証とは表現しない。

## v1の境界

対象は日本語横書き小説を中心とするReflowable EPUB。漫画、Fixed Layout、複雑な技術書、音声・動画、高度な縦書き、DRM、Amazon自動投稿は対象外。対象外の素材を検出した場合は、無理に変換せず理由と代替案を示す。
