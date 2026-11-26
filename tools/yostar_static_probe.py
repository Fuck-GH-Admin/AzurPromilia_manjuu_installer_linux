"""Read-only Yostar JP installer inspection. Never execute the installer."""
import argparse
import hashlib
import html
import json
import os
from pathlib import Path
import re
import struct
import subprocess
import tempfile
import urllib.request
import urllib.parse

PAGE = "https://azurpromilia.jp/PCdownload"
HOST = "launcher-pkg-azupro-jp.yo-star.com"
EXPECTED = "dde29b4fa4a6eabfb664838c807460e65746f590f1a51754a33d976763bb1e3f"
SIZE = 179893048

def fetch(url, allowed_host, limit):
    class StrictRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            check_url(newurl, allowed_host)
            return super().redirect_request(req, fp, code, msg, headers, newurl)
    check_url(url, allowed_host)
    opener = urllib.request.build_opener(StrictRedirect())
    with opener.open(urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"}), timeout=60) as response:
        check_url(response.geturl(), allowed_host)
        data = response.read(limit + 1)
    if len(data) > limit:
        raise ValueError("response exceeds allowed size")
    return data

def check_url(url, allowed_host):
    u = urllib.parse.urlparse(url)
    if u.scheme != "https" or u.hostname != allowed_host or u.username or u.password or u.port not in (None, 443):
        raise ValueError("unexpected origin")

def acquire(path):
    page = html.unescape(fetch(PAGE, "azurpromilia.jp", 2_000_000).decode("utf-8")).replace(r"\/", "/")
    candidates = sorted(set(re.findall(r'https://launcher-pkg-azupro-jp\.yo-star\.com/[^"\'<>\s\\]+?\.exe(?:\?[^"\'<>\s\\]*)?', page, re.I)))
    if len(candidates) != 1:
        raise ValueError(f"expected one official installer URL, found {len(candidates)}")
    path.write_bytes(fetch(candidates[0], HOST, SIZE + 1))

def digest(path):
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()

def inventory_asar(path):
    with path.open("rb") as f:
        prefix = f.read(16)
        if len(prefix) != 16:
            raise ValueError("short ASAR header")
        a, header_size, b, string_size = struct.unpack("<IIII", prefix)
        if a != 4 or not 0 < string_size <= header_size - 8 or header_size > 64 * 1024 * 1024:
            raise ValueError("invalid ASAR header")
        entries = json.loads(f.read(string_size))
        base = 8 + header_size
        files = []
        markers = set()
        domains = set()
        def walk(node, parent=""):
            for name, item in node.get("files", {}).items():
                rel = parent + name
                if "files" in item:
                    walk(item, rel + "/")
                elif "size" in item:
                    files.append({"path": rel, "bytes": item["size"], "unpacked": bool(item.get("unpacked"))})
                    if item.get("unpacked") or "offset" not in item or not rel.endswith((".js", ".json", ".cjs", ".mjs")):
                        continue
                    size = int(item["size"])
                    offset = int(item["offset"])
                    if size < 0 or size > 32 * 1024 * 1024 or offset < 0 or base + offset + size > path.stat().st_size:
                        continue
                    f.seek(base + offset)
                    data = f.read(size)
                    for token in (b"packageconfig", b"gameversion", b"downloadMode", b"packageType", b"REALITY", b"SLICE", b"launcherId", b"cutils", b"preload", b"manifest"):
                        if token in data:
                            markers.add((rel, token.decode()))
                    for match in re.findall(rb'https?://([A-Za-z0-9.-]+\.[A-Za-z]{2,})(?=[:/\'"\s]|$)', data):
                        domains.add(match.decode().lower())
        walk(entries)
    return {"sha256": digest(path), "file_count": len(files), "files": files,
            "markers": sorted(markers), "domains": sorted(domains)}

def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--installer", type=Path)
    ap.add_argument("--sevenzip", required=True)
    ap.add_argument("--out", type=Path, required=True)
    args = ap.parse_args()
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="yostar-static-") as tmp:
        root = Path(tmp)
        sample = args.installer or root / "official.exe"
        if not args.installer:
            acquire(sample)
        sha = digest(sample)
        size = sample.stat().st_size
        print(f"Installer bytes={size} sha256={sha}", flush=True)
        if size != SIZE or sha != EXPECTED:
            raise ValueError("installer differs from previously reviewed sample; stop for new provenance review")
        with sample.open("rb") as f:
            if f.read(2) != b"MZ":
                raise ValueError("not PE")
        extracted = root / "outer"
        subprocess.run([args.sevenzip, "x", "-y", "-o" + str(extracted), str(sample)], check=True, stdout=subprocess.DEVNULL)
        report = {"source": PAGE, "installer_bytes": size, "installer_sha256": sha, "archives": [], "asars": []}
        # Inventory nested packages, but never launch any executable.
        for p in sorted(extracted.rglob("*")):
            if p.is_file() and p.suffix.lower() in (".exe", ".7z", ".asar", ".node", ".dll"):
                report["archives"].append({"path": p.relative_to(extracted).as_posix(), "bytes": p.stat().st_size, "sha256": digest(p)})
        roots = [extracted]
        for i, p in enumerate(sorted(extracted.rglob("*"))):
            if not p.is_file() or p.name.lower() not in ("launcher-installer.exe", "app-32.7z", "app-64.7z"):
                continue
            dest = root / f"nested-{i}"
            proc = subprocess.run([args.sevenzip, "x", "-y", "-o" + str(dest), str(p)], stdout=subprocess.DEVNULL)
            report["archives"].append({"nested": p.relative_to(extracted).as_posix(), "extract_exit": proc.returncode})
            if proc.returncode == 0:
                roots.append(dest)
        for folder in roots:
            for p in sorted(folder.rglob("*.asar")):
                try:
                    report["asars"].append({"path": folder.name + "/" + p.relative_to(folder).as_posix(), **inventory_asar(p)})
                except (ValueError, OSError, KeyError) as exc:
                    report["asars"].append({"path": p.name, "error": str(exc)})
        args.out.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
        print("Archives:", len(report["archives"]), "ASARs:", len(report["asars"]), flush=True)
        for asar in report["asars"]:
            print(asar.get("path"), "files:", asar.get("file_count"), "markers:", asar.get("markers", [])[:30])
        if not any("file_count" in x for x in report["asars"]):
            raise SystemExit("No readable ASAR: inspect archive inventory")

if __name__ == "__main__":
    main()
