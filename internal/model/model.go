package model

// Config is the deterministic hand-off from the publishing agent to the builder.
// Paths are relative to InputDir unless documented otherwise.
type Config struct {
	Version    int        `json:"version"`
	InputDir   string     `json:"inputDir,omitempty"`
	OutputDir  string     `json:"outputDir,omitempty"`
	Book       Book       `json:"book"`
	Cover      Cover      `json:"cover"`
	Sections   []Section  `json:"sections"`
	KDP        KDP        `json:"kdp,omitempty"`
	Validation Validation `json:"validation,omitempty"`
	Decisions  []Decision `json:"decisions,omitempty"`
	Warnings   []string   `json:"warnings,omitempty"`
}

type Book struct {
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle,omitempty"`
	Series      string `json:"series,omitempty"`
	SeriesIndex string `json:"seriesIndex,omitempty"`
	Creator     string `json:"creator"`
	Language    string `json:"language,omitempty"`
	Identifier  string `json:"identifier,omitempty"`
	Publisher   string `json:"publisher,omitempty"`
	Description string `json:"description,omitempty"`
	Rights      string `json:"rights,omitempty"`
	Modified    string `json:"modified,omitempty"`
	WritingMode string `json:"writingMode,omitempty"`
}

type Cover struct {
	Path string `json:"path"`
	Alt  string `json:"alt,omitempty"`
}

type Section struct {
	Path   string         `json:"path"`
	Title  string         `json:"title,omitempty"`
	Level  int            `json:"level,omitempty"`
	Images []Illustration `json:"images,omitempty"`
}

type Illustration struct {
	Path           string `json:"path"`
	Alt            string `json:"alt,omitempty"`
	Caption        string `json:"caption,omitempty"`
	AfterParagraph int    `json:"afterParagraph,omitempty"`
}

type KDP struct {
	Description      string   `json:"description,omitempty"`
	Keywords         []string `json:"keywords,omitempty"`
	Categories       []string `json:"categories,omitempty"`
	PrimaryMarket    string   `json:"primaryMarket,omitempty"`
	RightsConfirmed  *bool    `json:"rightsConfirmed,omitempty"`
	PublicDomain     *bool    `json:"publicDomain,omitempty"`
	AIGeneratedText  string   `json:"aiGeneratedText,omitempty"`
	AIGeneratedCover string   `json:"aiGeneratedCover,omitempty"`
	AIGeneratedArt   string   `json:"aiGeneratedArt,omitempty"`
	AITranslation    string   `json:"aiTranslation,omitempty"`
	SelectIntent     string   `json:"selectIntent,omitempty"`
	PriceMemo        string   `json:"priceMemo,omitempty"`
	AudienceMemo     string   `json:"audienceMemo,omitempty"`
}

type Validation struct {
	RunEPUBCheck bool   `json:"runEpubCheck,omitempty"`
	EPUBCheck    string `json:"epubCheckPath,omitempty"`
}

type Decision struct {
	Kind       string `json:"kind"`
	Message    string `json:"message"`
	Confidence string `json:"confidence,omitempty"`
}

type Finding struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Result struct {
	Passed        []Finding `json:"passed"`
	Fixed         []Finding `json:"fixed"`
	Warnings      []Finding `json:"warnings"`
	Confirmations []Finding `json:"confirmations"`
	Errors        []Finding `json:"errors"`
}

func (r *Result) Pass(code, message string) {
	r.Passed = append(r.Passed, Finding{Code: code, Message: message})
}

func (r *Result) Fix(code, message string) {
	r.Fixed = append(r.Fixed, Finding{Code: code, Message: message})
}

func (r *Result) Warn(code, message string) {
	r.Warnings = append(r.Warnings, Finding{Code: code, Message: message})
}

func (r *Result) Confirm(code, message string) {
	r.Confirmations = append(r.Confirmations, Finding{Code: code, Message: message})
}

func (r *Result) Error(code, message string) {
	r.Errors = append(r.Errors, Finding{Code: code, Message: message})
}

func (r Result) Valid() bool { return len(r.Errors) == 0 }
