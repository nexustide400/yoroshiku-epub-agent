# Builder設定リファレンス

AgentとBuilderの境界はJSONファイルです。`scan` が下書きを作り、Agentが原稿と画像を読んで補正し、`build` が決定論的に処理します。パスは原則として `inputDir` からの相対パスです。

通常の利用者はJSONを直接編集する必要はありません。配布ルートの `book-info.md` へ分かる情報だけ入力すると、`scan` がEPUB用の次の項目を設定へ取り込みます。

- タイトル
- サブタイトル
- シリーズ名・シリーズ番号
- 著者名
- 言語

KDP画面で入力する商品説明、キーワード、カテゴリー、価格、権利確認、AI生成コンテンツ申告などは `book-info.md` の対象外です。それらはAgentが原稿から候補を作るか、`release/` の確認事項へ残します。

```json
{
  "version": 1,
  "inputDir": "C:/books/my-novel",
  "outputDir": "C:/books/my-novel/release",
  "book": {
    "title": "作品名",
    "subtitle": "",
    "series": "シリーズ名",
    "seriesIndex": "1",
    "creator": "著者名",
    "language": "ja",
    "identifier": "urn:uuid:...",
    "publisher": "",
    "description": "EPUB内メタデータ用の説明",
    "rights": "Copyright ...",
    "modified": "2026-09-02T00:00:00Z",
    "writingMode": "horizontal-tb"
  },
  "cover": {
    "path": "cover.jpg",
    "alt": "作品名 表紙"
  },
  "sections": [
    {
      "path": "manuscript/001.md",
      "title": "第一章",
      "level": 1,
      "images": [
        {
          "path": "images/001.jpg",
          "alt": "挿絵の内容を短く説明",
          "caption": "",
          "afterParagraph": 3
        }
      ]
    }
  ],
  "kdp": {
    "description": "KDP商品説明文",
    "keywords": ["候補1", "候補2"],
    "categories": ["文学・評論 > 小説"],
    "primaryMarket": "Amazon.co.jp",
    "rightsConfirmed": true,
    "publicDomain": false,
    "aiGeneratedText": "none",
    "aiGeneratedCover": "none",
    "aiGeneratedArt": "none",
    "aiTranslation": "none",
    "selectIntent": "未定。電子版独占条件を確認して判断",
    "priceMemo": "競合作品と配信コストを確認して決定",
    "audienceMemo": "一般向け"
  },
  "decisions": [],
  "warnings": []
}
```

## 挿絵

- 原稿中の標準Markdown画像 `![代替テキスト](../images/001.jpg)` は、その位置へ挿入されます。
- `sections[].images[]` は、原稿に画像記法がない場合にAgentが挿入位置を指定するために使います。
- `afterParagraph` が正なら、その本文段落の直後へ挿入します。`0` は章末です。
- 同じ画像を原稿記法と `sections[].images[]` の両方へ指定しないでください。
- `alt` は空にせず、装飾画像でも意図をAgentが判断してください。

## 対応Markdown（v1）

- ATX見出し（`#`〜`######`）
- 段落、改行、区切り線、引用、順序付き/なしリスト
- 強調、斜体、インラインコード、リンク、画像
- 青空文庫風ルビ `｜漢字《かんじ》`
- 簡易ルビ `{漢字|かんじ}`
- 安全な単純HTMLルビ `<ruby>漢字<rt>かんじ</rt></ruby>`

任意のHTMLは通過させません。v1は日本語小説の安全で予測可能な変換を優先します。
