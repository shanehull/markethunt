# markethunt

Country valuations and macro factors, ranked by the conditions a long-horizon buyer wants. One Go binary reads Aswath Damodaran's country equity multiples and a handful of World Bank series, scores every market, and renders one sortable table at markethunt.shanehull.com.

A bargain hunt: hunt where the crowd is most pessimistic, and let cheap valuations carry the ranking while the macro data guards against the countries that are cheap for a reason.

A research tool, not financial advice. It ranks conditions, it does not time markets.

## Ranking

Countries are ranked by a **valuation-led composite**. Each condition is a cross-sectional percentile rank; the composite is the weighted mean over the conditions a country has data for, so a missing factor is not penalised.

| Condition   | What it measures                                                                                 | Source     | Weight |
| ----------- | ------------------------------------------------------------------------------------------------ | ---------- | -----: |
| EV/EBIT     | Median EV/EBIT from Damodaran's latest snapshot. Capital-structure and tax neutral, so it leads.   | Damodaran  |   0.30 |
| P/B         | Median price-to-book from the same snapshot.                                                       | Damodaran  |   0.20 |
| Growth      | Five-year average annual GDP growth.                                                               | World Bank |   0.15 |
| Savings     | Gross national savings as a share of GDP.                                                          | World Bank |   0.10 |
| Trade       | External balance on goods and services as a share of GDP.                                          | World Bank |   0.10 |
| Debt        | General government gross debt as a share of GDP, latest completed WEO year.                        | IMF        |   0.10 |
| Free market | Fraser Institute Economic Freedom of the World overall rating (0-10) and its five-year change.     | Fraser     |   0.05 |

Valuation carries half the score, so a cheap market can outweigh weak macro, but macro still matters: a market needs growth, savings, a trade surplus, and a manageable debt load to look attractive, and strong institutions help. The free-market reading is deliberately a soft signal, because a market that looks closed can still be a bargain on a qualitative read.

The table exports the current view, filters and sort included, as CSV from the toolbar.

## How it works

```
Damodaran .xls  ─┐
World Bank API  ─┤
Fraser .xlsx    ─┼─→  ingest  ─→  in-memory cache  ─→  ranking  ─→  htmx table
IMF SDMX        ─┘
```

- `internal/xls` reads the legacy `.xls` (OLE2 compound storage, BIFF8, SST) workbooks Damodaran publishes, using only the standard library.
- `internal/xlsx` reads the Office Open XML workbook Fraser publishes, also with only the standard library.
- `internal/source` has one adapter per provider: `damodaran.go` parses the latest country multiples, `worldbank.go` pulls growth, savings and trade, `fraser.go` pulls economic freedom, and `imf.go` pulls government debt. Every source is public and keyless.
- `internal/country` is the reference table that maps Damodaran's country labels to ISO3 codes and World Bank regions.
- `internal/ranking` is the pure scoring and ranking, no I/O.
- `internal/cache` holds the assembled snapshot in memory, no database or disk.
- `internal/web` is the dashboard: Go `html/template` plus htmx, one binary, no build step.

## Quick start

Requires Go 1.26.5, nothing else: no database, no third-party dependencies.

```bash
go run ./cmd/markethunt
```

Then open http://localhost:8080. The first run fetches everything, which takes a few seconds.

## Configuration

| Flag         | Env                | Default        | Meaning                                                   |
| ------------ | ------------------ | -------------- | --------------------------------------------------------- |
| `-addr`      | `MARKETHUNT_ADDR`  | `:8080`        | HTTP listen address                                       |
| `-refresh`   |                    | `1h`           | Re-ingest interval (0 disables)                           |
| `-cache`     | `MARKETHUNT_CACHE` | user cache dir | Cache directory for downloaded payloads (empty disables)  |
| `-cache-ttl` |                    | `24h`          | How long a downloaded payload is reused before refetching |

## Operations

- `/healthz` is liveness: 200 while the process is up.
- `/readyz` is readiness: 503 until the first ingest lands, then 200.
- The process exits when its parent exits (PID 1 is exempt), so a killed `air`, shell, or supervisor cannot leave an orphan holding the port. SIGINT, SIGTERM, and SIGHUP drain in-flight requests.
- The container runs as UID 65534 (nobody), not root.

## Development

```bash
air                     # hot reload
golangci-lint run ./...
go test ./...
go run ./cmd/sri        # verify the pinned CDN SRI hashes
```

`mise` pins Go, golangci-lint, and air. The server sets read, write and idle timeouts, logs every request with `slog`, and drains in-flight requests on SIGINT and SIGTERM.

## Security

