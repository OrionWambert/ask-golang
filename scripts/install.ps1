# Builds ask from source and installs it for the current user.
#   .\scripts\install.ps1
#   .\scripts\install.ps1 -InstallDir C:\tools\ask -NoPath
[CmdletBinding()]
param(
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Programs\ask'),
    [switch]$NoPath,
    [string]$RegistryKey = 'Environment'
)

$root = Split-Path -Parent $PSScriptRoot

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error 'Go is required to build ask: https://go.dev/dl/'
    exit 1
}

$version = 'dev'
if (Get-Command git -ErrorAction SilentlyContinue) {
    $described = & cmd /c "git -C `"$root`" describe --tags --always --dirty 2>nul"
    if ($LASTEXITCODE -eq 0 -and $described) { $version = $described }
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$target = Join-Path $InstallDir 'ask.exe'

Push-Location $root
try {
    & go build -ldflags "-X oriowambert.com/ask-golang/cmd.version=$version" -o $target .
    if ($LASTEXITCODE -ne 0) {
        Write-Error 'go build failed'
        exit 1
    }
}
finally {
    Pop-Location
}
Write-Host "Installed ask $version to $target"

if (-not $NoPath) {
    # Read and write the raw value so entries such as %USERPROFILE% stay unexpanded.
    $key = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey($RegistryKey)
    try {
        $raw = [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
        $entries = @($raw -split ';' | Where-Object { $_ })
        if ($entries -notcontains $InstallDir) {
            $updated = (@($entries) + $InstallDir) -join ';'
            $key.SetValue('Path', $updated, [Microsoft.Win32.RegistryValueKind]::ExpandString)
            # Setting a variable through .NET tells running programs that the environment changed.
            [Environment]::SetEnvironmentVariable('ASK_PATH_REFRESH', $null, 'User')
            Write-Host "Added $InstallDir to your user PATH. Open a new terminal to use ask."
        }
    }
    finally {
        $key.Close()
    }
}

if (-not (Get-Command claude -ErrorAction SilentlyContinue)) {
    Write-Warning 'The claude binary is not on your PATH: install Claude Code before using ask.'
}
