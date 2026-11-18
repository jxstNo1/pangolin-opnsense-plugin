param(
    [string]$Version = "0.1.0",
    [string]$Arch = "amd64"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Name = "os-pangolin-$Version-$Arch"
$Dist = Join-Path $Root "dist"

& (Join-Path $PSScriptRoot "build-agent.ps1") -Version $Version -Archs @($Arch)
if ($LASTEXITCODE -ne 0) { throw "agent build failed" }

$Stage = Join-Path $Dist $Name
if (Test-Path $Stage) { Remove-Item -Recurse -Force $Stage }
New-Item -ItemType Directory -Force -Path $Stage | Out-Null
Copy-Item -Recurse (Join-Path $Root "plugin\src") (Join-Path $Stage "src")
Copy-Item (Join-Path $Root "plugin\Makefile") $Stage
Copy-Item (Join-Path $Root "plugin\pkg-descr") $Stage
Copy-Item (Join-Path $Root "plugin\+PRE_DEINSTALL") $Stage
Copy-Item (Join-Path $Root "plugin\+POST_INSTALL") $Stage
Copy-Item (Join-Path $Root "plugin\+POST_DEINSTALL") $Stage
Copy-Item (Join-Path $Root "LICENSE") $Stage
Copy-Item (Join-Path $Root "README.md") $Stage
Copy-Item (Join-Path $Root "CHANGELOG.md") $Stage

$Tar = Join-Path $Dist "$Name.tgz"
tar -czf $Tar -C $Dist $Name
$Hash = (Get-FileHash -Algorithm SHA256 $Tar).Hash.ToLower()
"$Hash  $Name.tgz" | Set-Content "$Tar.sha256"
Write-Host "created $Tar"
