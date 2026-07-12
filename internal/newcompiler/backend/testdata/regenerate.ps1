param(
    [string]$PythonRepo = (Join-Path $PSScriptRoot "../../../../../sonolus.py"),
    [string]$JavaScriptRepo = (Join-Path $PSScriptRoot "../../../../../sonolus.js-compiler")
)

$ErrorActionPreference = "Stop"
$pythonCommit = git -C $PythonRepo rev-parse HEAD
$jsCommit = git -C $JavaScriptRepo rev-parse HEAD
if ($pythonCommit -ne "1040bc0dcc116efdbca05f144edec302e839bcd3") {
    throw "sonolus.py commit is $pythonCommit; expected 1040bc0dcc116efdbca05f144edec302e839bcd3"
}
if ($jsCommit -ne "37b0eee5aa16d1e01973d33d625d86f5ef72d268") {
    throw "sonolus.js-compiler commit is $jsCommit; expected 37b0eee5aa16d1e01973d33d625d86f5ef72d268"
}

$repo = Resolve-Path (Join-Path $PSScriptRoot "../../../..")
Push-Location $repo
try {
    go test ./internal/newcompiler -run TestReferenceEngineDataGolden -update-reference -count=1
} finally {
    Pop-Location
}
