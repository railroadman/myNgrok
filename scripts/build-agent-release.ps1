[CmdletBinding()]
param(
    [ValidatePattern('^[0-9A-Za-z][0-9A-Za-z._+-]*$')]
    [string]$Version = "0.1.0",

    [ValidateSet("linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64")]
    [string[]]$Target = @("linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64")
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$agentDirectory = Join-Path $root "agent"
$outputDirectory = Join-Path $root "dist/agent"
$targets = @(
    @{ GOOS = "linux"; GOARCH = "amd64" },
    @{ GOOS = "linux"; GOARCH = "arm64" },
    @{ GOOS = "darwin"; GOARCH = "amd64" },
    @{ GOOS = "darwin"; GOARCH = "arm64" },
    @{ GOOS = "windows"; GOARCH = "amd64" }
)

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
$previousCGOEnabled = $env:CGO_ENABLED
$previousGOOS = $env:GOOS
$previousGOARCH = $env:GOARCH

try {
    $env:CGO_ENABLED = "0"
    Push-Location $agentDirectory
    foreach ($platform in $targets) {
        $targetName = "$($platform.GOOS)/$($platform.GOARCH)"
        if ($Target -notcontains $targetName) {
            continue
        }
        $env:GOOS = $platform.GOOS
        $env:GOARCH = $platform.GOARCH
        $extension = if ($platform.GOOS -eq "windows") { ".exe" } else { "" }
        $outputName = "tunnel-agent_{0}_{1}_{2}{3}" -f $Version, $platform.GOOS, $platform.GOARCH, $extension
        $outputPath = Join-Path $outputDirectory $outputName

        Write-Host "Building $outputName"
        go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $outputPath ./cmd/tunnel-agent
        if ($LASTEXITCODE -ne 0) {
            throw "Go build failed for $($platform.GOOS)/$($platform.GOARCH)."
        }
    }
}
finally {
    Pop-Location
    $env:CGO_ENABLED = $previousCGOEnabled
    $env:GOOS = $previousGOOS
    $env:GOARCH = $previousGOARCH
}
