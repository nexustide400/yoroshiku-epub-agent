package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yoroshiku-epub-agent/internal/discover"
	"yoroshiku-epub-agent/internal/epub"
	"yoroshiku-epub-agent/internal/model"
	"yoroshiku-epub-agent/internal/report"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var code int
	switch os.Args[1] {
	case "scan":
		code = runScan(os.Args[2:])
	case "build":
		code = runBuild(os.Args[2:])
	case "validate":
		code = runValidate(os.Args[2:])
	case "version", "--version", "-version":
		fmt.Println("book-builder " + version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "不明なコマンドです: %s\n\n", os.Args[1])
		usage()
		code = 2
	}
	os.Exit(code)
}

func runScan(args []string) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	input := fs.String("input", ".", "素材フォルダ")
	config := fs.String("config", filepath.Join(".kdp-work", "build-config.json"), "出力する設定JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := discover.Scan(*input)
	if err != nil {
		return fail(err)
	}
	configAbs, err := filepath.Abs(*config)
	if err != nil {
		return fail(err)
	}
	if err := model.SaveConfig(configAbs, cfg); err != nil {
		return fail(err)
	}
	fmt.Printf("スキャン完了: %s\n", configAbs)
	fmt.Printf("原稿: %d件 / 表紙: %s\n", len(cfg.Sections), cfg.Cover.Path)
	for _, decision := range cfg.Decisions {
		fmt.Printf("判断[%s]: %s\n", decision.Confidence, decision.Message)
	}
	for _, warning := range cfg.Warnings {
		fmt.Printf("警告: %s\n", warning)
	}
	return 0
}

func runBuild(args []string) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	config := fs.String("config", filepath.Join(".kdp-work", "build-config.json"), "Agentが確認した設定JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := model.LoadConfig(*config)
	if err != nil {
		return fail(err)
	}
	epubPath, result, buildErr := epub.Build(cfg)
	if buildErr != nil {
		result.Error("BUILD-FAILED", buildErr.Error())
	}
	if reportErr := report.WriteRelease(cfg, &result); reportErr != nil {
		return fail(reportErr)
	}
	if buildErr != nil {
		return fail(buildErr)
	}
	fmt.Printf("EPUB生成: %s\n", epubPath)
	fmt.Printf("検証: 成功%d / 自動修正%d / 警告%d / 要確認%d / エラー%d\n", len(result.Passed), len(result.Fixed), len(result.Warnings), len(result.Confirmations), len(result.Errors))
	fmt.Printf("成果物: %s\n", cfg.OutputDir)
	if !result.Valid() {
		return 1
	}
	return 0
}

func runValidate(args []string) int {
	if len(args) > 1 && !strings.HasPrefix(args[0], "-") {
		args = append(append([]string{}, args[1:]...), args[0])
	}
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	reportPath := fs.String("report", "", "Markdown検証レポートの出力先")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "validateにはEPUBファイルを1つ指定してください。")
		return 2
	}
	result, err := epub.Validate(fs.Arg(0))
	if err != nil {
		return fail(err)
	}
	if *reportPath != "" {
		if err := report.WriteValidation(*reportPath, result); err != nil {
			return fail(err)
		}
	}
	fmt.Printf("検証: 成功%d / 警告%d / 要確認%d / エラー%d\n", len(result.Passed), len(result.Warnings), len(result.Confirmations), len(result.Errors))
	for _, finding := range result.Errors {
		fmt.Printf("ERROR [%s] %s\n", finding.Code, finding.Message)
	}
	if !result.Valid() {
		return 1
	}
	return 0
}

func usage() {
	fmt.Print(`book-builder - Yoroshiku EPUB Agent Builder / Validator

使い方:
  book-builder scan --input . --config .kdp-work/build-config.json
  book-builder build --config .kdp-work/build-config.json
  book-builder validate release/book.epub --report release/validation-report.md
  book-builder version
`)
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "エラー:", err)
	return 1
}
