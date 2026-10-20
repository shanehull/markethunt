# AGENTS.md — markethunt

Country valuations and macro factors, ranked for a long-horizon buyer. Read [README.md](README.md) for what it does and how it is wired.

It is a research tool. Do not write copy that implies it can time a market or that a high rank is a recommendation.

## Commands

```bash
go run ./cmd/markethunt   # serve the dashboard on :8080
go build ./...            # builds clean (no dependencies)
go vet ./...              # always clean
go test ./...             # unit tests, offline, no network
go run ./cmd/sri          # verify the pinned htmx SRI hash
golangci-lint run ./...   # house lint
```

## Conventions

Mirror the house style established in housecycle, lodestar, go-nem, and go-jev.

- All Go passes `golangci-lint run ./...` with the default config, and `gofmt` before commit.
- No `any` outside genuine decode boundaries (JSON, heterogeneous provider payloads). Provider responses are typed structs with JSON tags.
- Prefer the standard library. Keep dependencies minimal. There are no third-party dependencies; the `.xls` reader and the URL helpers are hand-rolled against the stdlib rather than pulling a library.
- Every public method takes `ctx context.Context` first.
- The HTTP server sets read, write, and idle timeouts, logs every request, and drains in-flight requests on SIGINT and SIGTERM.
- Structured logging with `slog`. Prose in docs is not hard-wrapped: one line per paragraph.
- The binary exits when its parent exits (PID 1 is exempt), so a killed supervisor cannot leave an orphan holding the port. `/livez` is liveness; `/readyz` is readiness.
- Commit messages follow conventional commits. release-please generates the changelog. Do not commit unless the user asks; the user commits.

## Layout

```
cmd/markethunt        binary and wiring
cmd/sri               print and verify CDN script SRI hashes
internal/country      reference table: Damodaran label -> ISO3 and region
internal/model        shared types: valuation, macro, scores, snapshot
internal/xls          stdlib OLE2 + BIFF8 reader for legacy .xls workbooks
internal/xlsx         stdlib OOXML reader for .xlsx workbooks
internal/source       providers -> Dataset (damodaran.go, worldbank.go, fraser.go, imf.go)
internal/analytics    pure maths, no I/O
internal/ranking      pillar scoring and the ranked snapshot
internal/cache        in-memory snapshot (no database)
internal/ingest       fetch -> merge -> rank -> store
internal/web          templates, static assets, HTTP server
```

## Working on the ranking

The scoring lives in `internal/ranking/ranking.go`. Factor weights and the liquidity floor (`MinFirms`) are constants at the top of that file; tune them there. Valuation leads the composite (EV/EBIT 0.30, P/B 0.20) but the macro factors still score (growth 0.10, savings 0.15, trade 0.10, debt 0.10, freedom 0.05). Adding a factor means a new score line in `score`, a field on `model.Scores`, a `collect` population, a column in `internal/web/templates/table.html`, and a row in the method table in `internal/web/templates/index.html`. The pure maths (percentile rank) lives in `internal/analytics`.

Sources must degrade gracefully. A fetch error for one source never fails the page; it shrinks the ranking to the markets that still have all seven factors. `ingest.Run` stores whatever a source returned even when it also returned an error, and the wiring logs the error.

Every fetch replaces the snapshot, so repeated runs are idempotent. There is no persistence; the data comes from the providers each time.

## Working on the Damodaran source

`internal/source/damodaran.go` fetches the current `countrystats.xls` only and reads each country's median P/B, EV/EBIT and trailing P/E. There is no time averaging and no archive fetch; P/B and EV/EBIT are scored as separate factors in `internal/ranking`.

Two traps, both load-bearing:

- **Never use the "Average of ..." columns.** Damodaran also publishes firm-level averages (the mean of every firm's multiple), which are wildly skewed: the US average price-to-book is above 30 against a median near 1.7. Only the `median(...)` columns are safe.
- **`pickEVEBIT` must exclude `ebitda`**, which shares the `ev/ebit` prefix.

The column pickers (`pickPB`, `pickEVEBIT`, `pickPE`) match on lower-cased header substrings, because the header text changes between releases.

## Testing

`go test ./...` runs offline with no network: `internal/xls` (a small fixture workbook in `testdata/`), `internal/source` (column picking and aggregation over synthetic snapshots), `internal/ranking`, and `internal/analytics`.

A live tier against the real providers is not wired. Validate provider changes by hand with `go run ./cmd/markethunt` and reading the logs, or by pointing a throwaway program at a downloaded file.

## Data sources

All free and keyless. Damodaran's country statistics are legacy `.xls`, parsed by the stdlib reader in `internal/xls` (OLE2 compound storage, BIFF8 records, the SST shared string table). The World Bank is queried through its v2 JSON API by indicator code: `NY.GDP.MKTP.KD.ZG` (growth), `NY.GNS.ICTR.ZS` (savings), and `NE.RSB.GNFS.ZS` (external balance). Fraser's Economic Freedom of the World master workbook is `.xlsx`, parsed by the stdlib reader in `internal/xlsx`; it is ISO-coded, includes Hong Kong and Taiwan, and supplies the overall rating and five-year change. Government debt comes from the official IMF SDMX API: `IMF.RES,WEO` key `.GGXWDG_NGDP.A`, with only completed years kept.

Heritage blocks automated clients and has omitted Hong Kong from recent editions; do not reintroduce it. Fraser's canonical host blocks bots too, but its `efotw.org` master file is keyless and is the source used. If a series is needed but no keyless source exists, say so in the README rather than substituting a short or fragile one.

## Frontend

Go `html/template` plus htmx, one binary, no node step. htmx and Chart.js load from jsDelivr and are pinned by SRI; local CSS and JS are embedded with `embed.FS` and served from `/static/`.

Security headers are set for every response, including a Content-Security-Policy with a per-request script nonce and style nonce. The main stylesheet is an inline `<style>` block in the document, authorised by that document's style nonce. Nothing in an htmx fragment uses an inline style: the score and pillar bars are SVG `<rect width="…">` attributes, and the drawer's chart data rides in a `data-charts` attribute. This matters because an htmx fragment is a separate response with a **different** nonce, which can never match the document's CSP; never put a nonced `<style>` or `<script>` in a swapped fragment. Third-party scripts are pinned by SRI; their URLs and sha384 hashes live in `internal/web/cdn.go` as the single source of truth, feeding both the `integrity` attributes and the CSP. After bumping a script, run `go run ./cmd/sri` to print the new hash (it flags drift), then update that file.

CORS is deliberately absent. The app is same-origin, and no `Access-Control-Allow-Origin` is what makes browsers block cross-origin reads; do not add a permissive header.

htmx injects an indicator stylesheet unless `includeIndicatorStyles` is false, which the `htmx-config` meta tag in `index.html` sets. That meta tag also drops `style` from `attributesToSettle` (leaving `class`, `width`, `height`): by default htmx settles by `id` and re-applies the old element's attributes with `setAttribute`, so on the second drawer open it copied a Chart.js canvas `style` onto the new canvas and violated the strict `style-src`. Keep both settings.

The table swaps through `/` (htmx receives the table fragment when the `HX-Request` header is present, the full page otherwise) and the country panel loads through `/country/{iso3}`. `/export.csv` returns the same filtered, sorted rows as a CSV download. Sorting and filtering are server-driven from query params, so every view is a shareable URL and a refresh works. The drawer's history charts are Chart.js canvases built from a `data-charts` attribute in `static/app.js`.
