# KDP / EPUB 3 仕様前提

確認日: 2026-09-02

## 採用した前提

1. **出力はReflowable EPUB 3**
   W3C EPUB 3.3はEPUBの既定をリフロー型とし、OCFコンテナ、Package Document、manifest、spine、XHTML Content Documents、EPUB Navigation Documentを中核構造とします。本Builderはこの著者要件に合わせ、Package Documentの規定値 `version="3.0"` を出力します。

   Kindle側で主タイトルを確実に解決できるよう、主 `dc:title` には `id="title"` を付け、直後のrefinementで `title-type=main` を明示します。サブタイトルがある場合は別の `dc:title id="subtitle"` を `title-type=subtitle` として区別します。内蔵Validatorも主タイトルrefinementが1件だけ存在し、空でないタイトルIDを参照することを必須検査します。

2. **KDPへはEPUBを直接アップロード**
   KDPはEPUBをサポートし、Kindle Previewerで検証してからのアップロードを推奨しています。MOBIを主出力にはしません。

3. **表紙には2つの役割がある**
   KDP販売ページ用表紙はEPUB外で別途アップロードします。同じ元画像をRGB JPEGへ変換して `release/cover.jpg` とし、EPUB内ではOPF manifestの `properties="cover-image"` で指定します。Amazonのガイドに従い、重複するHTML表紙ページはspineへ入れません。

4. **表紙は引き伸ばさない**
   KDPのKindle Publishing Guidelines内の具体的な表紙ページが推奨する1600×2560 px、RGB、JPEG、5MB以下を警告基準にします。小さい画像を機械的に拡大すると品質が下がるため、再制作の要否をユーザーへ残します。

   KDPの別の表紙ヘルプには「50MB未満」「72 dpi」と書かれた箇所もあり、現行ページ間で数値が一致していません。本Builderはより厳しいKindle Publishing Guidelines側（5MB、300 PPI）を採用し、最終的にはKDPアップロード画面の判定を優先します。

5. **日本語でEnhanced Typesettingへ依存しない**
   Kindle Previewer自体は日本語をサポートしますが、現行ヘルプのEnhanced Typesetting対応言語一覧には日本語がありません。基本的なXHTML/CSSと端末既定の組版だけで成立する設計にします。

6. **KDP登録情報は候補と確認事項**
   商品説明、キーワード、カテゴリーは原稿からAgentが候補を作れます。一方、出版権、AI生成区分、成人向け区分、KDP Select、価格は事実や契約判断を含むため、未提供時に確定値を捏造しません。

7. **内蔵ValidatorとKindle Previewerの役割を分ける**
   Javaなしで構造・XML・参照・MIME type等を検査します。Amazon独自変換後の表示、背景色、端末差、Enhanced Typesetting可否はKindle Previewerで最終確認します。EPUBCheckが別途使える環境では追加検査として利用できますが必須にはしません。

## 公式資料

- [W3C EPUB 3.3](https://www.w3.org/TR/epub-33/)
- [KDP: Supported eBook manuscript formats](https://kdp.amazon.com/en_US/help/topic/G200634390)
- [KDP: Kindle Publishing Guidelines](https://kdp.amazon.com/en_US/help/topic/GU72M65VRFPH43L6)
- [KDP: Cover Image Guidelines](https://kdp.amazon.com/en_US/help/topic/G6GTK3T3NUHKLEFX)
- [KDP: eBook cover criteria](https://kdp.amazon.com/en_US/help/topic/G200645690)
- [KDP: Content Guidelines / AI content](https://kdp.amazon.com/en_US/help/topic/G200672390)
- [KDP: Kindle Previewer](https://kdp.amazon.com/en_US/help/topic/G202131170)
- [KDP: Keywords](https://kdp.amazon.com/en_US/help/topic/G201298500)
- [KDP: Categories](https://kdp.amazon.com/en_US/help/topic/G200652170)
- [KDP: Book description](https://kdp.amazon.com/en_US/help/topic/G201189630)
- [KDP Select requirements](https://kdp.amazon.com/en_US/help/topic/GD9PMU58BV24QFZ7)
