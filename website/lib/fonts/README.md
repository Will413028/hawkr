# Fonts

## jf-openhuninn 2.0: complete, unicode-ranged delivery

The brand still uses the original rounded outlines from justfont's
[jf-openhuninn v2.0](https://github.com/justfont/open-huninn-font/tree/dc5538c28ec309b6f6b51997bbfcdb531b9797a8).
The old CC BY-ND note was incorrect: this exact version is under
[SIL OFL 1.1](https://github.com/justfont/open-huninn-font/blob/dc5538c28ec309b6f6b51997bbfcdb531b9797a8/license.txt).
Derived files use the primary/internal family name **Hawkr Rounded** to
respect the reserved names. The license and upstream notices are retained,
including the complete license embedded in every WOFF2 file.

### Runtime ownership

- `styles/foundation/fonts.css` defines the existing `--font-openhuninn` token
  through `hk-font-rounded` on `<html>`, with the previous adjusted Arial fallback.
- `lib/fonts/openhuninn.ts` is only the stable root-class bridge, not a JS font
  loader. Roboto and Zen Maru Gothic continue using `next/font/local`.
- `styles/foundation/openhuninn.generated.css` owns the same-family
  `@font-face` rules and exact, disjoint `unicode-range` declarations.
- `assets/fonts/hawkr-rounded-v2/` contains reviewed/generated WOFF2 files,
  the pinned license, and the source/coverage/artifact manifest. CSS-relative
  imports let Next fingerprint the font assets under `/_next/static/`; no
  external font host or new CDN/cache policy is required.
- Use `font-display: swap` for every brand shard so late glyphs replace their
  fallback. Keep no font preload, the original 400–900 weight range, font
  stacks, and fallback metrics. Do not request fonts from JavaScript or
  preload all shards.

On 2026-09-09, a controlled Chrome reproduction with the base face already
loaded and a delayed cold shard showed why the former `optional` policy was
unsuitable: in `新增貼文`, `新貼文` used the rounded base while `增` stayed on
fallback even after its shard arrived. `swap` lets the heading converge to the
same rounded family. A slow visit can still show fallback temporarily and swap
once the font arrives; neither complete cmap coverage nor a successful font
request alone proves that the browser has replaced the rendered fallback.

The coverage union is the complete upstream **11,345 Unicode codepoints**.
All **2,698** codepoints in the former 481,748-byte subset remain covered.
Product names, seller names, chat text, and other runtime content select
additional shards natively; they do not require a new build-time text scan.
A character that the upstream font itself does not contain (for example 𠮷)
still uses the existing OS fallback. This is full upstream coverage, not a
claim to cover all Unicode or every Han character.

Latin and supported combining characters are kept together for mixed-language
shaping. The core-codepoint list is versioned, selected from production UI,
and not regenerated on ordinary application builds. It is a delivery priority,
not an allowlist that can remove any glyph from the complete union.

### Post-build runtime guard

After a normal `pnpm build`, run the independent manual gate:

```sh
pnpm fonts:check-build
```

It follows the stylesheets referenced by `.next/server/app/index.html` and
verifies the root class, compiled token bridge, all 80 native font faces, each
compiled WOFF2's content hash and manifest coverage, and the absence of a brand
font preload or the legacy subset. This gate is intentionally not appended to
the `build` script and is not an automatic CI step yet.

During the 2026-09-05 migration, one incremental production build reused a
Turbopack persistent-cache graph that omitted the newly added nested CSS import:
the source PostCSS result was correct, but the compiled homepage CSS contained
neither the token bridge nor any font face. A fresh build after moving only
`.next/cache/turbopack` aside restored the CSS and all 80 compiled WOFF2 assets.
Treat a guard failure as a stale or invalid build artifact to diagnose; the
guard never deletes caches or rewrites output itself.

### Development server recovery

`next dev` has a **separate** persistent compilation cache at
`.next/dev/cache/turbopack`; refreshing the production build cache does not
refresh it. A normal server restart also reuses this development cache.
See the installed Next.js `turbopackFileSystemCache` guide or the
[official cache documentation](https://nextjs.org/docs/app/api-reference/config/next-config-js/turbopackFileSystemCache).

During the initial font migration, `fonts.css` was wired into the running dev
server before `openhuninn.generated.css` existed. The failed import was cached;
creating the generated file and restarting later did not invalidate that old
result. Complete generation before introducing a new import into a running
server. Ordinary regeneration already installs the font assets before replacing
the generated CSS.

If development reports `Can't resolve './openhuninn.generated.css'`:

1. First verify the file exists and run
   `node --import tsx --test tests/font-loading-contract.test.ts`. If files are
   actually missing, restore/regenerate the complete reviewed artifact set;
   removing the font import is not a fix.
2. If the files/contracts are correct but the running server still gives that
   resolution error, stop this project's `pnpm dev` completely.
3. From the repository directory, move **only** the development compiler cache
   to a unique temporary backup, then restart normally:

   ```sh
   font_dev_cache_backup=$(mktemp -d /tmp/hawkr-dev-font-cache.XXXXXX)
   mv .next/dev/cache/turbopack "$font_dev_cache_backup/turbopack"
   pnpm dev
   ```

4. Verify the HTTPS page returns 200, the browser has no build error overlay,
   and a heading actually renders with `Hawkr Rounded`. A successful
   production build alone is not proof that the dev server has recovered.

Do not remove the entire `.next` directory, touch another project's server,
or permanently disable filesystem caching for this one-time migration failure.
The backup is recoverable and should not be restored into a running dev server.

### Regenerate and verify

Generation is an explicit maintenance step. Normal Next/OpenNext builds use
the checked-in artifacts and need neither Python nor network access.
Use an isolated Python environment with the exact versions in
`scripts/fonts/requirements.txt`; the generator refuses version drift.

Download the upstream TTF and `license.txt` from the immutable commit above,
not from `master`. The generator verifies their exact sizes/hashes before
writing anything:

- TTF SHA-256: `eec8b0b68c34b9166ae37bed839b6126116b225d97f21a5954bb542b9fd1e68c`
- License SHA-256: `c1a67a328fa5ce1f1f205fa9481d15fe1f05a27e6e4076919c4de85385e6f521`

```sh
/path/to/font-venv/bin/python scripts/fonts/generate_openhuninn.py generate \
  --source /path/to/jf-openhuninn-2.0.ttf \
  --license /path/to/license.txt \
  --core-file scripts/fonts/openhuninn-v2-core.txt \
  --output-dir assets/fonts/hawkr-rounded-v2 \
  --css-output styles/foundation/openhuninn.generated.css \
  --bucket-mode ordinal --bucket-size 128
```

Replace `generate` with `check` and use the same arguments to rebuild in a
temporary directory and byte-compare all artifacts. Verification checks actual
font cmaps, complete/disjoint coverage, original layout metrics, embedded names,
license metadata, and deterministic output. The Node suite separately checks
the pinned complete-cmap digest, CSS/manifest agreement, file hashes/WOFF2
headers, root token ownership, swap loading, and the old fallback metrics.

Do not tune the core only to a Lighthouse screenshot, live customer names, or
today's catalog. Preserve a stable production-UI core, include all seasonal
variants, and measure additional-shard requests on runtime canaries and
multiple routes. A smaller first download is not enough if cold/slow visits
mix glyph styles, jump in width, or turn dynamic headings into many requests.

### Admin display subset

`admin/src/app/fonts/hawkr-rounded-2.0-subset.woff2` is a single-file subset of
the same pinned upstream font, used only by the admin app. It has the same
`Hawkr Rounded` family name, license embedding, name validation and subsetting
options as the shards. Its codepoints are the reviewed list in
`scripts/fonts/hawkr-rounded-admin-subset.txt`. Regenerate or verify it with the
same upstream files and venv as above; do not edit the output by hand:

```sh
/path/to/font-venv/bin/python scripts/fonts/generate_openhuninn.py generate-subset \
  --source /path/to/jf-openhuninn-2.0.ttf \
  --license /path/to/license.txt \
  --codepoint-file scripts/fonts/hawkr-rounded-admin-subset.txt \
  --output ../admin/src/app/fonts/hawkr-rounded-2.0-subset.woff2
```

Use `check-subset` with the same arguments to rebuild in a temporary file,
validate its names and byte-compare it with the checked-in file.

No pre-generation subset of the upstream font is kept in the repository. The
Node suite's pinned complete-cmap digest and the generator's complete/disjoint
coverage check are the coverage floor. Old fixed-string canaries include
傷嘴扔歇泛癒逸 and 幸運籤餅; newly covered runtime canaries include
彧祐萱昀堃𪜶.

## Roboto + Zen Maru Gothic (self-hosted Google fonts)

`files/roboto-latin.woff2` and `files/zen-maru-gothic-latin-{400,500,700,900}.woff2` are the **Latin-subset** WOFF2 that Google Fonts serves for these families. They are vendored — not loaded via `next/font/google` — because the Google loader makes Turbopack fetch the font at compile/request time, which flakes in CI / offline builds (`Can't resolve '@vercel/turbopack-next/internal/font/google/font'`, [vercel/next.js#61886](https://github.com/vercel/next.js/discussions/61886)). Vendoring removes the network dependency and keeps builds deterministic; the browser-side result is identical (`next/font` self-hosts either way).

- **Roboto** is a single variable font (wght axis 100–900), so one file covers every weight — `lib/fonts/roboto.ts` declares `weight: "100 900"`.
- **Zen Maru Gothic** has discrete weights (not variable), so there is one file per weight in `lib/fonts/zenMaruGothic.ts`.

### Regenerate

Rerun only to pick up a new Google Fonts revision or to add a weight. The files are the exact Latin-subset WOFF2 Google serves, so output is byte-stable across runs unless Google bumps the font version.

```bash
# Requires network. UA must look like a modern Chrome so Google returns
# woff2 (not ttf). Writes into lib/fonts/files/.
python3 - <<'PY'
import re, os, urllib.request
UA = ("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36")
OUT = "lib/fonts/files"; os.makedirs(OUT, exist_ok=True)
# (slug, css2 family query, weights to keep, subset range, single-file?)
JOBS = [
    ("zen-maru-gothic", "Zen+Maru+Gothic:wght@400;500;700;900", ["400","500","700","900"], "latin", False),
    ("roboto",          "Roboto:wght@400;500;700",              ["400"],                    "latin", True),
]
def fetch(u): return urllib.request.urlopen(
    urllib.request.Request(u, headers={"User-Agent": UA}), timeout=30).read()
blk = re.compile(r"/\*\s*([a-z0-9-]+)\s*\*/\s*(@font-face\s*\{.*?\})", re.S)
wre = re.compile(r"font-weight:\s*(\d+)"); ure = re.compile(r"url\((https://[^)]+\.woff2)\)")
for slug, fam, weights, rng, single in JOBS:
    css = fetch(f"https://fonts.googleapis.com/css2?family={fam}&display=swap").decode()
    byw = {wre.search(b).group(1): ure.search(b).group(1)
           for lbl, b in blk.findall(css) if lbl == rng and wre.search(b) and ure.search(b)}
    for w in weights:
        fn = f"{OUT}/{slug}-{rng}.woff2" if single else f"{OUT}/{slug}-{rng}-{w}.woff2"
        open(fn, "wb").write(fetch(byw[w])); print("OK", fn)
PY
```

Roboto's variable font returns the same file for every weight, so the script keeps just one (`-latin.woff2`).
