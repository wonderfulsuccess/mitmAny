param([Parameter(Mandatory=$true)][string]$Executable, [Parameter(Mandatory=$true)][string]$StatusPath)
$ErrorActionPreference = 'Stop'
if (-not $env:WINDOWS_CERTIFICATE_PFX -and -not $env:WINDOWS_CERTIFICATE_PASSWORD) {
  'Windows: unsigned (no Authenticode signature).' | Set-Content $StatusPath
  exit 0
}
if (-not $env:WINDOWS_CERTIFICATE_PFX -or -not $env:WINDOWS_CERTIFICATE_PASSWORD) { throw 'Both Windows signing secrets are required.' }
if ($env:CI -ne 'true') { throw 'Signing script requires an ephemeral CI runner.' }
$signTool = Get-ChildItem 'C:\Program Files (x86)\Windows Kits\10\bin\*\x64\signtool.exe' | Sort-Object FullName | Select-Object -Last 1
if (-not $signTool) { throw 'Windows SDK signtool.exe not found.' }
$certificate = Join-Path $env:RUNNER_TEMP ('mitmany-' + [guid]::NewGuid().ToString() + '.pfx')
try {
  [IO.File]::WriteAllBytes($certificate, [Convert]::FromBase64String($env:WINDOWS_CERTIFICATE_PFX))
  & $signTool.FullName sign /fd SHA256 /td SHA256 /tr http://timestamp.digicert.com /f $certificate /p $env:WINDOWS_CERTIFICATE_PASSWORD $Executable
  if ($LASTEXITCODE -ne 0) { throw 'Authenticode signing failed.' }
  & $signTool.FullName verify /pa /v $Executable
  if ($LASTEXITCODE -ne 0) { throw 'Authenticode verification failed.' }
  'Windows: Authenticode signed and timestamped; signature verification passed.' | Set-Content $StatusPath
} finally { Remove-Item $certificate -ErrorAction SilentlyContinue }
