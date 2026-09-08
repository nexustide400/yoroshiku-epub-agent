package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"yoroshiku-epub-agent/internal/model"
)

func WriteRelease(cfg model.Config, result *model.Result) error {
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return err
	}
	description := firstNonEmpty(cfg.KDP.Description, cfg.Book.Description)
	if description == "" {
		description = "【要作成】原稿の内容に即した商品説明文を入力してください。"
		result.Confirm("KDP-DESCRIPTION", "商品説明文が未確定です。KDP登録前に作成・確認してください。")
	} else if utf8.RuneCountInString(description) > 4000 {
		result.Warn("KDP-DESCRIPTION-LENGTH", fmt.Sprintf("商品説明文は%d文字です。KDP上限4000文字以内へ短縮してください。", utf8.RuneCountInString(description)))
	} else {
		result.Pass("KDP-DESCRIPTION", fmt.Sprintf("商品説明文は%d文字です。", utf8.RuneCountInString(description)))
	}
	keywords := append([]string(nil), cfg.KDP.Keywords...)
	if len(keywords) > 7 {
		keywords = keywords[:7]
		result.Fix("KDP-KEYWORDS-LIMIT", "キーワード候補をKDP上限の7件に絞りました。")
	}
	if len(keywords) == 0 {
		result.Confirm("KDP-KEYWORDS", "検索キーワード候補が未確定です（最大7件）。")
	}
	categories := append([]string(nil), cfg.KDP.Categories...)
	if len(categories) > 3 {
		categories = categories[:3]
		result.Fix("KDP-CATEGORIES-LIMIT", "カテゴリー候補をKDP選択上限の3件に絞りました。")
	}
	if len(categories) == 0 {
		result.Confirm("KDP-CATEGORIES", "カテゴリー候補が未確定です（最大3件、登録時のマーケットで再確認）。")
	}
	addConfirmations(cfg, result)

	files := map[string]string{
		"description.txt":         description + "\n",
		"keywords.txt":            listOrTODO(keywords, "【要選定】読者が検索に使う具体的な語句を最大7件") + "\n",
		"categories.md":           categoriesMarkdown(categories),
		"kdp-metadata.md":         metadataMarkdown(cfg, description, keywords, categories),
		"publishing-checklist.md": checklistMarkdown(cfg, result),
		"validation-report.md":    ValidationMarkdown(result),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(cfg.OutputDir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	configJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	configJSON = append(configJSON, '\n')
	return os.WriteFile(filepath.Join(cfg.OutputDir, "build-config.json"), configJSON, 0o644)
}

func WriteValidation(path string, result model.Result) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(ValidationMarkdown(&result)), 0o644)
}

func ValidationMarkdown(result *model.Result) string {
	var out strings.Builder
	out.WriteString("# EPUB検証レポート\n\n")
	status := "合格"
	if len(result.Errors) > 0 {
		status = "不合格（修正が必要）"
	}
	out.WriteString("総合判定: **" + status + "**\n\n")
	writeFindings(&out, "正常に処理できた項目", result.Passed, "該当なし")
	writeFindings(&out, "自動修正した項目", result.Fixed, "なし")
	writeFindings(&out, "警告", result.Warnings, "なし")
	writeFindings(&out, "ユーザー確認が必要な項目", result.Confirmations, "なし")
	writeFindings(&out, "エラー", result.Errors, "なし")
	out.WriteString("## 検証範囲\n\n内蔵ValidatorはOCF/ZIP、container.xml、OPF metadata・manifest・spine、EPUB Navigation Document、XHTML XML整形式、CSS、MIME type、画像・内部リンク・フラグメント、表紙指定を検査します。Kindle変換後の見た目はKindle Previewerで確認してください。\n")
	return out.String()
}

func writeFindings(out *strings.Builder, heading string, findings []model.Finding, empty string) {
	out.WriteString("## " + heading + "\n\n")
	if len(findings) == 0 {
		out.WriteString("- " + empty + "\n\n")
		return
	}
	for _, finding := range findings {
		out.WriteString("- `" + finding.Code + "` " + finding.Message + "\n")
	}
	out.WriteByte('\n')
}

func metadataMarkdown(cfg model.Config, description string, keywords, categories []string) string {
	var out strings.Builder
	out.WriteString("# KDP登録情報\n\n")
	out.WriteString("> これは登録候補の整理票です。KDP画面・表紙・本文の表記を一致させ、最終判断は権利者が行ってください。\n\n")
	field(&out, "タイトル", cfg.Book.Title)
	field(&out, "サブタイトル", cfg.Book.Subtitle)
	field(&out, "シリーズ", joinNonEmpty(cfg.Book.Series, cfg.Book.SeriesIndex))
	field(&out, "著者名", cfg.Book.Creator)
	field(&out, "言語", cfg.Book.Language)
	field(&out, "出版社名", cfg.Book.Publisher)
	field(&out, "主なマーケット", cfg.KDP.PrimaryMarket)
	out.WriteString("## 商品説明文\n\n" + description + "\n\n")
	out.WriteString("## キーワード候補（最大7件）\n\n" + markdownList(keywords, "【要選定】") + "\n")
	out.WriteString("## カテゴリー候補（最大3件）\n\n" + markdownList(categories, "【要選定】") + "\n")
	out.WriteString("## 出版権\n\n")
	out.WriteString("- 権利保有確認: " + boolStatus(cfg.KDP.RightsConfirmed) + "\n")
	out.WriteString("- パブリックドメイン: " + boolStatus(cfg.KDP.PublicDomain) + "\n")
	field(&out, "権利メモ", cfg.Book.Rights)
	out.WriteString("## AI生成コンテンツ申告整理\n\n")
	out.WriteString("- 本文: " + unknown(cfg.KDP.AIGeneratedText) + "\n")
	out.WriteString("- 表紙: " + unknown(cfg.KDP.AIGeneratedCover) + "\n")
	out.WriteString("- 挿絵: " + unknown(cfg.KDP.AIGeneratedArt) + "\n")
	out.WriteString("- 翻訳: " + unknown(cfg.KDP.AITranslation) + "\n\n")
	field(&out, "KDP Select検討", cfg.KDP.SelectIntent)
	field(&out, "価格設定メモ", cfg.KDP.PriceMemo)
	field(&out, "対象読者メモ", cfg.KDP.AudienceMemo)
	return out.String()
}

