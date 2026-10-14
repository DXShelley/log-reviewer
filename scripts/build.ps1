param([string]$Go = 'go', [switch]$AllPlatforms)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
Push-Location $root
$oldOS = $env:GOOS
$oldArch = $env:GOARCH
$oldCGO = $env:CGO_ENABLED
try {
    New-Item -ItemType Directory -Force -Path dist | Out-Null
    $env:CGO_ENABLED = '0'
    $targets = @('windows/amd64')
    if ($AllPlatforms) { $targets = @('windows/amd64', 'windows/arm64', 'linux/amd64', 'linux/arm64', 'darwin/amd64', 'darwin/arm64') }
    foreach ($target in $targets) {
        $parts = $target.Split('/')
        $env:GOOS = $parts[0]
        $env:GOARCH = $parts[1]
        $suffix = ''
        if ($env:GOOS -eq 'windows') { $suffix = '.exe' }
        $output = "dist/log-reviewer-$($env:GOOS)-$($env:GOARCH)$suffix"
        & $Go build -trimpath -ldflags='-s -w -buildid=' -o $output ./cmd/log-reviewer
        if ($LASTEXITCODE -ne 0) { throw "Build failed: $target" }
        $item = Get-Item -LiteralPath $output
        if ($item.Length -gt 10MB) { throw "Binary exceeds 10 MiB: $output" }
        Write-Host "$output : $([math]::Round($item.Length / 1MB, 2)) MiB"
    }
    Get-ChildItem dist -File | ForEach-Object { Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256 } | Format-Table -AutoSize
} finally {
    $env:GOOS = $oldOS
    $env:GOARCH = $oldArch
    $env:CGO_ENABLED = $oldCGO
    Pop-Location
}
