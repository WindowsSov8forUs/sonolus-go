param(
    [string]$Baseline,
    [string]$Output
)

$ErrorActionPreference = "Stop"
$temporary = Join-Path ([System.IO.Path]::GetTempPath()) ("sonolus-go-benchmark-" + [guid]::NewGuid().ToString("N"))
try {
    New-Item -ItemType Directory -Path $temporary | Out-Null
    $benchmarks = @(go test ./internal/compiler -run '^$' -list '^Benchmark(CompilerStages|CompileAll)$') | Where-Object { $_ -match '^Benchmark' }
    if ($LASTEXITCODE -ne 0) {
        throw "benchmark discovery failed with exit code $LASTEXITCODE"
    }
    $rows = foreach ($benchmark in $benchmarks) {
        foreach ($sample in 1..5) {
            $sampleFile = Join-Path $temporary "$benchmark-$sample.txt"
            go test ./internal/compiler -run '^$' -bench "^$benchmark$" -benchmem -benchtime=1x -count=1 | Tee-Object -FilePath $sampleFile | Out-Null
            if ($LASTEXITCODE -ne 0) {
                throw "$benchmark sample $sample failed with exit code $LASTEXITCODE"
            }
            foreach ($line in Get-Content -LiteralPath $sampleFile) {
                if ($line -match '^(Benchmark\S+)-\d+\s+\d+\s+([0-9.]+) ns/op.*?([0-9.]+) B/op\s+([0-9.]+) allocs/op') {
                    [pscustomobject]@{ Name = $Matches[1]; Time = [double]$Matches[2]; Bytes = [double]$Matches[3]; Allocs = [double]$Matches[4] }
                }
            }
        }
    }
    $summary = foreach ($group in $rows | Group-Object Name) {
        $times = @($group.Group.Time | Sort-Object)
        $bytes = @($group.Group.Bytes | Sort-Object)
        $allocs = @($group.Group.Allocs | Sort-Object)
        [pscustomobject]@{ Name = $group.Name; TimeNs = $times[[int]($times.Count / 2)]; Bytes = $bytes[[int]($bytes.Count / 2)]; Allocs = $allocs[[int]($allocs.Count / 2)] }
    }
    if ($Baseline) {
        $previous = Get-Content -LiteralPath $Baseline -Raw | ConvertFrom-Json
        foreach ($item in $summary) {
            $old = $previous | Where-Object Name -eq $item.Name
            if ($old) {
                $item | Add-Member TimeChangePercent ((($item.TimeNs / $old.TimeNs) - 1) * 100)
                $item | Add-Member AllocationChangePercent ((($item.Allocs / $old.Allocs) - 1) * 100)
            }
        }
    }
    $json = $summary | ConvertTo-Json -Depth 4
    if ($Output) { [System.IO.File]::WriteAllText((Join-Path (Get-Location) $Output), $json + "`n", [System.Text.UTF8Encoding]::new($false)) } else { $json }
} finally {
    Remove-Item -LiteralPath $temporary -Recurse -Force -ErrorAction SilentlyContinue
}
