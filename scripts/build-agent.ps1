param(
    [string]$Version = "0.1.0",
    [string[]]$Archs = @("amd64"),
    [string]$Out = ""
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Go = Join-Path $env:LOCALAPPDATA "Programs\go\bin"
if (Test-Path $Go) { $env:Path = "$env:Path;$Go" }

if (-not $Out) {
    $Out = Join-Path $Root "plugin\src\sbin\pangolin-agent"
}
$Commit = "unknown"
try { $Commit = (git -C $Root rev-parse --short HEAD) } catch {}
$Date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Out) | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $Root "dist") | Out-Null

Push-Location (Join-Path $Root "agent")
try {
    foreach ($Arch in $Archs) {
        $Target = $Out
        if ($Arch -ne "amd64") {
            $Target = Join-Path $Root "dist\pangolin-agent-freebsd-$Arch"
        }
        Write-Host "building $Target (freebsd/$Arch)"
        $env:GOOS = "freebsd"
        $env:GOARCH = $Arch
        $env:CGO_ENABLED = "0"
        go build -trimpath `
            -ldflags "-s -w -X github.com/os-pangolin/os-pangolin/agent/internal/version.Version=$Version -X github.com/os-pangolin/os-pangolin/agent/internal/version.Commit=$Commit -X github.com/os-pangolin/os-pangolin/agent/internal/version.Date=$Date" `
            -o $Target ./cmd/pangolin-agent
        if ($LASTEXITCODE -ne 0) { throw "go build failed" }
    }
} finally {
    Pop-Location
}
Write-Host "agent build complete"
