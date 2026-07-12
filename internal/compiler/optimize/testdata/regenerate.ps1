param(
    [string]$PythonCheckout = "../sonolus.py"
)

$ErrorActionPreference = "Stop"
$expected = "1040bc0dcc116efdbca05f144edec302e839bcd3"
$actual = (git -C $PythonCheckout rev-parse HEAD).Trim()
if ($actual -ne $expected) {
    throw "sonolus.py checkout is $actual; expected $expected"
}

$root = (Resolve-Path (Join-Path $PSScriptRoot "../../../..")).Path
$env:PYTHONPATH = (Resolve-Path $PythonCheckout).Path
$harness = Join-Path $root "internal/compiler/optimize/testdata/harness.py"
$output = python $harness
if ($LASTEXITCODE -ne 0) {
    throw "Python optimizer harness failed with exit code $LASTEXITCODE"
}
$generated = ($output -join "`n") | ConvertFrom-Json
$snapshot = [ordered]@{
    schemaVersion = 1
    pythonCommit = $expected
    ssaCases = $generated.ssaCases
    sccpCases = $generated.sccpCases
    fromSSACases = $generated.fromSSACases
}
$golden = Join-Path $PSScriptRoot "py_pass_golden.json"
[System.IO.File]::WriteAllText(
    $golden,
    (($snapshot | ConvertTo-Json -Depth 20) + "`n"),
    [System.Text.UTF8Encoding]::new($false)
)
