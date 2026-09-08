package epub

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	bookmd "yoroshiku-epub-agent/internal/markdown"
	"yoroshiku-epub-agent/internal/model"
)

const defaultCSS = `@charset "UTF-8";
html {
  -webkit-writing-mode: horizontal-tb;
  writing-mode: horizontal-tb;
}
body {
  font-family: serif;
  line-height: 1.8;
  margin: 0 5%;
  padding: 0;
  text-align: justify;
  overflow-wrap: break-word;
}
h1, h2, h3, h4, h5, h6 {
  font-family: sans-serif;
  font-weight: normal;
  line-height: 1.4;
  margin: 2em 0 1.2em;
  text-align: start;
}
h1 { font-size: 1.5em; page-break-before: always; break-before: page; }
h2 { font-size: 1.25em; }
p { margin: 0; text-indent: 1em; }
p + p { margin-top: 0.35em; }
blockquote { margin: 1em 1.5em; }
blockquote p, li p, figcaption { text-indent: 0; }
hr { border: 0; height: 0; margin: 2em 0; page-break-after: always; break-after: page; }
figure { margin: 1.5em 0; page-break-inside: avoid; break-inside: avoid; text-align: center; }
img { height: auto; max-width: 100%; }
.inline-image { max-height: 1.2em; vertical-align: middle; }
figcaption { font-size: 0.85em; margin-top: 0.5em; text-align: center; }
ruby rt { font-size: 0.55em; }
nav ol { padding-inline-start: 1.5em; }
.missing-image { border: 1px solid currentColor; padding: 1em; text-indent: 0; }
`

type fileEntry struct {
	Name string
	Data []byte
}

type asset struct {
	ID        string
	AbsPath   string
	EPUBPath  string
	MediaType string
	Data      []byte
}

type assetRegistry struct {
	inputDir string
	byPath   map[string]*asset
	byDigest map[string]*asset
	assets   []*asset
}

func Build(cfg model.Config) (string, model.Result, error) {
	cfg.ApplyDefaults()
	result := model.Result{}
	if err := cfg.Validate(); err != nil {
		return "", result, err
	}
	inputAbs, err := filepath.Abs(cfg.InputDir)
	if err != nil {
		return "", result, err
	}
	outputAbs, err := filepath.Abs(cfg.OutputDir)
	if err != nil {
		return "", result, err
	}
	if err := os.MkdirAll(outputAbs, 0o755); err != nil {
		return "", result, err
	}

	coverAbs, err := safeInputPath(inputAbs, cfg.Cover.Path)
	if err != nil {
		return "", result, fmt.Errorf("表紙: %w", err)
	}
	coverJPEG, coverWidth, coverHeight, err := coverAsJPEG(coverAbs)
	if err != nil {
		return "", result, fmt.Errorf("表紙をJPEGへ変換できません: %w", err)
	}
	if coverWidth < 1600 || coverHeight < 2560 {
		result.Warn("KDP-COVER-RECOMMENDED", fmt.Sprintf("表紙は %d×%d px です。KDP推奨の1600×2560 px未満です（拡大はしていません）。", coverWidth, coverHeight))
	} else {
		result.Pass("KDP-COVER-SIZE", fmt.Sprintf("表紙サイズ %d×%d px はKDP推奨寸法以上です。", coverWidth, coverHeight))
	}
	if coverHeight <= coverWidth {
		result.Warn("KDP-COVER-RATIO", "表紙が縦長ではありません。販売ページと端末で意図した表示になるか確認してください。")
	}
	if len(coverJPEG) > 5*1024*1024 {
		result.Warn("KDP-COVER-FILE-SIZE", "販売用cover.jpgが5MBを超えています。KDPの現行推奨値を確認してください。")
	}

	registry := &assetRegistry{inputDir: inputAbs, byPath: map[string]*asset{}, byDigest: map[string]*asset{}}
	css := loadCSS(inputAbs)
	var entries []fileEntry
	var sections []string
	for i, section := range cfg.Sections {
		sectionAbs, err := safeInputPath(inputAbs, section.Path)
		if err != nil {
			return "", result, fmt.Errorf("原稿 %s: %w", section.Path, err)
		}
		data, err := os.ReadFile(sectionAbs)
		if err != nil {
			return "", result, fmt.Errorf("原稿を読めません (%s): %w", section.Path, err)
		}
		resolver := func(sourcePath, alt string) (string, error) {
			item, err := registry.add(sourcePath)
			if err != nil {
				return "", err
			}
			return "../images/" + filepath.Base(item.EPUBPath), nil
		}
		rendered := bookmd.Render(string(data), section.Title, sectionAbs, section.Images, resolver)
		for _, warning := range bookmd.SortedWarnings(rendered) {
			result.Warn("MARKDOWN-IMAGE", section.Path+": "+warning)
		}
		sort.Strings(rendered.Errors)
		for _, renderErr := range rendered.Errors {
			result.Error("MARKDOWN-IMAGE-MISSING", section.Path+": "+renderErr)
		}
		name := fmt.Sprintf("EPUB/text/section-%03d.xhtml", i+1)
		entries = append(entries, fileEntry{Name: name, Data: []byte(contentDocument(cfg.Book.Language, section.Title, rendered.Body))})
		sections = append(sections, name)
	}

	coverAsset := &asset{ID: "cover-image", AbsPath: coverAbs, EPUBPath: "EPUB/images/cover.jpg", MediaType: "image/jpeg", Data: coverJPEG}
	allAssets := append([]*asset{coverAsset}, registry.assets...)
	for _, item := range registry.assets {
		cfg, _, decodeErr := image.DecodeConfig(bytes.NewReader(item.Data))
		if decodeErr == nil && (cfg.Width > 10000 || cfg.Height > 10000 || len(item.Data) > 10*1024*1024) {
			result.Warn("KDP-IMAGE-LARGE", fmt.Sprintf("%s は %d×%d px / %.1f MBです。リサイズせず組み込みました。Kindle Previewerでメモリ負荷と表示を確認してください。", filepath.ToSlash(item.AbsPath), cfg.Width, cfg.Height, float64(len(item.Data))/(1024*1024)))
		}
	}
	entries = append(entries,
		fileEntry{Name: "EPUB/css/novel.css", Data: []byte(css)},
		fileEntry{Name: "EPUB/nav.xhtml", Data: []byte(navDocument(cfg, sections))},
		fileEntry{Name: "EPUB/package.opf", Data: []byte(packageDocument(cfg, sections, allAssets))},
	)
	for _, item := range registry.assets {
		entries = append(entries, fileEntry{Name: item.EPUBPath, Data: item.Data})
	}
	entries = append(entries, fileEntry{Name: coverAsset.EPUBPath, Data: coverJPEG})
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	epubPath := filepath.Join(outputAbs, "book.epub")
	if err := writeEPUB(epubPath, entries); err != nil {
		return "", result, err
	}
	if err := os.WriteFile(filepath.Join(outputAbs, "cover.jpg"), coverJPEG, 0o644); err != nil {
		return "", result, err
	}
	result.Fix("COVER-RGB-JPEG", "元表紙を白背景のRGB JPEG（300 PPI記録）として決定論的に再エンコードし、EPUB内部表紙と販売用cover.jpgを生成しました。")
	for _, warning := range cfg.Warnings {
		result.Warn("AGENT-SCAN", warning)
	}
	validation, err := Validate(epubPath)
	if err != nil {
		return epubPath, result, err
	}
	mergeResult(&result, validation)
	return epubPath, result, nil
}

