$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot
$outputDir = Join-Path $projectRoot "bin"
New-Item -ItemType Directory -Force -Path $outputDir | Out-Null

$targets = @(
    @{ OS = "windows"; Arch = "amd64"; Name = "book-builder-windows-amd64.exe" },
    @{ OS = "windows"; Arch = "arm64"; Name = "book-builder-windows-arm64.exe" },
    @{ OS = "darwin"; Arch = "amd64"; Name = "book-builder-macos-amd64" },
    @{ OS = "darwin"; Arch = "arm64"; Name = "book-builder-macos-arm64" },
    @{ OS = "linux"; Arch = "amd64"; Name = "book-builder-linux-amd64" },
    @{ OS = "linux"; Arch = "arm64"; Name = "book-builder-linux-arm64" }
)

Push-Location $projectRoot
try {
    foreach ($target in $targets) {
        $env:GOOS = $target.OS
        $env:GOARCH = $target.Arch
        $env:CGO_ENABLED = "0"
        go build -trimpath -ldflags "-s -w" -o (Join-Path $outputDir $target.Name) ./cmd/book-builder
    }
}
finally {
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}
