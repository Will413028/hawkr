# TGOS QueryAddr response schema notes

Four real captures against `QueryAddr` (via `tgos-proxy`, `oFuzzyType=0` unless noted):

- `tgos_queryaddr_response.json` — full match, `oIsOnlyFullMatch=false` (`oAddress=台北市大安區忠孝東路四段1號`)
- `tgos_queryaddr_response_fuzzy.json` — fuzzy match, `oIsOnlyFullMatch=false` (`oAddress=台北市大安區忠孝東路四段999號`, a real road with a nonexistent house number — TGOS fell back to nearest lower number)
- `tgos_queryaddr_response_noresult.json` — zero result, `oIsOnlyFullMatch=false` (`oAddress=火星市外星區銀河路一段1號`, a nonsense address)
- `tgos_queryaddr_response_validate_nomatch.json` — **the actual param combination `Validate` uses** (`oIsOnlyFullMatch=true`, same fuzzy-only address as above): confirms the server-side flag alone cleanly filters out the fuzzy match, `OutTotal:"0"`/`AddressList:[]`, no separate client-side `OutMatchType` check needed for `Validate`.

## Wire shape

HTTP GET on the ASMX endpoint (`/addrws/v40/QueryAddr.asmx/QueryAddr`) wraps the actual JSON in an XML string: `<string xmlns="http://tempuri.org/">{...}</string>`. No HTML/XML entity escaping was observed inside the string for this response (Chinese characters and the `\t` in `OutMatchCode` came through as literal UTF-8 / a real tab, not `&#9;`). Strip the `<?xml ...?>` prolog and the `<string ...>`/`</string>` wrapper to get the raw JSON — that's what's saved in the fixture.

`Info[]` echoes back every request parameter as `In<Name>` — but they're not uniformly booleans. Only the ones that were boolean *inputs* (`InIsOnlyFullMatch`, `InIsSupportPast`, `InIsShowCodeBase`, `InIsLock*`, `InCanIgnore*`) come back as stringified booleans (`"True"`/`"False"`, capitalized, not JSON `true`/`false`). The rest echo their actual type as a string regardless: `InAddress`/`InSRS`/`InFuzzyType` are descriptive strings, `InFuzzyBuffer`/`InReturnMaxCount` are numeric-strings (`"0"`, `"5"`). Only relevant if a future caller wants to sanity-check the round-trip; the actual result data is in `AddressList[]`.

## `AddressList[]` field → `pkg/geocode` domain type mapping

| TGOS field | Type in JSON | Maps to (`Suggestion`/`ValidationResult`) | Notes |
|---|---|---|---|
| `COUNTY` | string | `County` | e.g. `"臺北市"` — note **臺** not **台**; existing `pkg/geocode/parse.go` has a `normalizeCounty` helper (台/臺 dedupe) already built for Google's variant handling, reusable here |
| `TOWN` | string | `District` | e.g. `"大安區"` |
| `FULL_ADDR` | string | `FullAddress` | Full formatted address including 村里/鄰, e.g. `"臺北市大安區光武里27鄰忠孝東路四段1號"` |
| `X` | number (float) | `Lng` | Confirmed longitude (121.5xx — matches Taipei), consistent with `oSRS=EPSG:4326` request param |
| `Y` | number (float) | `Lat` | Confirmed latitude (25.0xx — matches Taipei) |
| `VILLAGE`, `NEIGHBORHOOD`, `ROAD`, `SECTION`, `LANE`, `ALLEY`, `SUB_ALLEY`, `TONG`, `AREA`, `NUMBER`, `PERIOD` | string | not currently in `Suggestion`/`ValidationResult` | Available if `Detail` needs to be built from structured parts instead of string-extracted from `FULL_ADDR` (TGOS gives clean structured components — likely more reliable than Google's `extractDetail` string-slicing fallback) |

## `Info[]` (request/response metadata, one element)

- `IsSuccess`: `"True"`/`"False"` (string, not bool) — means "the API call itself succeeded", **not** "a match was found". Stayed `"True"` in all three captures, including the zero-result one. Not useful as a match/precision signal — don't use it for that.
- `OutTotal`: string-encoded integer — count of `AddressList` entries. `"0"` in both zero-result captures, confirming `AddressList` truly empties out to `[]` (not `null`, not an omitted key) rather than the count disagreeing with the array.
- `OutMatchType`: only meaningful when `OutTotal != "0"` — confirmed values `"完全比對"` (full/exact match) vs `"模糊比對"` (fuzzy match). **Not reliable when `OutTotal == "0"`**: the two zero-result captures show *different* `OutMatchType` values in that state (`"完全比對"` for a nonsense address, `"模糊比對"` for a real-but-imprecise one) — it's just leftover/undefined text, not a signal. **`Validate` doesn't need to read this field at all**: setting `oIsOnlyFullMatch=true` on the request makes TGOS itself filter to full matches only — a fuzzy-only address comes back as `OutTotal:"0"`/empty `AddressList` (see `tgos_queryaddr_response_validate_nomatch.json`), so `len(AddressList) == 0` is a sufficient and simpler validity gate. Only `Suggest` (which wants fuzzy candidates too, so uses `oIsOnlyFullMatch=false`) would ever need to inspect `OutMatchType` per-item, and even there it's optional since `Suggest` returns all candidates regardless of match quality.
- `OutMatchCode`, `OutTraceInfo`: diagnostic strings (the latter is a human-readable trace of what matched/fell back, e.g. `"進行[找小於且最接近的門牌號碼]"` — useful for logs/debugging a bad address, not needed by the parser's return value).