func (r *assetRegistry) add(source string) (*asset, error) {
	abs, err := safeInputPath(r.inputDir, source)
	if err != nil {
		return nil, fmt.Errorf("画像 %s: %w", source, err)
	}
	key := strings.ToLower(filepath.Clean(abs))
	if existing := r.byPath[key]; existing != nil {
		return existing, nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("画像を読めません (%s): %w", source, err)
	}
	mediaType := detectImageMediaType(data, abs)
	if !strings.HasPrefix(mediaType, "image/") {
		return nil, fmt.Errorf("未対応の画像形式です: %s", source)
	}
	ext := canonicalImageExtension(abs, mediaType)
	hash := sha256.Sum256(data)
	stem := "img-" + hex.EncodeToString(hash[:6])
	if existing := r.byDigest[stem]; existing != nil {
		r.byPath[key] = existing
		return existing, nil
	}
	item := &asset{ID: stem, AbsPath: abs, EPUBPath: "EPUB/images/" + stem + ext, MediaType: mediaType, Data: data}
	r.byPath[key] = item
	r.byDigest[stem] = item
	r.assets = append(r.assets, item)
	return item, nil
}

func safeInputPath(inputDir, value string) (string, error) {
	path := filepath.FromSlash(strings.Trim(value, "<>"))
	if !filepath.IsAbs(path) {
		path = filepath.Join(inputDir, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(inputDir, abs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("入力フォルダ外のパスは使用できません: %s", value)
	}
	return abs, nil
}

func coverAsJPEG(path string) ([]byte, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return nil, 0, 0, err
	}
	bounds := img.Bounds()
	canvas := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(canvas, canvas.Bounds(), img, bounds.Min, draw.Over)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, canvas, &jpeg.Options{Quality: 94}); err != nil {
		return nil, 0, 0, err
	}
	encoded := setJPEGDensity300(out.Bytes())
	return encoded, bounds.Dx(), bounds.Dy(), nil
}

