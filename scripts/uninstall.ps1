# Removes ask and its PATH entry. Saved config and history are kept.
[CmdletBinding()]
param(
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Programs\ask'),
    [string]$RegistryKey = 'Environment'
)

$target = Join-Path $InstallDir 'ask.exe'
if (Test-Path $target) {
    Remove-Item $target -Force
    Write-Host "Removed $target"
}
if ((Test-Path $InstallDir) -and -not (Get-ChildItem -Force $InstallDir)) {
    Remove-Item $InstallDir -Force
}

$key = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey($RegistryKey)
try {
    $raw = [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    $entries = @($raw -split ';' | Where-Object { $_ })
    if ($entries -contains $InstallDir) {
        $updated = @($entries | Where-Object { $_ -ne $InstallDir }) -join ';'
        $key.SetValue('Path', $updated, [Microsoft.Win32.RegistryValueKind]::ExpandString)
        [Environment]::SetEnvironmentVariable('ASK_PATH_REFRESH', $null, 'User')
        Write-Host "Removed $InstallDir from your user PATH."
    }
}
finally {
    $key.Close()
}

$configDir = Join-Path $env:APPDATA 'ask'
if (Test-Path $configDir) {
    Write-Host "Your config and history are still in $configDir"
}