func categoriesMarkdown(categories []string) string {
	return "# KDPカテゴリー候補\n\n" + markdownList(categories, "【要選定】作品内容に正確なカテゴリーをKDP画面で最大3件選択") + "\n> KDPのカテゴリー構成はマーケットや時期により変わるため、登録時に候補名を再確認してください。\n"
}

func checklistMarkdown(cfg model.Config, result *model.Result) string {
	checked := func(ok bool) string {
		if ok {
			return "[x]"
		}
		return "[ ]"
	}
	return `# KDP出版チェックリスト

## 成果物

- ` + checked(result.Valid()) + ` 内蔵Validatorでbook.epubが合格
- [ ] Kindle PreviewerでEPUBを開き、エラー・警告を確認
- [ ] 端末種別、縦横、文字サイズ、白/黒系背景で本文を確認
- [ ] 目次の全リンクと「移動」メニューを確認
- [ ] 表紙と全挿絵を目視確認

## KDP登録内容

- ` + checked(cfg.Book.Title != "") + ` タイトル確定
- ` + checked(cfg.Book.Creator != "") + ` 著者名確定
- ` + checked(firstNonEmpty(cfg.KDP.Description, cfg.Book.Description) != "") + ` 商品説明文確定（4000文字以内）
- ` + checked(len(cfg.KDP.Keywords) > 0 && len(cfg.KDP.Keywords) <= 7) + ` キーワード最大7件を確定
- ` + checked(len(cfg.KDP.Categories) > 0 && len(cfg.KDP.Categories) <= 3) + ` カテゴリー最大3件を登録画面で確定
- [ ] KDP画面、表紙、EPUB内のタイトル・著者・シリーズ表記を一致させた

## 権利・申告

- ` + checked(cfg.KDP.RightsConfirmed != nil && *cfg.KDP.RightsConfirmed) + ` 本文・表紙・挿絵・フォント等の出版権を確認
- ` + checked(aiKnown(cfg.KDP.AIGeneratedText) && aiKnown(cfg.KDP.AIGeneratedCover) && aiKnown(cfg.KDP.AIGeneratedArt) && aiKnown(cfg.KDP.AITranslation)) + ` AI生成コンテンツ申告を本文・表紙・挿絵・翻訳ごとに確認
- [ ] 成人向け、対象年齢、パブリックドメイン等の該当項目を確認

## 販売条件

- [ ] 主なマーケット、価格、ロイヤリティ条件、配信地域を確認
- [ ] KDP Selectの90日間の電子版独占条件を理解して参加可否を決定
- [ ] 最終プレビュー後にKDPへbook.epubとcover.jpgをアップロード
`
}

func addConfirmations(cfg model.Config, result *model.Result) {
	if cfg.KDP.RightsConfirmed == nil || !*cfg.KDP.RightsConfirmed {
		result.Confirm("KDP-RIGHTS", "本文・表紙・挿絵など全素材の出版権を権利者が確認してください。")
	}
	for label, value := range map[string]string{"本文": cfg.KDP.AIGeneratedText, "表紙": cfg.KDP.AIGeneratedCover, "挿絵": cfg.KDP.AIGeneratedArt, "翻訳": cfg.KDP.AITranslation} {
		if !aiKnown(value) {
			result.Confirm("KDP-AI-DISCLOSURE", label+"のAI生成コンテンツ申告区分が未確定です。")
		}
	}
	if strings.TrimSpace(cfg.KDP.PriceMemo) == "" {
		result.Confirm("KDP-PRICE", "希望価格、ロイヤリティ条件、配信コストをKDP登録時に確認してください。")
	}
	if strings.TrimSpace(cfg.KDP.SelectIntent) == "" {
		result.Confirm("KDP-SELECT", "KDP Selectの90日間の電子版独占条件を確認し、参加可否を決めてください。")
	}
}

func field(out *strings.Builder, name, value string) {
	if strings.TrimSpace(value) == "" {
		value = "【要確認】"
	}
	out.WriteString("- " + name + ": " + value + "\n")
}

func markdownList(values []string, empty string) string {
	if len(values) == 0 {
		return "- " + empty + "\n"
	}
	var out strings.Builder
	for _, value := range values {
		out.WriteString("- " + value + "\n")
	}
	return out.String()
}

func listOrTODO(values []string, todo string) string {
	if len(values) == 0 {
		return todo
	}
	return strings.Join(values, "\n")
}

func boolStatus(value *bool) string {
	if value == nil {
		return "【要確認】"
	}
	if *value {
		return "はい"
	}
	return "いいえ"
}

func unknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "【要確認】"
	}
	return value
}

func aiKnown(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "no", "generated", "assisted", "none", "あり", "なし", "ai生成", "ai支援":
		return true
	default:
		return false
	}
}

func joinNonEmpty(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " / " + b
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
