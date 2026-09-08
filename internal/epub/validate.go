package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"yoroshiku-epub-agent/internal/model"
)

type manifestItem struct {
	ID         string
	Href       string
	MediaType  string
	Properties string
}

type packageInfo struct {
	Version       string
	UniqueID      string
	Metadata      map[string][]string
	IdentifierIDs map[string]bool
	Titles        []opfTitle
	Metas         []opfMeta
	Manifest      []manifestItem
	Spine         []string
	PackagePath   string
	PackageFolder string
}

type opfTitle struct {
	ID    string
	Value string
}

type opfMeta struct {
	Refines  string
	Property string
	Value    string
}

type linkRef struct {
	From   string
	Target string
	Attr   string
}

var cssURLPattern = regexp.MustCompile(`(?i)url\(\s*["']?([^)'"\s]+)["']?\s*\)`)

func Validate(epubPath string) (model.Result, error) {
	result := model.Result{}
	zr, err := zip.OpenReader(epubPath)
	if err != nil {
		return result, fmt.Errorf("EPUBを開けません: %w", err)
	}
	defer zr.Close()
	if len(zr.File) == 0 {
		result.Error("OCF-EMPTY", "EPUB ZIPが空です。")
		return result, nil
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		name := filepath.ToSlash(f.Name)
		if _, exists := files[name]; exists {
			result.Error("OCF-DUPLICATE", "ZIP内でパスが重複しています: "+name)
		}
		files[name] = f
		if name != path.Clean(name) || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || name == ".." || strings.HasPrefix(name, "../") {
			result.Error("OCF-PATH", "不正または非正規のEPUBパスです: "+name)
		}
	}
	if zr.File[0].Name != "mimetype" || zr.File[0].Method != zip.Store {
		result.Error("OCF-MIMETYPE-ORDER", "mimetypeはZIPの先頭に無圧縮で格納する必要があります。")
	} else if data, readErr := readZip(zr.File[0]); readErr != nil || string(data) != "application/epub+zip" {
		result.Error("OCF-MIMETYPE-VALUE", "mimetypeの内容がapplication/epub+zipではありません。")
	} else {
		result.Pass("OCF-MIMETYPE", "mimetypeは先頭・無圧縮・正しい値です。")
	}
	containerFile := files["META-INF/container.xml"]
	if containerFile == nil {
		result.Error("OCF-CONTAINER", "META-INF/container.xmlがありません。")
		return result, nil
	}
	containerData, err := readZip(containerFile)
	if err != nil {
		return result, err
	}
	packagePath, err := parseContainer(containerData)
	if err != nil {
		result.Error("OCF-CONTAINER-XML", err.Error())
		return result, nil
	}
	result.Pass("OCF-CONTAINER", "container.xmlからパッケージ文書を解決できました。")
	opfFile := files[packagePath]
	if opfFile == nil {
		result.Error("OPF-MISSING", "container.xmlが示すパッケージ文書がありません: "+packagePath)
		return result, nil
	}
	opfData, err := readZip(opfFile)
	if err != nil {
		return result, err
	}
	info, parseErr := parsePackage(opfData, packagePath)
	if parseErr != nil {
		result.Error("OPF-XML", parseErr.Error())
		return result, nil
	}
	validatePackage(info, files, &result)

	idsByFile := map[string]map[string]bool{}
	var refs []linkRef
	for name, file := range files {
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".xhtml") || strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm") || strings.HasSuffix(lower, ".svg") {
			data, readErr := readZip(file)
			if readErr != nil {
				return result, readErr
			}
			ids, links, tocCount, xmlErr := inspectXML(name, data)
			if xmlErr != nil {
				result.Error("CONTENT-XML", fmt.Sprintf("%s: XMLとして不正です: %v", name, xmlErr))
				continue
			}
			idsByFile[name] = ids
			refs = append(refs, links...)
			if isNavItem(name, info) {
				if tocCount != 1 {
					result.Error("NAV-TOC", fmt.Sprintf("%s: epub:type=\"toc\"のnav要素は1個必要です（検出%d個）。", name, tocCount))
				} else {
					result.Pass("NAV-TOC", "機械可読かつ表示可能なEPUB目次があります。")
				}
			}
		}
		if strings.HasSuffix(lower, ".css") {
			data, readErr := readZip(file)
			if readErr != nil {
				return result, readErr
			}
			if len(bytes.TrimSpace(data)) == 0 {
				result.Error("CSS-EMPTY", name+": CSSが空です。")
			}
			for _, m := range cssURLPattern.FindAllStringSubmatch(string(data), -1) {
				if len(m) > 1 {
					refs = append(refs, linkRef{From: name, Target: m[1], Attr: "url"})
				}
			}
		}
	}
	validateLinks(refs, files, idsByFile, &result)
	if len(result.Errors) == 0 {
		result.Pass("EPUB-INTERNAL", "内蔵Validatorの必須構造・参照検査をすべて通過しました。")
	}
	result.Confirm("KINDLE-PREVIEWER", "Amazon Kindle Previewerで端末別・文字サイズ別・背景色別の最終表示を確認してください。")
	return result, nil
}

