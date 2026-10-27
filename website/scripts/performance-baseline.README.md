# PERF-00 Lighthouse baseline CLI

`performance-baseline.mjs` creates reproducible, cold-navigation Lighthouse
evidence. It does not build, start, modify, or deploy Hawkr. Start the
target separately, and use only public, non-sensitive URLs.

## Tool contract

- Lighthouse is fixed to exactly `13.4.1`. The repository's current LHCI
  dependency resolves Lighthouse `12.6.1`, so point `--lighthouse-path` at a
  separately installed `13.4.1` CLI. A mismatch fails before opening any URL.
- Pass Chrome explicitly with `--chrome-path`. Both tool versions are read back
  and stored with the browser executable path in `manifest.json` and
  `summary.json`.
- Repeat `--ready-marker` once per URL, in the same order. After all cold runs
  for a URL, the CLI makes one auxiliary document request and checks HTTP 200,
  HTML, the exact public readiness attribute, and a served `data-dpl-id`
  matching `--build-marker` (or `DEPLOYMENT_ID`). This check occurs after the
  measured runs, so it cannot warm them—but it also cannot prove which document
  Lighthouse measured.
- The default is five sequential runs per URL. Every run gets a new temporary
  Chrome user-data directory, so browser cache/storage is cold without blocking
  page resources or changing application behaviour.
- Existing output directories must be empty. The CLI never deletes or replaces
  baseline evidence.

Example:

```sh
node scripts/performance-baseline.mjs \
  --url http://localhost:3000/ \
  --ready-marker 'data-hk-initial-content-ready="home"' \
  --url http://localhost:3000/market/products \
  --ready-marker 'data-hk-tab-content-ready="market"' \
  --build-marker local-08cae72f \
  --git-revision 08cae72f1ce12784d628e5ff8b2c13404006ae0d \
  --lighthouse-path /absolute/tool-cache/lighthouse-13.4.1/cli/index.js \
  --chrome-path /absolute/path/to/chrome \
  --output-dir output/performance-baseline/2026-09-05-local
```

Use `--dry-run` with the same arguments to inspect the complete fixed config.
It does not inspect binaries or Git, create files, launch Chrome, or send a
network request. `--help` has the same offline guarantee.

## Outputs

```text
<output-dir>/
  manifest.json
  summary.json
  01-<host>-<route>/
    run-01.report.json       raw Lighthouse result
    run-01.result.json       normalized timestamps, versions, scores and metrics
    run-01.report-0.trace.json
    run-01.report-0.devtoolslog.json
    run-01.lighthouse.log
    ...
```

`summary.json` reports medians only across `report-valid` runs. A report becomes
`report-invalid` when a requested run fails, redirects to another URL, uses a
different Lighthouse version, has a runtime error, lacks any requested category
score or metric numeric value, lacks saved assets, or fails the HTTP-status /
browser-console-error audits. `collectionSucceeded: true` means only that every
report and the auxiliary document check passed; CLI exit code 0 has the same
limited meaning. The resulting `verificationStatus` remains
`requires-measured-document-verification`, never “passed” or “eligible.” Before
acceptance, manually inspect every run's filmstrip and trace and confirm that
the measured representative data/content page is the intended page. Do not
count a CORS failure, challenge, empty, example-only, or wrong fixture page.

## Deliberate non-scope

This is a Lighthouse **initial navigation** baseline. The plan's real 30-second
interaction journey is a separate browser acceptance suite covering SPA route
readiness, chat, Hawkr Agent streaming, and server-confirmed add-to-cart. This
CLI neither performs those interactions nor infers their latency from a page
score.
