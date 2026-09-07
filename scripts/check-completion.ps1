param([Parameter(Mandatory)][string]$Binary)
$ErrorActionPreference = 'Stop'
$completionPath = Join-Path ([System.IO.Path]::GetTempPath()) ('dmut-completion-' + [guid]::NewGuid().ToString('N') + '.ps1')
try {
    $completionLines = & $Binary --completion powershell
    if ($LASTEXITCODE -ne 0) { throw 'Could not generate PowerShell completion.' }
    [System.IO.File]::WriteAllText($completionPath, [string]::Join([char]10, $completionLines), [System.Text.UTF8Encoding]::new($false))
    $completionTokens = $null
    $completionErrors = $null
    [System.Management.Automation.Language.Parser]::ParseFile($completionPath, [ref]$completionTokens, [ref]$completionErrors) | Out-Null
    if ($completionErrors.Count) { throw ($completionErrors | Out-String) }
    . $completionPath
    $completionMatches = [System.Management.Automation.CommandCompletion]::CompleteInput('dmut --pre', 10, $null).CompletionMatches.CompletionText
    if ('--preview' -notin $completionMatches -or '--preview-limit' -notin $completionMatches) {
        throw 'PowerShell completion did not return the expected options.'
    }
    Write-Output 'PowerShell completion checks passed.'
} finally {
    Remove-Item -LiteralPath $completionPath -ErrorAction SilentlyContinue
}
