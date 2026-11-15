#!/usr/bin/env python3
"""shaded jar 內含函式庫的漏洞門檻。

osv-scanner（scan source --no-resolve）只看 pom.xml 與 lockfile 寫出的座標，看不到被打包進別的 jar 的函式庫
（例如 io.grpc:grpc-netty-shaded 內含 netty、nacos-client 內含 guava）。這裡掃已解析的 runtime jar：

  - META-INF/*versions.properties（netty 格式：`<artifact>.version=<版本>`）→ io.netty:<artifact>
  - META-INF/maven/<group>/<artifact>/pom.properties，扣掉 jar 自己的那一筆 → 內含的 <group>:<artifact>

再以 OSV querybatch（ecosystem Maven）查漏洞。漏洞 id（或其別名）在 osv-scanner.toml 的 IgnoredVulns 且
ignoreUntil 未過 → 放行；其餘失敗；ignoreUntil 已過的視同未放行。

用法：
  shaded-jar-gate.py --jars <放 jar 的目錄> [--config osv-scanner.toml] [--today YYYY-MM-DD]
jar 目錄的產生：backend/services/java 下
  ./mvnw -B -q -DskipTests package dependency:copy-dependencies -DoutputDirectory=<目錄> \
      -DincludeScope=runtime -DexcludeGroupIds=io.github.will413028.hawkr
"""
import argparse
import datetime
import json
import re
import sys
import tomllib
import urllib.request
import zipfile
from pathlib import Path

OSV_BATCH = "https://api.osv.dev/v1/querybatch"
OSV_VULN = "https://api.osv.dev/v1/vulns/"
VERSIONS_RE = re.compile(r"^META-INF/(?:[^/]*\.)?versions\.properties$")
POM_PROPS_RE = re.compile(r"^META-INF/maven/([^/]+)/([^/]+)/pom\.properties$")
NETTY_VERSION_RE = re.compile(r"^(netty-[\w.-]+)\.version=(\S+)$")


def read_props(text: str) -> dict[str, str]:
    props = {}
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        k, v = line.split("=", 1)
        props[k.strip()] = v.strip()
    return props


def own_coordinates(jar_name: str, poms: list[tuple[str, str, str]]) -> tuple[str, str] | None:
    """jar 自己的座標：artifactId 是檔名開頭、且最長的那筆。"""
    best = None
    for group, artifact, _ in poms:
        if jar_name.startswith(artifact + "-") and (best is None or len(artifact) > len(best[1])):
            best = (group, artifact)
    return best


def bundled_in_jar(jar_name: str, files: dict[str, str]) -> list[tuple[str, str, str]]:
    """files 是 {jar 內路徑: 文字內容}（只含 versions.properties 與 pom.properties）。
    回傳 [(group:artifact, version, 來源)]。"""
    poms = []
    for path, text in files.items():
        m = POM_PROPS_RE.match(path)
        if m:
            p = read_props(text)
            poms.append((p.get("groupId", m.group(1)), p.get("artifactId", m.group(2)), p.get("version", "")))
    own = own_coordinates(jar_name, poms)
    out: list[tuple[str, str, str]] = []
    for group, artifact, version in poms:
        if (group, artifact) != own and version:
            out.append((f"{group}:{artifact}", version, f"{jar_name}!pom.properties"))
    own_is_netty = own is not None and own[0] == "io.netty"
    for path, text in files.items():
        if not VERSIONS_RE.match(path) or own_is_netty:
            continue
        for line in text.splitlines():
            m = NETTY_VERSION_RE.match(line.strip())
            if m:
                out.append((f"io.netty:{m.group(1)}", m.group(2), f"{jar_name}!{path}"))
    return out


def scan_jar(path: Path) -> list[tuple[str, str, str]]:
    files: dict[str, str] = {}
    with zipfile.ZipFile(path) as z:
        for name in z.namelist():
            if VERSIONS_RE.match(name) or POM_PROPS_RE.match(name):
                files[name] = z.read(name).decode("utf-8", "replace")
    return bundled_in_jar(path.name, files)