- A per-request Content Security Policy with script and style nonces, SRI hashes for the pinned CDN scripts, `script-src-attr 'none'`, and no `unsafe-inline`. The document's stylesheet is inline and carries the style nonce; swapped fragments contain no inline styles at all (bar widths are SVG attributes), so they do not need one. htmx's injected indicator stylesheet is disabled with `<meta name="htmx-config">`, and `style` is dropped from htmx's `attributesToSettle` so it never re-applies a `style` attribute when settling by `id`; `connect-src` allows the CDN only so devtools can fetch source maps.
- There is **no CORS header**, deliberately: the app is same-origin, and an absent `Access-Control-Allow-Origin` is what makes browsers block cross-origin reads. Adding `*` would be the vulnerability.
- Response headers: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY` plus `frame-ancestors 'none'`, `Referrer-Policy: no-referrer`, `Cross-Origin-Opener-Policy: same-origin`, `Cross-Origin-Resource-Policy: same-origin`, a restrictive `Permissions-Policy`, and HSTS.
- Routes are GET-only, so anything else is a 405. The HTTP server bounds header size and sets read-header, read, write, and idle timeouts; shutdown drains in-flight requests.
- No third-party Go dependencies. Frontend scripts are pinned by SRI in `internal/web/cdn.go`; run `go run ./cmd/sri` after bumping one.

## The universe

A market is ranked when it clears a liquidity floor and has the data the ranking needs:

- at least **30 listed firms** in Damodaran's sample, so a three-firm "market" cannot top the table on a noisy multiple, and
- a valuation multiple and a GDP growth history.

The ranked universe changes with provider coverage: a market is included when it clears the floor and has the required data. Countries Damodaran values but the World Bank does not cover are dropped, as are micro-markets below the floor.

## Data notes

- **Valuation is the latest Damodaran snapshot, never averaged.** His file gives each country a median EV/EBIT and a median price-to-book. The two are scored as separate factors, so a market cheap on earnings but expensive on book is not flattened into a single number. EV/EBIT carries the larger weight because it is capital-structure and tax neutral, which matters across countries. The annual archive files are no longer fetched: they existed only to compute multi-year averages.
- **This is not MSCI-style CAPE or CAPB.** Those average ten years of inflation-adjusted *aggregate* earnings or book value. Damodaran publishes one-year *firm medians*, which is a different construct and not comparable. Siblis Research publishes a keyless, genuinely cyclically adjusted CAPE for a handful of markets, but no country-level P/B, so broad coverage won here.
- **P/FCF is not here.** No keyless source publishes country-level price-to-free-cash-flow. Damodaran's country file carries price-to-book, EV/EBIT, EV/EBITDA, price-to-earnings and dividend yield, but no free-cash-flow multiple, so EV/EBIT is the cash-flow-adjacent measure.
- **Free-market conditions come from Fraser, not Heritage.** Heritage blocks automated clients and has omitted Hong Kong from recent editions. The World Bank governance indicators are a narrower proxy. Fraser's `efotw.org` master workbook is keyless and ISO-coded, covers most of the universe including Hong Kong and Taiwan, and carries the full history, so the factor is the overall EFW rating (0-10) plus a five-year change. Its report lags the valuation data by about a year; it is a slow-moving institutional measure. Markets Fraser does not cover drop the factor and renormalise.
- **Debt comes from the official IMF SDMX API.** General government gross debt as a share of GDP, annual, from the World Economic Outlook. The WEO carries projection years, so only completed years are kept; projections are never used. Coverage is broad: where the IMF omits a market, the factor is dropped and the weights renormalise. DBnomics is not used.
- **Growth is the five-year average annual GDP growth.** It is scored. A country with a short or gappy history averages what it has, and the years used are shown in the country panel. The standard deviation of that growth is shown there too, for context, but is not scored. Five years keeps the reading current: a ten-year average lags a turning point by roughly five years, so it reads a market that has just slumped, or just started to recover, as its old decade.
- **Multiples are Damodaran's, and inherit his sample.** Median multiples are computed across the listed firms he covers, not a float-adjusted index, so a market dominated by a handful of large caps reads differently from an index multiple. EV/EBIT medians also include loss-making firms, whose negative EBIT pulls the median down.
- **Every fetch replaces the snapshot**, so repeated runs are idempotent. There is no persistence of the ranking; the data comes from the providers each time.
- **Freshness is bounded by the cache TTL.** Downloaded payloads are reused for `-cache-ttl` (24h by default), then refetched. The refresher runs hourly, which means the ranking is never more than about 25 hours behind the providers.

## Disclaimer

A research tool, not financial advice. Equity multiples are published by Aswath Damodaran; macro and governance data by the World Bank; their terms govern use.

## License

MIT. See [LICENSE](LICENSE).
