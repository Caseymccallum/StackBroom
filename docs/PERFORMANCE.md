# Scan performance measurements

**Measured:** 27 October 2026, Windows 10 Pro 19045, Go 1.27.0 build, Docker
Desktop 29.4.0 (local named pipe), one WSL2 distribution, ~400 processes.
**Method:** each `stackbroom scan --json` run reports `timings` per stage; values
below are from real runs, not estimates. A cold first run is excluded and called
out separately.

## Results

| Mode | Total (ms) | Processes (ms) | Docker (ms) | WSL (ms) | Analysis (ms) |
| --- | --- | --- | --- | --- | --- |
| Default (3 runs) | 1798 / 1969 / 1973 | 473 / 473 / 588 | 1278 / 1444 / 1334 | 45 / 50 / 49 | 0 / 0 / 1 |
| `--skip-docker` (2 runs) | 515 / 541 | 469 / 491 | skipped | 45 / 49 | 0 |
| `--no-command-lines` (2 runs) | 1356 / 1342 | 23 / 20 | 1289 / 1274 | 45 | 0 |

A cold start immediately after Docker Desktop or WSL had been idle was observed at
**12.8 s**, dominated by Docker's first response. This is environment warm-up, not
steady-state cost, and the per-command timeout still bounds it.

## What dominates

1. **Docker is ~70% of a warm scan** (~1.3 s). It issues six CLI calls
   (`context show`, `context inspect`, `version`, `ps`, `system df`, `image ls`);
   `system df` and `image ls` account for most of the time.
2. **Process collection is ~0.5 s**, of which the PowerShell/CIM command-line
   query is ~0.45 s.
3. **WSL is ~50 ms.**
4. **Analysis is ~1 ms**, even with thousands of processes (see
   `BenchmarkSummarizeLargeInventory`).

## Does a fast mode help?

| Change | Saves (warm) | Already available? |
| --- | --- | --- |
| Skip Docker entirely | ~1.3 s (≈70%) | `--skip-docker` |
| Skip the CIM command-line query | ~0.45 s (≈23%) | `--no-command-lines` |
| Skip Docker storage accounting only (`system df`, `image ls`) | ~0.8-1.0 s (inferred from per-call cost; not separately measured) | No |

**Conclusion:** a new `--fast` mode is **not justified for v0.1.1**. The existing
flags already cover the meaningful cases, and a partial Docker mode would produce
a confusing result: container data without storage data. If users ask for a faster
default, the next step would be to measure `system df` and `image ls` separately
and consider a `--no-storage` flag, not a vague "fast" mode.

## Reproducing

```powershell
1..3 | ForEach-Object {
  $r = .\bin\stackbroom.exe scan --json | ConvertFrom-Json
  "total={0} proc={1} docker={2} wsl={3} analysis={4}" -f `
    $r.durationMs, $r.timings.processesMs, $r.timings.dockerMs, $r.timings.wslMs, $r.timings.analysisMs
}
```

Caveat: Docker and CIM times depend on machine load, Docker Desktop state and the
number of processes. Treat the table as an order-of-magnitude baseline, not a
benchmark contract.
