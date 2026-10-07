param(
    [string]$PythonCheckout = "../sonolus.py",
    [switch]$Historical
)

$ErrorActionPreference = "Stop"
$expected = if ($Historical) { "1040bc0dcc116efdbca05f144edec302e839bcd3" } else { "c45300d46ae53f659d71e0108216e39339434463" }
$actual = (git -C $PythonCheckout rev-parse HEAD).Trim()
if ($actual -ne $expected) {
    throw "sonolus.py checkout is $actual; expected $expected"
}

$root = (Resolve-Path (Join-Path $PSScriptRoot "../../../..")).Path
if (-not $Historical) {
    # Build the exact Git tree in isolation: the current optimizer needs Cython
    # extensions, and a source-only PYTHONPATH is insufficient.
    $temporary = Join-Path ([System.IO.Path]::GetTempPath()) ("sonolus-reference-" + [guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $temporary | Out-Null
    $archive = Join-Path $temporary "source.zip"
    git -C $PythonCheckout archive $expected --format=zip -o $archive
    if ($LASTEXITCODE -ne 0) { throw "Cannot archive Python reference" }
    python -m venv (Join-Path $temporary "venv")
    if ($LASTEXITCODE -ne 0) { throw "Cannot create reference environment" }
    $python = Join-Path $temporary "venv/Scripts/python.exe"
    if (-not (Test-Path $python)) { $python = Join-Path $temporary "venv/bin/python" }
    & $python -m pip install $archive
    if ($LASTEXITCODE -ne 0) { throw "Cannot build Python reference" }
    $savedPythonPath = $env:PYTHONPATH
    try {
        $env:PYTHONPATH = ""
        $output = & $python (Join-Path $PSScriptRoot "harness_pipeline.py") --current
        if ($LASTEXITCODE -ne 0) { throw "Current Python optimizer harness failed" }
        $snapshot = [ordered]@{
            schemaVersion = 1
            pythonCommit = $expected
            pipelineCases = ($output -join "`n") | ConvertFrom-Json
        }
        [System.IO.File]::WriteAllText((Join-Path $PSScriptRoot "py_current_golden.json"), (($snapshot | ConvertTo-Json -Depth 30) + "`n"), [System.Text.UTF8Encoding]::new($false))
    } finally { $env:PYTHONPATH = $savedPythonPath }
    Write-Host "Reference build environment: $temporary"
    return
}
$env:PYTHONPATH = (Resolve-Path $PythonCheckout).Path
$harness = Join-Path $root "internal/compiler/testdata/optimize/harness.py"
$output = python $harness
if ($LASTEXITCODE -ne 0) {
    throw "Python optimizer harness failed with exit code $LASTEXITCODE"
}
$generated = ($output -join "`n") | ConvertFrom-Json
$pipelineHarness = Join-Path $root "internal/compiler/testdata/optimize/harness_pipeline.py"
$pipelineOutput = python $pipelineHarness
if ($LASTEXITCODE -ne 0) {
    throw "Python optimizer pipeline harness failed with exit code $LASTEXITCODE"
}
$pipeline = ($pipelineOutput -join "`n") | ConvertFrom-Json
$snapshot = [ordered]@{
    schemaVersion = 4
    pythonCommit = $expected
    ssaCases = $generated.ssaCases
    sccpCases = $generated.sccpCases
    fromSSACases = $generated.fromSSACases
    pipelineCases = $pipeline
}
$golden = Join-Path $PSScriptRoot "py_pass_golden.json"
[System.IO.File]::WriteAllText(
    $golden,
    (($snapshot | ConvertTo-Json -Depth 20) + "`n"),
    [System.Text.UTF8Encoding]::new($false)
)
