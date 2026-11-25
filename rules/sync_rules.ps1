$ErrorActionPreference = "Stop"
$OutDir = Join-Path $PSScriptRoot "build"
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

$RuleSets = @{
    "geosite-ir.srs" = "https://raw.githubusercontent.com/Chocolate4U/Iran-sing-box-rules/rule-set/geosite-ir.srs"
    "geoip-ir.srs"   = "https://raw.githubusercontent.com/Chocolate4U/Iran-sing-box-rules/rule-set/geoip-ir.srs"
}

foreach ($pair in $RuleSets.GetEnumerator()) {
    $dest = Join-Path $OutDir $pair.Key
    Write-Output "Fetching $($pair.Key)..."
    curl.exe -fsSL --retry 3 $pair.Value -o $dest
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to fetch $($pair.Key)"
    }
}
Write-Output "Iran .srs rule-sets ready in $OutDir"