func setJPEGDensity300(data []byte) []byte {
	// JFIF stores density as unit + two big-endian uint16 values. Go's encoder
	// may omit APP0, so add a minimal segment when one is not already present.
	if len(data) >= 18 && bytes.Equal(data[:4], []byte{0xff, 0xd8, 0xff, 0xe0}) && string(data[6:11]) == "JFIF\x00" {
		data[13] = 1
		data[14], data[15] = 0x01, 0x2c
		data[16], data[17] = 0x01, 0x2c
		return data
	}
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
		return data
	}
	app0 := []byte{0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x01, 0x01, 0x2c, 0x01, 0x2c, 0x00, 0x00}
	withJFIF := make([]byte, 0, len(data)+len(app0))
	withJFIF = append(withJFIF, data[:2]...)
	withJFIF = append(withJFIF, app0...)
	withJFIF = append(withJFIF, data[2:]...)
	return withJFIF
}

func loadCSS(inputDir string) string {
	for _, candidate := range []string{
		filepath.Join(inputDir, "templates", "novel.css"),
		filepath.Join(inputDir, "novel.css"),
	} {
		if data, err := os.ReadFile(candidate); err == nil && len(bytes.TrimSpace(data)) > 0 {
			return string(data)
		}
	}
	return defaultCSS
}

func contentDocument(language, title, body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="` + html.EscapeString(language) + `" lang="` + html.EscapeString(language) + `">
<head>
  <meta charset="UTF-8"/>
  <title>` + html.EscapeString(title) + `</title>
  <link rel="stylesheet" type="text/css" href="../css/novel.css"/>
</head>
<body epub:type="bodymatter">
<section epub:type="chapter">
` + body + `</section>
</body>
</html>
`
}

func navDocument(cfg model.Config, sectionPaths []string) string {
	var items strings.Builder
	for i, section := range cfg.Sections {
		title := section.Title
		if title == "" {
			title = fmt.Sprintf("第%d章", i+1)
		}
		items.WriteString(fmt.Sprintf("      <li><a href=\"%s\">%s</a></li>\n", strings.TrimPrefix(sectionPaths[i], "EPUB/"), html.EscapeString(title)))
	}
	first := ""
	if len(sectionPaths) > 0 {
		first = strings.TrimPrefix(sectionPaths[0], "EPUB/")
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="` + html.EscapeString(cfg.Book.Language) + `" lang="` + html.EscapeString(cfg.Book.Language) + `">
<head>
  <meta charset="UTF-8"/>
  <title>目次</title>
  <link rel="stylesheet" type="text/css" href="css/novel.css"/>
</head>
<body>
  <nav epub:type="toc" id="toc" role="doc-toc">
    <h1>目次</h1>
    <ol>
` + items.String() + `    </ol>
  </nav>
  <nav epub:type="landmarks" hidden="hidden">
    <ol>
      <li><a epub:type="bodymatter" href="` + html.EscapeString(first) + `">本文</a></li>
    </ol>
  </nav>
</body>
</html>
`
}

