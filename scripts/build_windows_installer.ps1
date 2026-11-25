[CmdletBinding()]
param(
    [string]$ReleaseDir = "",
    [string]$OutputPath = "",
    [string]$NsisPath = ""
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($ReleaseDir)) {
    $ReleaseDir = Join-Path $root "app\build\windows\x64\runner\Release"
}
if (-not (Test-Path -LiteralPath $ReleaseDir -PathType Container)) {
    throw "Windows Release directory not found: $ReleaseDir"
}
$ReleaseDir = (Resolve-Path -LiteralPath $ReleaseDir).Path

$pubspec = Join-Path $root "app\pubspec.yaml"
$line = (Select-String -Path $pubspec -Pattern '^version:\s*(.+)$').Matches[0].Groups[1].Value.Trim()
$parts = $line -split '\+'
$version = $parts[0]
$build = if ($parts.Length -gt 1) { $parts[1] } else { "1" }
if ($version -notmatch '^[0-9]+\.[0-9]+\.[0-9]+$') { throw "Invalid pubspec version: $line" }
if ($build -notmatch '^[0-9]+$') { throw "Invalid pubspec build number: $line" }

$required = @(
    (Join-Path $ReleaseDir "mlz_vpn.exe"),
    (Join-Path $ReleaseDir "mlz-engine.exe"),
    (Join-Path $ReleaseDir "cores\sing-box.exe"),
    (Join-Path $ReleaseDir "cores\xray.exe"),
    (Join-Path $ReleaseDir "rules\geosite-ir.srs"),
    (Join-Path $ReleaseDir "rules\geoip-ir.srs")
)
foreach ($file in $required) {
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing release file: $file" }
    if ((Get-Item -LiteralPath $file).Length -le 0) { throw "Empty release file: $file" }
}

if ([string]::IsNullOrWhiteSpace($OutputPath)) {
    $OutputPath = Join-Path $root "dist\windows-x64\mlz-vpn-windows-x64-setup.exe"
}
$outputDir = Split-Path -Parent $OutputPath
New-Item -ItemType Directory -Force -Path $outputDir | Out-Null
$OutputPath = [IO.Path]::GetFullPath($OutputPath)
if (Test-Path -LiteralPath $OutputPath) { Remove-Item -LiteralPath $OutputPath -Force }

if ([string]::IsNullOrWhiteSpace($NsisPath)) {
    $candidates = @(
        (Join-Path ${env:ProgramFiles(x86)} 'NSIS\makensis.exe'),
        (Join-Path $env:ProgramFiles 'NSIS\makensis.exe')
    )
    $cmd = Get-Command makensis.exe -ErrorAction SilentlyContinue
    if ($cmd) { $candidates = @($cmd.Source) + $candidates }
    $NsisPath = $candidates | Where-Object { $_ -and (Test-Path -LiteralPath $_ -PathType Leaf) } | Select-Object -First 1
}
if (-not $NsisPath -or -not (Test-Path -LiteralPath $NsisPath -PathType Leaf)) {
    throw "NSIS makensis.exe not found. Install NSIS 3.x or pass -NsisPath."
}

$script = Join-Path $root "app\windows\installer\mlz-vpn.nsi"
$icon = Join-Path $root "app\windows\runner\resources\app_icon.ico"
$license = Join-Path $root "LICENSE"
$nsisArgs = @(
    "/V2",
    "/DAPP_VERSION=$version",
    "/DAPP_BUILD=$build",
    "/DSOURCE_DIR=$ReleaseDir",
    "/DOUTPUT_FILE=$OutputPath",
    "/DAPP_ICON=$icon",
    "/DLICENSE_FILE=$license",
    $script
)
Write-Host "Building MLZ VPN $version ($build) setup..."
Write-Host "  Source: $ReleaseDir"
Write-Host "  Output: $OutputPath"
& $NsisPath @nsisArgs
if ($LASTEXITCODE -ne 0) { throw "makensis failed with exit code $LASTEXITCODE" }
if (-not (Test-Path -LiteralPath $OutputPath -PathType Leaf)) { throw "Installer was not created: $OutputPath" }
$info = Get-Item -LiteralPath $OutputPath
$hash = (Get-FileHash -LiteralPath $OutputPath -Algorithm SHA256).Hash.ToLower()
Write-Host ("Setup ready: {0} ({1:N0} bytes)" -f $info.FullName, $info.Length)
Write-Host "SHA256: $hash"
$info | Select-Object FullName, Length, LastWriteTime
