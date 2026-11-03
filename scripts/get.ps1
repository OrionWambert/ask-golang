# Downloads the latest ask release and installs it for the current user.
#   irm https://raw.githubusercontent.com/OrionWambert/ask-golang/main/scripts/get.ps1 | iex
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$installDir = Join-Path $env:LOCALAPPDATA 'Programs\ask'
$base = 'https://github.com/OrionWambert/ask-golang/releases/latest/download'
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$archive = "ask_windows_$arch.zip"

$tmp = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    $archivePath = Join-Path $tmp $archive
    $checksumsPath = Join-Path $tmp 'checksums.txt'
    Invoke-WebRequest -UseBasicParsing -Uri "$base/$archive" -OutFile $archivePath
    Invoke-WebRequest -UseBasicParsing -Uri "$base/checksums.txt" -OutFile $checksumsPath

    $expected = Get-Content $checksumsPath |
        ForEach-Object { $parts = $_ -split '\s+'; if ($parts[1] -eq $archive) { $parts[0] } }
    $actual = (Get-FileHash -Algorithm SHA256 $archivePath).Hash
    if (-not $expected -or $expected -ne $actual) {
        throw "Checksum mismatch for $archive"
    }

    Expand-Archive -Path $archivePath -DestinationPath $tmp
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    Copy-Item (Join-Path $tmp 'ask.exe') (Join-Path $installDir 'ask.exe') -Force
}
finally {
    Remove-Item -Recurse -Force $tmp
}
Write-Host "Installed $(& (Join-Path $installDir 'ask.exe') version) to $installDir"

# Read and write the raw value so entries such as %USERPROFILE% stay unexpanded.
$key = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey('Environment')
try {
    $raw = [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    $entries = @($raw -split ';' | Where-Object { $_ })
    if ($entries -notcontains $installDir) {
        $updated = (@($entries) + $installDir) -join ';'
        $key.SetValue('Path', $updated, [Microsoft.Win32.RegistryValueKind]::ExpandString)
        # Setting a variable through .NET tells running programs that the environment changed.
        [Environment]::SetEnvironmentVariable('ASK_PATH_REFRESH', $null, 'User')
        Write-Host "Added $installDir to your user PATH. Open a new terminal to use ask."
    }
}
finally {
    $key.Close()
}

if (-not (Get-Command claude -ErrorAction SilentlyContinue)) {
    Write-Warning 'The claude binary is not on your PATH: install Claude Code before using ask.'
}