func parseContainer(data []byte) (string, error) {
	var container struct {
		Rootfiles []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := xml.Unmarshal(data, &container); err != nil {
		return "", fmt.Errorf("container.xmlが不正です: %w", err)
	}
	if len(container.Rootfiles) == 0 || container.Rootfiles[0].FullPath == "" {
		return "", fmt.Errorf("container.xmlにrootfileがありません")
	}
	return path.Clean(container.Rootfiles[0].FullPath), nil
}

func parsePackage(data []byte, packagePath string) (packageInfo, error) {
	var raw struct {
		Version  string `xml:"version,attr"`
		UniqueID string `xml:"unique-identifier,attr"`
		Metadata struct {
			Identifiers []struct {
				ID    string `xml:"id,attr"`
				Value string `xml:",chardata"`
			} `xml:"identifier"`
			Titles []struct {
				ID    string `xml:"id,attr"`
				Value string `xml:",chardata"`
			} `xml:"title"`
			Languages []string `xml:"language"`
			Creators  []string `xml:"creator"`
			Metas     []struct {
				Property string `xml:"property,attr"`
				Refines  string `xml:"refines,attr"`
				Value    string `xml:",chardata"`
			} `xml:"meta"`
		} `xml:"metadata"`
		Manifest struct {
			Items []struct {
				ID         string `xml:"id,attr"`
				Href       string `xml:"href,attr"`
				MediaType  string `xml:"media-type,attr"`
				Properties string `xml:"properties,attr"`
			} `xml:"item"`
		} `xml:"manifest"`
		Spine struct {
			Items []struct {
				IDRef string `xml:"idref,attr"`
			} `xml:"itemref"`
		} `xml:"spine"`
	}
	if err := xml.Unmarshal(data, &raw); err != nil {
		return packageInfo{}, err
	}
	info := packageInfo{Version: raw.Version, UniqueID: raw.UniqueID, Metadata: map[string][]string{}, IdentifierIDs: map[string]bool{}, PackagePath: packagePath, PackageFolder: path.Dir(packagePath)}
	for _, identifier := range raw.Metadata.Identifiers {
		info.Metadata["identifier"] = append(info.Metadata["identifier"], strings.TrimSpace(identifier.Value))
		if identifier.ID != "" {
			info.IdentifierIDs[identifier.ID] = true
		}
	}
	for _, title := range raw.Metadata.Titles {
		value := strings.TrimSpace(title.Value)
		info.Metadata["title"] = append(info.Metadata["title"], value)
		info.Titles = append(info.Titles, opfTitle{ID: title.ID, Value: value})
	}
	info.Metadata["language"] = raw.Metadata.Languages
	info.Metadata["creator"] = raw.Metadata.Creators
	for _, meta := range raw.Metadata.Metas {
		value := strings.TrimSpace(meta.Value)
		info.Metas = append(info.Metas, opfMeta{Refines: meta.Refines, Property: meta.Property, Value: value})
		if meta.Property != "" {
			info.Metadata["meta:"+meta.Property] = append(info.Metadata["meta:"+meta.Property], value)
		}
	}
	for _, item := range raw.Manifest.Items {
		info.Manifest = append(info.Manifest, manifestItem{ID: item.ID, Href: item.Href, MediaType: item.MediaType, Properties: item.Properties})
	}
	for _, item := range raw.Spine.Items {
		info.Spine = append(info.Spine, item.IDRef)
	}
	return info, nil
}

func validatePackage(info packageInfo, files map[string]*zip.File, result *model.Result) {
	metadataErrorsBefore := len(result.Errors)
	if info.Version != "3.0" {
		result.Error("OPF-VERSION", "package versionはEPUB 3用の3.0である必要があります。")
	} else {
		result.Pass("OPF-VERSION", "EPUB package version 3.0です。")
	}
	for _, name := range []string{"identifier", "title", "language"} {
		if !hasNonEmpty(info.Metadata[name]) {
			result.Error("OPF-METADATA", "必須メタデータがありません: dc:"+name)
		}
	}
	if !hasNonEmpty(info.Metadata["meta:dcterms:modified"]) {
		result.Error("OPF-MODIFIED", "dcterms:modifiedがありません。")
	} else if _, err := time.Parse("2006-01-02T15:04:05Z", info.Metadata["meta:dcterms:modified"][0]); err != nil {
		result.Error("OPF-MODIFIED-FORMAT", "dcterms:modifiedはUTCのYYYY-MM-DDThh:mm:ssZ形式である必要があります。")
	}
	if info.UniqueID == "" || !info.IdentifierIDs[info.UniqueID] {
		result.Error("OPF-UNIQUE-ID", "packageのunique-identifierがdc:identifierのidを参照していません。")
	}
	validateMainTitle(info, result)
	if len(result.Errors) == metadataErrorsBefore {
		result.Pass("OPF-METADATA", "identifier/title/language/modifiedを確認しました。")
	}
	ids := map[string]bool{}
	itemsByID := map[string]manifestItem{}
	manifestPaths := map[string]bool{}
	navCount, coverCount := 0, 0
	for _, item := range info.Manifest {
		if item.ID == "" || ids[item.ID] {
			result.Error("OPF-MANIFEST-ID", "manifest idが空または重複しています: "+item.ID)
		}
		ids[item.ID] = true
		itemsByID[item.ID] = item
		resolved := path.Clean(path.Join(info.PackageFolder, item.Href))
		if item.Href == "" || strings.HasPrefix(resolved, "../") || files[resolved] == nil {
			result.Error("OPF-MANIFEST-HREF", "manifest参照先がありません: "+item.Href)
		} else {
			if manifestPaths[resolved] {
				result.Error("OPF-MANIFEST-DUPLICATE-HREF", "manifestのhrefが重複しています: "+item.Href)
			}
			manifestPaths[resolved] = true
			expected := mediaTypeForEPUB(resolved)
			if expected != "" && expected != item.MediaType {
				result.Error("OPF-MIME", fmt.Sprintf("%s: MIME typeは%sですが%sが指定されています。", resolved, expected, item.MediaType))
			}
			if strings.HasPrefix(item.MediaType, "image/") {
				if data, err := readZip(files[resolved]); err == nil {
					if item.MediaType == "image/svg+xml" {
						if _, _, _, xmlErr := inspectXML(resolved, data); xmlErr != nil {
							result.Error("OPF-IMAGE-DECODE", fmt.Sprintf("%s: SVGを解析できません: %v", resolved, xmlErr))
						}
					} else if actual, decodeErr := decodeRasterMediaType(data); decodeErr != nil {
						result.Error("OPF-IMAGE-DECODE", fmt.Sprintf("%s: 画像を解析できません: %v", resolved, decodeErr))
					} else if actual != item.MediaType {
						result.Error("OPF-MIME-CONTENT", fmt.Sprintf("%s: 画像内容は%sですがmanifestは%sです。", resolved, actual, item.MediaType))
					}
				}
			}
		}
		if tokenContains(item.Properties, "nav") {
			navCount++
		}
		if tokenContains(item.Properties, "cover-image") {
			coverCount++
		}
	}
	if navCount != 1 {
		result.Error("OPF-NAV", fmt.Sprintf("properties=navのmanifest itemは1個必要です（検出%d個）。", navCount))
	} else {
		result.Pass("OPF-NAV", "manifestにEPUB Navigation Documentが1件あります。")
	}
	if coverCount != 1 {
		result.Error("OPF-COVER", fmt.Sprintf("properties=cover-imageのmanifest itemは1個必要です（検出%d個）。", coverCount))
	} else {
		result.Pass("OPF-COVER", "内部表紙画像がcover-imageとして指定されています。")
	}
	if len(info.Spine) == 0 {
		result.Error("OPF-SPINE", "spineが空です。")
	}
	for _, idref := range info.Spine {
		if !ids[idref] {
			result.Error("OPF-SPINE-IDREF", "spineのidrefがmanifestにありません: "+idref)
		} else if itemsByID[idref].MediaType != "application/xhtml+xml" && itemsByID[idref].MediaType != "image/svg+xml" {
			result.Error("OPF-SPINE-MEDIA", "spine itemはXHTMLまたはSVGである必要があります: "+idref)
		}
	}
	if len(info.Spine) > 0 {
		result.Pass("OPF-SPINE", fmt.Sprintf("spineの本文順序 %d件を確認しました。", len(info.Spine)))
	}
	for name := range files {
		if name == "mimetype" || name == "META-INF/container.xml" || name == info.PackagePath || strings.HasSuffix(name, "/") {
			continue
		}
		if !manifestPaths[name] {
			result.Error("OPF-UNMANIFESTED", "manifest未登録のEPUBリソースです: "+name)
		}
	}
}

func validateMainTitle(info packageInfo, result *model.Result) {
	titlesByID := map[string][]opfTitle{}
	for _, title := range info.Titles {
		if title.ID != "" {
			titlesByID[title.ID] = append(titlesByID[title.ID], title)
		}
	}
	var mainMetas []opfMeta
	for _, meta := range info.Metas {
		if meta.Property == "title-type" && meta.Value == "main" {
			mainMetas = append(mainMetas, meta)
		}
	}
	if len(mainMetas) != 1 {
		result.Error("OPF-TITLE-MAIN", fmt.Sprintf("主タイトルを一意に示す<meta property=\"title-type\">main</meta>は1件必要です（検出%d件、Kindle E20006/E21011対策）。", len(mainMetas)))
		return
	}
	refines := mainMetas[0].Refines
	if !strings.HasPrefix(refines, "#") || len(refines) == 1 {
		result.Error("OPF-TITLE-MAIN-REFINES", "title-type=mainのrefinesは主dc:titleのIDを#付きで参照する必要があります。")
		return
	}
	targetID := strings.TrimPrefix(refines, "#")
	targets := titlesByID[targetID]
	if len(targets) != 1 || strings.TrimSpace(targets[0].Value) == "" {
		result.Error("OPF-TITLE-MAIN-REFINES", fmt.Sprintf("title-type=mainの参照先%sに、一意で空でないdc:titleがありません。", refines))
		return
	}
	result.Pass("OPF-TITLE-MAIN", fmt.Sprintf("主タイトル「%s」はdc:title#%sとして一意に特定され、title-type=mainから正しく参照されています。", targets[0].Value, targetID))
}

func inspectXML(name string, data []byte) (map[string]bool, []linkRef, int, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	ids := map[string]bool{}
	var refs []linkRef
	tocCount := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ids, refs, tocCount, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		id := attr(start.Attr, "id")
		if id != "" {
			ids[id] = true
		}
		if start.Name.Local == "nav" && tokenContains(attr(start.Attr, "type"), "toc") {
			tocCount++
		}
		for _, key := range []string{"href", "src"} {
			value := attr(start.Attr, key)
			if value != "" {
				refs = append(refs, linkRef{From: name, Target: value, Attr: key})
			}
		}
		if start.Name.Local == "img" && attrPresent(start.Attr, "alt") == false {
			refs = append(refs, linkRef{From: name, Target: "", Attr: "missing-alt"})
		}
	}
	return ids, refs, tocCount, nil
}

