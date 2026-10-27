#!/usr/bin/env python3
"""Build reproducible, unicode-ranged Hawkr Rounded webfont shards.

The upstream font is deliberately not stored in this repository. Generation is
an explicit release step which requires the immutable upstream binary, its full
OFL text, and a reviewed/versioned core-codepoint file. Normal application
builds must never rescan product copy or repartition the font automatically.

Examples:

  python3 scripts/fonts/generate_openhuninn.py generate \
    --source /path/to/jf-openhuninn-2.0.ttf \
    --license /path/to/license.txt \
    --core-file scripts/fonts/openhuninn-v2-core.txt \
    --output-dir assets/fonts/hawkr-rounded-v2 \
    --css-output styles/foundation/openhuninn.generated.css

  python3 scripts/fonts/generate_openhuninn.py check [same arguments]

  python3 scripts/fonts/generate_openhuninn.py generate-subset \
    --source /path/to/jf-openhuninn-2.0.ttf \
    --license /path/to/license.txt \
    --codepoint-file scripts/fonts/hawkr-rounded-admin-subset.txt \
    --output ../admin/src/app/fonts/hawkr-rounded-2.0-subset.woff2

  python3 scripts/fonts/generate_openhuninn.py check-subset [same arguments]

The generated manifest records exact cmap ranges and hashes. ``check`` builds
the complete artifact set in a temporary directory, byte-compares it with the
checked-in files, and also verifies that shard cmaps are disjoint and their
union equals the complete upstream Unicode cmap.

``generate-subset`` / ``check-subset`` build the single-file admin display
subset from the same pinned source with the same subsetting options, renaming
and name-table validation, using a reviewed codepoint file instead of shards.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import logging
import os
import re
import shutil
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable, Sequence


SOURCE_VERSION = "2.0"
SOURCE_COMMIT = "dc5538c28ec309b6f6b51997bbfcdb531b9797a8"
SOURCE_GIT_BLOB_SHA1 = "71452b127afaef99e873fdbb20e2e4ca0dffb687"
SOURCE_SHA256 = "eec8b0b68c34b9166ae37bed839b6126116b225d97f21a5954bb542b9fd1e68c"
SOURCE_BYTES = 4_573_152
LICENSE_SHA256 = "c1a67a328fa5ce1f1f205fa9481d15fe1f05a27e6e4076919c4de85385e6f521"

FONTTOOLS_VERSION = "4.56.0"
BROTLI_VERSION = "1.2.0"

FAMILY_NAME = "Hawkr Rounded"
FONT_DISPLAY = "swap"
POSTSCRIPT_PREFIX = "HawkrRoundedV2"
SUBSET_SHARD_ID = "admin-subset"
LICENSE_FILENAME = "OFL-1.1.txt"
MANIFEST_FILENAME = "manifest.json"
FONT_FILENAME_PREFIX = "hawkr-rounded-v2"
DEFAULT_CSS_URL_PREFIX = "../../assets/fonts/hawkr-rounded-v2"

FALLBACK_METRICS = {
    "ascent_override": "80.97%",
    "descent_override": "11.04%",
    "line_gap_override": "0%",
    "size_adjust": "106.13%",
}

# Keep every source-supported Latin base/mark/precomposed glyph in one face.
# Splitting these ranges can break grapheme-cluster shaping and Tai-lo tone-mark
# fallback even when individual codepoints appear to be covered.
SHAPING_SAFE_RANGES = (
    (0x0000, 0x024F),
    (0x0300, 0x036F),
    (0x1E00, 0x1EFF),
)

RFN_PATTERN = re.compile(r"(?:open[\s_-]*huninn|huninn|粉圓)", re.IGNORECASE)
RANGE_TOKEN = re.compile(r"^U\+([0-9A-Fa-f]{1,6})(?:-(?:U\+)?([0-9A-Fa-f]{1,6}))?$")
GENERATED_FONT_PATTERN = re.compile(
    rf"^{re.escape(FONT_FILENAME_PREFIX)}-[a-z0-9-]+-[0-9a-f]{{12}}\.woff2$"
)


class FontBuildError(RuntimeError):
    """Raised when an input or generated artifact violates the release contract."""


@dataclass(frozen=True)
class BuildConfig:
    source: Path
    license_file: Path
    core_file: Path
    output_dir: Path
    css_output: Path
    css_url_prefix: str
    bucket_size: int
    bucket_mode: str


@dataclass(frozen=True)
class ShardPlan:
    shard_id: str
    codepoints: frozenset[int]


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def canonical_codepoint_digest(codepoints: Iterable[int]) -> str:
    payload = ",".join(f"{codepoint:X}" for codepoint in sorted(set(codepoints)))
    return sha256_bytes(payload.encode("ascii"))


def load_fonttools():
    try:
        import brotli
        import fontTools
        from fontTools import subset
        from fontTools.ttLib import TTFont
    except ImportError as error:
        raise FontBuildError(
            "fontTools with WOFF2 support and brotli are required. "
            f"Expected fontTools {FONTTOOLS_VERSION} and brotli {BROTLI_VERSION}."
        ) from error

    actual_fonttools = getattr(fontTools, "version", None) or getattr(
        fontTools, "__version__", "unknown"
    )
    actual_brotli = getattr(brotli, "__version__", "unknown")
    if actual_fonttools != FONTTOOLS_VERSION or actual_brotli != BROTLI_VERSION:
        raise FontBuildError(
            "Reproducible generation requires exact tool versions: "
            f"fontTools {FONTTOOLS_VERSION} (found {actual_fonttools}), "
            f"brotli {BROTLI_VERSION} (found {actual_brotli})."
        )
    return subset, TTFont


def validate_pinned_inputs(config: BuildConfig) -> None:
    validate_source_and_license(config.source, config.license_file)

    if not config.core_file.is_file():
        raise FontBuildError(f"Reviewed core-codepoint file not found: {config.core_file}")
    if config.bucket_size not in {128, 256, 512, 1024}:
        raise FontBuildError("Bucket size must be one of 128, 256, 512, or 1024.")
    if config.bucket_mode not in {"ordinal", "unicode-span"}:
        raise FontBuildError("Bucket mode must be ordinal or unicode-span.")
    if not config.css_url_prefix or "\n" in config.css_url_prefix:
        raise FontBuildError("CSS URL prefix must be a non-empty, single-line path.")


def validate_source_and_license(source: Path, license_file: Path) -> None:
    if not source.is_file():
        raise FontBuildError(f"Source font not found: {source}")
    if source.stat().st_size != SOURCE_BYTES:
        raise FontBuildError(
            f"Unexpected source size: expected {SOURCE_BYTES}, "
            f"found {source.stat().st_size}."
        )
    source_hash = sha256_file(source)
    if source_hash != SOURCE_SHA256:
        raise FontBuildError(
            f"Unexpected source SHA-256: expected {SOURCE_SHA256}, found {source_hash}."
        )

    if not license_file.is_file():
        raise FontBuildError(f"OFL text not found: {license_file}")
    license_hash = sha256_file(license_file)
    if license_hash != LICENSE_SHA256:
        raise FontBuildError(
            f"Unexpected OFL SHA-256: expected {LICENSE_SHA256}, found {license_hash}."
        )
    license_text = license_file.read_text(encoding="utf-8")
    required_license_phrases = (
        "SIL OPEN FONT LICENSE Version 1.1",
        "Reserved Font Names 'open huninn' and 'huninn'",
        "Copyright (c) 2010 MOTOYA CO.,LTD.",
        "Copyright (c) 2011-2016 The Varela Round Project Authors",
    )
    for phrase in required_license_phrases:
        if phrase not in license_text:
            raise FontBuildError(f"OFL text is missing required notice: {phrase}")


def unicode_cmap(font) -> dict[int, str]:
    mapping: dict[int, str] = {}
    for table in font["cmap"].tables:
        if not table.isUnicode():
            continue
        for codepoint, glyph_name in table.cmap.items():
            previous = mapping.setdefault(codepoint, glyph_name)
            if previous != glyph_name:
                raise FontBuildError(
                    f"Conflicting Unicode cmap mappings for U+{codepoint:04X}: "
                    f"{previous!r} and {glyph_name!r}."
                )
    if not mapping:
        raise FontBuildError("Source font has no Unicode cmap entries.")
    return mapping


def parse_core_file(path: Path) -> set[int]:
    """Parse reviewed literal text and/or U+XXXX[-YYYY] lines."""

    codepoints: set[int] = set()
    for line_number, raw_line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue

        tokens = line.split()
        if tokens and all(token.upper().startswith("U+") for token in tokens):
            for token in tokens:
                match = RANGE_TOKEN.fullmatch(token)
                if not match:
                    raise FontBuildError(
                        f"Invalid codepoint token in {path}:{line_number}: {token!r}"
                    )
                start = int(match.group(1), 16)
                end = int(match.group(2), 16) if match.group(2) else start
                if start > end or end > 0x10FFFF:
                    raise FontBuildError(
                        f"Invalid codepoint range in {path}:{line_number}: {token!r}"
                    )
                codepoints.update(range(start, end + 1))
        else:
            codepoints.update(ord(character) for character in raw_line)
    if not codepoints:
        raise FontBuildError(f"Core-codepoint file is empty: {path}")
    return codepoints


def compress_ranges(codepoints: Iterable[int]) -> list[tuple[int, int]]:
    ordered = sorted(set(codepoints))
    if not ordered:
        return []
    ranges: list[tuple[int, int]] = []
    start = previous = ordered[0]
    for codepoint in ordered[1:]:
        if codepoint == previous + 1:
            previous = codepoint
            continue
        ranges.append((start, previous))
        start = previous = codepoint
    ranges.append((start, previous))
    return ranges


def range_token(start: int, end: int) -> str:
    if start == end:
        return f"U+{start:04X}"
    return f"U+{start:04X}-{end:04X}"


def range_string(codepoints: Iterable[int]) -> str:
    return ", ".join(range_token(start, end) for start, end in compress_ranges(codepoints))


def make_plan(
    source_codepoints: set[int], core_codepoints: set[int], bucket_size: int, bucket_mode: str
) -> list[ShardPlan]:
    unsupported = core_codepoints - source_codepoints
    if unsupported:
        preview = ", ".join(f"U+{value:04X}" for value in sorted(unsupported)[:12])
        suffix = " …" if len(unsupported) > 12 else ""
        print(
            f"note: {len(unsupported)} reviewed core codepoints are absent from upstream "
            f"and will use the OS fallback ({preview}{suffix})",
            file=sys.stderr,
        )

    supported_core = core_codepoints & source_codepoints
    if not supported_core:
        raise FontBuildError("Core-codepoint file has no characters supported by the source font.")

    plans = [ShardPlan("base", frozenset(supported_core))]
    remaining = sorted(source_codepoints - supported_core)
    if bucket_mode == "ordinal":
        for index, start in enumerate(range(0, len(remaining), bucket_size)):
            plans.append(
                ShardPlan(
                    f"cold-{index:03d}",
                    frozenset(remaining[start : start + bucket_size]),
                )
            )
    else:
        buckets: dict[int, set[int]] = {}
        for codepoint in remaining:
            buckets.setdefault(codepoint // bucket_size, set()).add(codepoint)
        for bucket_number in sorted(buckets):
            lower = bucket_number * bucket_size
            upper = lower + bucket_size - 1
            plans.append(
                ShardPlan(
                    f"span-u{lower:04x}-u{upper:04x}",
                    frozenset(buckets[bucket_number]),
                )
            )

    validate_plan(plans, source_codepoints)
    return plans


def validate_plan(plans: Sequence[ShardPlan], source_codepoints: set[int]) -> None:
    seen: set[int] = set()
    for plan in plans:
        if not plan.codepoints:
            raise FontBuildError(f"Shard plan {plan.shard_id} is empty.")
        overlap = seen & plan.codepoints
        if overlap:
            first = min(overlap)
            raise FontBuildError(
                f"Shard plan {plan.shard_id} overlaps an earlier shard at U+{first:04X}."
            )
        seen.update(plan.codepoints)
    missing = source_codepoints - seen
    extra = seen - source_codepoints
    if missing or extra:
        raise FontBuildError(
            "Shard plans do not exactly cover the source cmap: "
            f"missing={len(missing)}, extra={len(extra)}."
        )


def source_metrics(font) -> dict[str, int]:
    return {
        "units_per_em": font["head"].unitsPerEm,
        "hhea_ascent": font["hhea"].ascent,
        "hhea_descent": font["hhea"].descent,
        "hhea_line_gap": font["hhea"].lineGap,
        "os2_typo_ascent": font["OS/2"].sTypoAscender,
        "os2_typo_descent": font["OS/2"].sTypoDescender,
        "os2_typo_line_gap": font["OS/2"].sTypoLineGap,
        "os2_win_ascent": font["OS/2"].usWinAscent,
        "os2_win_descent": font["OS/2"].usWinDescent,
    }


def rename_font(font, shard_id: str, complete_license_text: str) -> str:
    name_table = font["name"]
    compact_id = re.sub(r"[^A-Za-z0-9]", "", shard_id.title())
    postscript_name = f"{POSTSCRIPT_PREFIX}{compact_id}"
    unique_id = f"{POSTSCRIPT_PREFIX};{SOURCE_VERSION};{shard_id};{SOURCE_SHA256[:12]}"

    replacement = {
        1: FAMILY_NAME,
        2: "Regular",
        3: unique_id,
        4: FAMILY_NAME,
        5: f"Version {SOURCE_VERSION}",
        6: postscript_name,
        16: FAMILY_NAME,
        17: "Regular",
    }
    for name_id in replacement:
        name_table.removeNames(nameID=name_id)
    name_table.removeNames(nameID=13)
    name_table.removeNames(nameID=14)
    for platform_id, encoding_id, language_id in (
        (3, 1, 0x0409),
        (3, 1, 0x0404),
        (1, 0, 0),
    ):
        for name_id, value in replacement.items():
            name_table.setName(value, name_id, platform_id, encoding_id, language_id)
        name_table.setName(complete_license_text, 13, platform_id, encoding_id, language_id)
        name_table.setName(
            "http://scripts.sil.org/OFL", 14, platform_id, encoding_id, language_id
        )

    return postscript_name


def validate_names(font, expected_postscript_name: str, complete_license_text: str) -> None:
    names_by_id: dict[int, list[str]] = {}
    for record in font["name"].names:
        try:
            value = record.toUnicode()
        except UnicodeDecodeError as error:
            raise FontBuildError(f"Undecodable font name ID {record.nameID}.") from error
        names_by_id.setdefault(record.nameID, []).append(value)

    for name_id in (1, 3, 4, 6, 16):
        values = names_by_id.get(name_id, [])
        if not values:
            raise FontBuildError(f"Generated font is missing required name ID {name_id}.")
        for value in values:
            if RFN_PATTERN.search(value):
                raise FontBuildError(
                    f"Reserved Font Name remains in generated name ID {name_id}: {value!r}"
                )
    if set(names_by_id[1]) != {FAMILY_NAME} or set(names_by_id[16]) != {FAMILY_NAME}:
        raise FontBuildError("Generated family names are not uniformly renamed.")
    if set(names_by_id[6]) != {expected_postscript_name}:
        raise FontBuildError("Generated PostScript name does not match its shard ID.")

    license_texts = names_by_id.get(13, [])
    license_urls = names_by_id.get(14, [])
    if not license_texts or set(license_texts) != {complete_license_text}:
        raise FontBuildError("Generated font does not embed the complete OFL text in name ID 13.")
    if not license_urls or not any("scripts.sil.org/OFL" in value for value in license_urls):
        raise FontBuildError("Generated font is missing the OFL URL in name ID 14.")
    if not names_by_id.get(0):
        raise FontBuildError("Generated font is missing upstream copyright notices in name ID 0.")


def subset_font(
    source: Path,
    plan: ShardPlan,
    destination: Path,
    expected_metrics: dict[str, int],
    complete_license_text: str,
):
    subset, TTFont = load_fonttools()
    font = TTFont(source, recalcTimestamp=False)
    try:
        options = subset.Options()
        options.layout_features = ["*"]
        options.name_IDs = ["*"]
        options.name_legacy = True
        options.name_languages = ["*"]
        options.notdef_glyph = True
        options.notdef_outline = True
        options.recommended_glyphs = True
        options.recalc_timestamp = False
        # The legacy production subset was generated with --no-hinting. The
        # upstream v2.0 glyf table currently has zero instruction bytes, but
        # keeping this explicit prevents a future input from changing parity.
        options.hinting = False

        subsetter = subset.Subsetter(options=options)
        subsetter.populate(unicodes=plan.codepoints)
        subsetter.subset(font)
        postscript_name = rename_font(font, plan.shard_id, complete_license_text)
        font.flavor = "woff2"
        font.recalcTimestamp = False
        destination.parent.mkdir(parents=True, exist_ok=True)
        font.save(destination, reorderTables=True)
    finally:
        font.close()

    generated = TTFont(destination, recalcTimestamp=False, lazy=True)
    try:
        actual_codepoints = set(unicode_cmap(generated))
        if actual_codepoints != set(plan.codepoints):
            raise FontBuildError(
                f"Generated cmap mismatch for {plan.shard_id}: "
                f"expected={len(plan.codepoints)}, actual={len(actual_codepoints)}."
            )
        if source_metrics(generated) != expected_metrics:
            raise FontBuildError(f"Layout metrics changed in shard {plan.shard_id}.")
        validate_names(generated, postscript_name, complete_license_text)
        if generated["OS/2"].usWeightClass != 400:
            raise FontBuildError(f"Unexpected weight metadata in shard {plan.shard_id}.")
        if generated["OS/2"].fsType != 0:
            raise FontBuildError(f"Embedding restriction bits appeared in shard {plan.shard_id}.")
    finally:
        generated.close()
    return postscript_name


def wrap_css_ranges(codepoints: Iterable[int], indent: str = "\t") -> str:
    tokens = [range_token(start, end) for start, end in compress_ranges(codepoints)]
    lines: list[str] = []
    current = indent + "unicode-range: "
    for index, token in enumerate(tokens):
        piece = token + ("," if index < len(tokens) - 1 else ";")
        if len(current) + len(piece) + 1 > 110 and current.strip() != "unicode-range:":
            lines.append(current.rstrip())
            current = indent * 2 + piece + " "
        else:
            current += piece + " "
    lines.append(current.rstrip())
    return "\n".join(lines)


def render_css(shards: Sequence[dict[str, object]], url_prefix: str) -> bytes:
    prefix = url_prefix.rstrip("/")
    sections = [
        "/* Generated by scripts/fonts/generate_openhuninn.py. Do not edit. */",
        "",
    ]
    for shard in shards:
        sections.extend(
            (
                "@font-face {",
                f'\tfont-family: "{FAMILY_NAME}";',
                "\tfont-style: normal;",
                "\tfont-weight: 400 900;",
                f"\tfont-display: {FONT_DISPLAY};",
                f'\tsrc: url("{prefix}/{shard["file"]}") format("woff2");',
                wrap_css_ranges(shard["codepoint_values"]),
                "}",
                "",
            )
        )
    return "\n".join(sections).encode("utf-8")


def build_artifacts(config: BuildConfig, stage_dir: Path, stage_css: Path) -> dict[str, object]:
    validate_pinned_inputs(config)
    _, TTFont = load_fonttools()
    source_font = TTFont(config.source, recalcTimestamp=False, lazy=True)
    try:
        source_mapping = unicode_cmap(source_font)
        source_codepoints = set(source_mapping)
        metrics = source_metrics(source_font)
    finally:
        source_font.close()

    core_requested = parse_core_file(config.core_file)
    shaping_safe_codepoints = {
        codepoint
        for codepoint in source_codepoints
        if any(start <= codepoint <= end for start, end in SHAPING_SAFE_RANGES)
    }
    missing_shaping_codepoints = shaping_safe_codepoints - core_requested
    if missing_shaping_codepoints:
        preview = ", ".join(
            f"U+{codepoint:04X}" for codepoint in sorted(missing_shaping_codepoints)[:12]
        )
        raise FontBuildError(
            "Core file must keep all source-supported Latin and combining-mark "
            f"codepoints in the base shard; missing {len(missing_shaping_codepoints)} "
            f"({preview} …)."
        )
    plans = make_plan(source_codepoints, core_requested, config.bucket_size, config.bucket_mode)
    stage_dir.mkdir(parents=True, exist_ok=True)
    complete_license_text = config.license_file.read_text(encoding="utf-8")

    shard_records: list[dict[str, object]] = []
    for plan in plans:
        temporary_font = stage_dir / f".{plan.shard_id}.woff2"
        postscript_name = subset_font(
            config.source,
            plan,
            temporary_font,
            metrics,
            complete_license_text,
        )
        digest = sha256_file(temporary_font)
        filename = f"{FONT_FILENAME_PREFIX}-{plan.shard_id}-{digest[:12]}.woff2"
        final_font = stage_dir / filename
        temporary_font.replace(final_font)
        shard_records.append(
            {
                "id": plan.shard_id,
                "file": filename,
                "sha256": digest,
                "bytes": final_font.stat().st_size,
                "codepoints": len(plan.codepoints),
                "cmap_sha256": canonical_codepoint_digest(plan.codepoints),
                "unicode_ranges": range_string(plan.codepoints),
                "postscript_name": postscript_name,
                # Kept only while rendering CSS; removed from the persisted manifest.
                "codepoint_values": sorted(plan.codepoints),
            }
        )

    css_bytes = render_css(shard_records, config.css_url_prefix)
    stage_css.parent.mkdir(parents=True, exist_ok=True)
    stage_css.write_bytes(css_bytes)
    shutil.copyfile(config.license_file, stage_dir / LICENSE_FILENAME)

    persisted_shards = [
        {key: value for key, value in shard.items() if key != "codepoint_values"}
        for shard in shard_records
    ]
    manifest: dict[str, object] = {
        "schema_version": 1,
        "family": FAMILY_NAME,
        "source": {
            "name": "jf open huninn",
            "version": SOURCE_VERSION,
            "commit": SOURCE_COMMIT,
            "git_blob_sha1": SOURCE_GIT_BLOB_SHA1,
            "sha256": SOURCE_SHA256,
            "bytes": SOURCE_BYTES,
            "cmap_codepoints": len(source_codepoints),
            "cmap_sha256": canonical_codepoint_digest(source_codepoints),
        },
        "license": {
            "file": LICENSE_FILENAME,
            "sha256": LICENSE_SHA256,
            "name_id_13_preserved": True,
            "name_id_14_preserved": True,
        },
        "generator": {
            "script": "scripts/fonts/generate_openhuninn.py",
            "fonttools": FONTTOOLS_VERSION,
            "brotli": BROTLI_VERSION,
        },
        "partition": {
            "mode": config.bucket_mode,
            "bucket_size": config.bucket_size,
            "core_file_sha256": sha256_file(config.core_file),
            "requested_core_codepoints": len(core_requested),
            "supported_core_codepoints": len(core_requested & source_codepoints),
            "unsupported_core_codepoints": len(core_requested - source_codepoints),
            "core_cmap_sha256": canonical_codepoint_digest(core_requested & source_codepoints),
            "complete_union": True,
            "pairwise_disjoint": True,
        },
        "css": {
            "path": "styles/foundation/openhuninn.generated.css",
            "url_prefix": config.css_url_prefix,
            "sha256": sha256_bytes(css_bytes),
            "font_display": FONT_DISPLAY,
            "font_style": "normal",
            "font_weight": "400 900",
        },
        "fallback_metrics": FALLBACK_METRICS,
        "source_layout_metrics": metrics,
        "total_woff2_bytes": sum(int(shard["bytes"]) for shard in persisted_shards),
        "shards": persisted_shards,
    }
    manifest_bytes = (json.dumps(manifest, ensure_ascii=False, indent=2) + "\n").encode("utf-8")
    (stage_dir / MANIFEST_FILENAME).write_bytes(manifest_bytes)
    return manifest


def expected_output_names(manifest: dict[str, object]) -> set[str]:
    shards = manifest.get("shards")
    if not isinstance(shards, list):
        raise FontBuildError("Generated manifest has an invalid shards array.")
    names = {LICENSE_FILENAME, MANIFEST_FILENAME}
    for shard in shards:
        if not isinstance(shard, dict) or not isinstance(shard.get("file"), str):
            raise FontBuildError("Generated manifest has an invalid shard record.")
        names.add(shard["file"])
    return names


def compare_artifacts(config: BuildConfig, expected_dir: Path, expected_css: Path) -> None:
    if not config.output_dir.is_dir():
        raise FontBuildError(f"Generated font directory is missing: {config.output_dir}")
    if not config.css_output.is_file():
        raise FontBuildError(f"Generated CSS is missing: {config.css_output}")

    expected_manifest = json.loads((expected_dir / MANIFEST_FILENAME).read_text(encoding="utf-8"))
    names = expected_output_names(expected_manifest)
    failures: list[str] = []
    for name in sorted(names):
        expected = expected_dir / name
        actual = config.output_dir / name
        if not actual.is_file():
            failures.append(f"missing {actual}")
        elif expected.read_bytes() != actual.read_bytes():
            failures.append(f"content differs: {actual}")

    stale_fonts = sorted(
        path.name
        for path in config.output_dir.glob(f"{FONT_FILENAME_PREFIX}-*.woff2")
        if path.name not in names
    )
    failures.extend(f"stale generated shard: {name}" for name in stale_fonts)
    if expected_css.read_bytes() != config.css_output.read_bytes():
        failures.append(f"content differs: {config.css_output}")
    if failures:
        raise FontBuildError("Generated font artifacts are stale:\n- " + "\n- ".join(failures))


def install_artifacts(config: BuildConfig, stage_dir: Path, stage_css: Path, manifest) -> None:
    config.output_dir.mkdir(parents=True, exist_ok=True)
    config.css_output.parent.mkdir(parents=True, exist_ok=True)

    old_generated_fonts = {
        path.name for path in config.output_dir.glob(f"{FONT_FILENAME_PREFIX}-*.woff2")
    }
    new_names = expected_output_names(manifest)
    for name in sorted(new_names):
        source = stage_dir / name
        destination = config.output_dir / name
        temporary = destination.with_name(f".{destination.name}.tmp")
        shutil.copyfile(source, temporary)
        os.replace(temporary, destination)

    css_temporary = config.css_output.with_name(f".{config.css_output.name}.tmp")
    shutil.copyfile(stage_css, css_temporary)
    os.replace(css_temporary, config.css_output)

    for stale_name in sorted(old_generated_fonts - new_names):
        if GENERATED_FONT_PATTERN.fullmatch(stale_name):
            (config.output_dir / stale_name).unlink()


def build_single_subset(
    source: Path, license_file: Path, codepoint_file: Path, destination: Path
) -> str:
    """Build the one-file admin subset at destination; return its PostScript name."""

    validate_source_and_license(source, license_file)
    if not codepoint_file.is_file():
        raise FontBuildError(f"Reviewed codepoint file not found: {codepoint_file}")
    _, TTFont = load_fonttools()
    source_font = TTFont(source, recalcTimestamp=False, lazy=True)
    try:
        source_codepoints = set(unicode_cmap(source_font))
        metrics = source_metrics(source_font)
    finally:
        source_font.close()
    requested = parse_core_file(codepoint_file)
    unsupported = requested - source_codepoints
    if unsupported:
        preview = ", ".join(f"U+{codepoint:04X}" for codepoint in sorted(unsupported)[:12])
        raise FontBuildError(
            f"Codepoint file lists {len(unsupported)} codepoints the source lacks ({preview} …)."
        )
    return subset_font(
        source,
        ShardPlan(SUBSET_SHARD_ID, frozenset(requested)),
        destination,
        metrics,
        license_file.read_text(encoding="utf-8"),
    )


def run_subset_command(args: argparse.Namespace) -> None:
    output = args.output.resolve()
    with tempfile.TemporaryDirectory(prefix="hawkr-rounded-subset-") as temporary_root:
        staged = Path(temporary_root) / output.name
        postscript_name = build_single_subset(
            args.source.resolve(),
            args.license_file.resolve(),
            args.codepoint_file.resolve(),
            staged,
        )
        license_text = args.license_file.resolve().read_text(encoding="utf-8")
        if args.command == "check-subset":
            if not output.is_file():
                raise FontBuildError(f"Admin subset is missing: {output}")
            _, TTFont = load_fonttools()
            actual = TTFont(output, recalcTimestamp=False, lazy=True)
            try:
                validate_names(actual, postscript_name, license_text)
            finally:
                actual.close()
            if staged.read_bytes() != output.read_bytes():
                raise FontBuildError(f"Admin subset is stale: content differs: {output}")
            print(f"verified admin subset {output.name}, {output.stat().st_size} bytes")
        else:
            output.parent.mkdir(parents=True, exist_ok=True)
            temporary = output.with_name(f".{output.name}.tmp")
            shutil.copyfile(staged, temporary)
            os.replace(temporary, output)
            print(f"generated admin subset {output.name}, {output.stat().st_size} bytes")


def build_config(args: argparse.Namespace) -> BuildConfig:
    return BuildConfig(
        source=args.source.resolve(),
        license_file=args.license_file.resolve(),
        core_file=args.core_file.resolve(),
        output_dir=args.output_dir.resolve(),
        css_output=args.css_output.resolve(),
        css_url_prefix=args.css_url_prefix,
        bucket_size=args.bucket_size,
        bucket_mode=args.bucket_mode,
    )


def parse_arguments(argv: Sequence[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "command", choices=("generate", "check", "generate-subset", "check-subset")
    )
    parser.add_argument("--source", type=Path, required=True, help="Pinned upstream v2.0 TTF")
    parser.add_argument(
        "--license", dest="license_file", type=Path, required=True, help="Pinned upstream license.txt"
    )
    parser.add_argument(
        "--core-file",
        type=Path,
        help="Reviewed UTF-8 literal text and/or U+XXXX[-YYYY] codepoint list",
    )
    parser.add_argument("--output-dir", type=Path)
    parser.add_argument("--css-output", type=Path)
    parser.add_argument("--css-url-prefix", default=DEFAULT_CSS_URL_PREFIX)
    parser.add_argument("--bucket-size", type=int, default=256)
    parser.add_argument("--bucket-mode", choices=("ordinal", "unicode-span"), default="ordinal")
    parser.add_argument(
        "--codepoint-file",
        type=Path,
        help="generate-subset/check-subset: reviewed admin subset codepoint list",
    )
    parser.add_argument("--output", type=Path, help="generate-subset/check-subset: output file")
    args = parser.parse_args(argv)
    if args.command in ("generate", "check"):
        required = ("core_file", "output_dir", "css_output")
    else:
        required = ("codepoint_file", "output")
    missing = [f"--{name.replace('_', '-')}" for name in required if getattr(args, name) is None]
    if missing:
        parser.error(f"{args.command} requires {', '.join(missing)}")
    return args


def main(argv: Sequence[str] | None = None) -> int:
    logging.getLogger("fontTools.subset").setLevel(logging.ERROR)
    args = parse_arguments(argv or sys.argv[1:])
    if args.command in ("generate-subset", "check-subset"):
        try:
            run_subset_command(args)
            return 0
        except FontBuildError as error:
            print(f"error: {error}", file=sys.stderr)
            return 1
    config = build_config(args)
    try:
        with tempfile.TemporaryDirectory(prefix="hawkr-rounded-v2-") as temporary_root:
            temporary = Path(temporary_root)
            stage_dir = temporary / "fonts"
            stage_css = temporary / "openhuninn.generated.css"
            manifest = build_artifacts(config, stage_dir, stage_css)
            if args.command == "check":
                compare_artifacts(config, stage_dir, stage_css)
                print(
                    f"verified {len(manifest['shards'])} shards, "
                    f"{manifest['source']['cmap_codepoints']} cmap codepoints, "
                    f"{manifest['total_woff2_bytes']} bytes"
                )
            else:
                install_artifacts(config, stage_dir, stage_css, manifest)
                print(
                    f"generated {len(manifest['shards'])} shards, "
                    f"{manifest['source']['cmap_codepoints']} cmap codepoints, "
                    f"{manifest['total_woff2_bytes']} bytes"
                )
        return 0
    except FontBuildError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