def load_ignores(toml_text: str) -> dict[str, datetime.date]:
    data = tomllib.loads(toml_text)
    return {e["id"]: e["ignoreUntil"] for e in data.get("IgnoredVulns", [])}


def evaluate(findings: dict[tuple[str, str], list[dict]], ignores: dict[str, datetime.date], today: datetime.date):
    """findings: {(座標, 版本): [{id, aliases}]}。回傳 (失敗行, 放行行)。"""
    failed, allowed = [], []
    for (coord, version), vulns in sorted(findings.items()):
        for v in vulns:
            ids = [v["id"], *v.get("aliases", [])]
            hit = next((i for i in ids if i in ignores), None)
            if hit is None:
                failed.append(f"{coord}@{version}: {v['id']} 不在 osv-scanner.toml 的 IgnoredVulns")
            elif ignores[hit] < today:
                failed.append(f"{coord}@{version}: {v['id']}（{hit}）的 ignoreUntil {ignores[hit]} 已過")
            else:
                allowed.append(f"{coord}@{version}: {v['id']} 已放行（{hit}，到 {ignores[hit]}）")
    return failed, allowed


def _post(url: str, body: dict) -> dict:
    req = urllib.request.Request(url, json.dumps(body).encode(), {"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.load(r)


def query_osv(coords: list[tuple[str, str]]) -> dict[tuple[str, str], list[dict]]:
    """querybatch 只回 id；有漏洞時再取一次別名。分頁以 next_page_token 追到底。"""
    result: dict[tuple[str, str], list[dict]] = {}
    for start in range(0, len(coords), 500):
        chunk = coords[start:start + 500]
        queries = [{"package": {"name": n, "ecosystem": "Maven"}, "version": v} for n, v in chunk]
        pending = list(enumerate(queries))
        while pending:
            resp = _post(OSV_BATCH, {"queries": [q for _, q in pending]})
            nxt = []
            for (idx, q), res in zip(pending, resp["results"]):
                key = chunk[idx]
                for vuln in res.get("vulns", []):
                    result.setdefault(key, []).append({"id": vuln["id"]})
                if res.get("next_page_token"):
                    nxt.append((idx, {**q, "page_token": res["next_page_token"]}))
            pending = nxt
    for vulns in result.values():
        for v in vulns:
            with urllib.request.urlopen(OSV_VULN + v["id"], timeout=60) as r:
                v["aliases"] = json.load(r).get("aliases", [])
    return result


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--jars", required=True)
    ap.add_argument("--config", default=str(Path(__file__).resolve().parent.parent / "osv-scanner.toml"))
    ap.add_argument("--today", default=datetime.date.today().isoformat())
    args = ap.parse_args()

    jars = sorted(Path(args.jars).glob("*.jar"))
    if not jars:
        print(f"::error::{args.jars} 沒有 jar：依賴解析可能失敗")
        return 2
    bundled: dict[tuple[str, str], set[str]] = {}
    for jar in jars:
        for coord, version, source in scan_jar(jar):
            bundled.setdefault((coord, version), set()).add(source)
    print(f"掃了 {len(jars)} 個 jar，找到 {len(bundled)} 個內含的（座標，版本）")
    for (coord, version), sources in sorted(bundled.items()):
        print(f"  {coord}@{version}  <- {', '.join(sorted(sources))}")
    findings = query_osv(sorted(bundled))
    failed, allowed = evaluate(
        findings, load_ignores(Path(args.config).read_text()), datetime.date.fromisoformat(args.today)
    )
    for line in allowed:
        print("allow", line)
    for line in failed:
        print("::error::" + line + f"（來源：{', '.join(sorted(bundled[tuple(line.split(': ')[0].rsplit('@', 1))]))}）")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