func validateLinks(refs []linkRef, files map[string]*zip.File, ids map[string]map[string]bool, result *model.Result) {
	checked := 0
	external := 0
	for _, ref := range refs {
		if ref.Attr == "missing-alt" {
			result.Error("CONTENT-IMG-ALT", ref.From+": img要素にalt属性がありません。")
			continue
		}
		u, err := url.Parse(ref.Target)
		if err != nil {
			result.Error("CONTENT-LINK", ref.From+": 不正な参照です: "+ref.Target)
			continue
		}
		if u.IsAbs() || strings.HasPrefix(ref.Target, "data:") || strings.HasPrefix(ref.Target, "mailto:") {
			external++
			continue
		}
		targetFile := ref.From
		if u.Path != "" {
			decoded, decodeErr := url.PathUnescape(u.Path)
			if decodeErr != nil {
				result.Error("CONTENT-LINK", ref.From+": URLデコードできません: "+ref.Target)
				continue
			}
			targetFile = path.Clean(path.Join(path.Dir(ref.From), decoded))
		}
		if targetFile == ".." || strings.HasPrefix(targetFile, "../") || files[targetFile] == nil {
			result.Error("CONTENT-LINK-MISSING", fmt.Sprintf("%s の%s参照先がありません: %s", ref.From, ref.Attr, ref.Target))
			continue
		}
		if u.Fragment != "" && ids[targetFile] != nil && !ids[targetFile][u.Fragment] {
			result.Error("CONTENT-FRAGMENT-MISSING", fmt.Sprintf("%s のリンク先IDがありません: %s", ref.From, ref.Target))
			continue
		}
		checked++
	}
	result.Pass("CONTENT-LINKS", fmt.Sprintf("内部リンク・画像・CSS参照 %d件を確認しました。", checked))
	if external > 0 {
		result.Warn("CONTENT-EXTERNAL-LINKS", fmt.Sprintf("外部リンク%d件は到達確認の対象外です。KDPポリシーとリンク先を手動確認してください。", external))
	}
}

func isNavItem(name string, info packageInfo) bool {
	for _, item := range info.Manifest {
		if tokenContains(item.Properties, "nav") && path.Clean(path.Join(info.PackageFolder, item.Href)) == name {
			return true
		}
	}
	return false
}

func readZip(file *zip.File) ([]byte, error) {
	r, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func attr(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func attrPresent(attrs []xml.Attr, local string) bool {
	for _, a := range attrs {
		if a.Name.Local == local {
			return true
		}
	}
	return false
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hasNonEmpty(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func tokenContains(value, target string) bool {
	for _, token := range strings.Fields(value) {
		if token == target {
			return true
		}
	}
	return false
}

func mediaTypeForEPUB(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".xhtml", ".html", ".htm":
		return "application/xhtml+xml"
	case ".css":
		return "text/css"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	default:
		return ""
	}
}