func packageDocument(cfg model.Config, sections []string, assets []*asset) string {
	var metadata strings.Builder
	metadata.WriteString("    <dc:identifier id=\"pub-id\">" + html.EscapeString(cfg.Book.Identifier) + "</dc:identifier>\n")
	metadata.WriteString("    <dc:title id=\"title\">" + html.EscapeString(cfg.Book.Title) + "</dc:title>\n")
	metadata.WriteString("    <meta refines=\"#title\" property=\"title-type\">main</meta>\n")
	if cfg.Book.Subtitle != "" {
		metadata.WriteString("    <dc:title id=\"subtitle\">" + html.EscapeString(cfg.Book.Subtitle) + "</dc:title>\n")
		metadata.WriteString("    <meta refines=\"#subtitle\" property=\"title-type\">subtitle</meta>\n")
	}
	metadata.WriteString("    <dc:language>" + html.EscapeString(cfg.Book.Language) + "</dc:language>\n")
	metadata.WriteString("    <dc:creator id=\"creator\">" + html.EscapeString(cfg.Book.Creator) + "</dc:creator>\n")
	metadata.WriteString("    <meta refines=\"#creator\" property=\"role\" scheme=\"marc:relators\">aut</meta>\n")
	if cfg.Book.Publisher != "" {
		metadata.WriteString("    <dc:publisher>" + html.EscapeString(cfg.Book.Publisher) + "</dc:publisher>\n")
	}
	if cfg.Book.Description != "" {
		metadata.WriteString("    <dc:description>" + html.EscapeString(cfg.Book.Description) + "</dc:description>\n")
	}
	if cfg.Book.Rights != "" {
		metadata.WriteString("    <dc:rights>" + html.EscapeString(cfg.Book.Rights) + "</dc:rights>\n")
	}
	metadata.WriteString("    <meta property=\"dcterms:modified\">" + html.EscapeString(cfg.Book.Modified) + "</meta>\n")
	metadata.WriteString("    <meta property=\"rendition:layout\">reflowable</meta>\n")
	metadata.WriteString("    <meta property=\"rendition:flow\">auto</meta>\n")
	metadata.WriteString("    <meta property=\"rendition:orientation\">auto</meta>\n")
	if cfg.Book.Series != "" {
		metadata.WriteString("    <meta property=\"belongs-to-collection\" id=\"series\">" + html.EscapeString(cfg.Book.Series) + "</meta>\n")
		metadata.WriteString("    <meta refines=\"#series\" property=\"collection-type\">series</meta>\n")
		if cfg.Book.SeriesIndex != "" {
			metadata.WriteString("    <meta refines=\"#series\" property=\"group-position\">" + html.EscapeString(cfg.Book.SeriesIndex) + "</meta>\n")
		}
	}

	var manifest, spine strings.Builder
	manifest.WriteString("    <item id=\"nav\" href=\"nav.xhtml\" media-type=\"application/xhtml+xml\" properties=\"nav\"/>\n")
	manifest.WriteString("    <item id=\"css\" href=\"css/novel.css\" media-type=\"text/css\"/>\n")
	spine.WriteString("    <itemref idref=\"nav\"/>\n")
	for i, sectionPath := range sections {
		id := fmt.Sprintf("section-%03d", i+1)
		href := strings.TrimPrefix(sectionPath, "EPUB/")
		manifest.WriteString(fmt.Sprintf("    <item id=\"%s\" href=\"%s\" media-type=\"application/xhtml+xml\"/>\n", id, href))
		spine.WriteString("    <itemref idref=\"" + id + "\"/>\n")
	}
	for _, item := range assets {
		properties := ""
		if item.ID == "cover-image" {
			properties = " properties=\"cover-image\""
		}
		manifest.WriteString(fmt.Sprintf("    <item id=\"%s\" href=\"%s\" media-type=\"%s\"%s/>\n", item.ID, strings.TrimPrefix(item.EPUBPath, "EPUB/"), item.MediaType, properties))
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id" xml:lang="` + html.EscapeString(cfg.Book.Language) + `" prefix="rendition: http://www.idpf.org/vocab/rendition/# marc: http://id.loc.gov/vocabulary/">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
` + metadata.String() + `  </metadata>
  <manifest>
` + manifest.String() + `  </manifest>
  <spine page-progression-direction="ltr">
` + spine.String() + `  </spine>
</package>
`
}

func writeEPUB(path string, entries []fileEntry) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	header := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	header.SetMode(0o644)
	w, err := zw.CreateHeader(header)
	if err == nil {
		_, err = io.WriteString(w, "application/epub+zip")
	}
	if err == nil {
		container := `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`
		w, err = zw.Create("META-INF/container.xml")
		if err == nil {
			_, err = io.WriteString(w, container)
		}
	}
	for _, entry := range entries {
		if err != nil {
			break
		}
		w, createErr := zw.Create(filepath.ToSlash(entry.Name))
		if createErr != nil {
			err = createErr
			break
		}
		_, err = w.Write(entry.Data)
	}
	closeZipErr := zw.Close()
	closeFileErr := f.Close()
	if err == nil {
		err = closeZipErr
	}
	if err == nil {
		err = closeFileErr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			_ = os.Remove(tmp)
			return err2
		}
	}
	return nil
}

func mediaTypeFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

func detectImageMediaType(data []byte, filename string) string {
	if mediaType, err := decodeRasterMediaType(data); err == nil {
		return mediaType
	}
	return mediaTypeFor(filename)
}

func decodeRasterMediaType(data []byte) (string, error) {
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	switch strings.ToLower(format) {
	case "jpeg":
		return "image/jpeg", nil
	case "png":
		return "image/png", nil
	case "gif":
		return "image/gif", nil
	default:
		return "", fmt.Errorf("未対応の画像内容です: %s", format)
	}
}

func canonicalImageExtension(path, mediaType string) string {
	switch mediaType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	default:
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			return ".bin"
		}
		return ext
	}
}

func mergeResult(target *model.Result, source model.Result) {
	target.Passed = append(target.Passed, source.Passed...)
	target.Fixed = append(target.Fixed, source.Fixed...)
	target.Warnings = append(target.Warnings, source.Warnings...)
	target.Confirmations = append(target.Confirmations, source.Confirmations...)
	target.Errors = append(target.Errors, source.Errors...)
}

func resolveEPUBReference(base, reference string) string {
	u, err := url.Parse(reference)
	if err != nil || u.IsAbs() || strings.HasPrefix(reference, "#") {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(base), filepath.FromSlash(u.Path))))
}
