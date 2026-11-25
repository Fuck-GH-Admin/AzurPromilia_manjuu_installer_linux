#!/usr/bin/env python3
import argparse
import hashlib
import pathlib
import re
import struct


def find_all(blob: bytes, needle: bytes, limit: int = 20):
    out = []
    start = 0
    while len(out) < limit:
        idx = blob.find(needle, start)
        if idx < 0:
            break
        out.append(idx)
        start = idx + 1
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("installer")
    ap.add_argument("--out", default="yostar-report.txt")
    args = ap.parse_args()

    p = pathlib.Path(args.installer)
    b = p.read_bytes()
    lines = []
    add = lines.append

    add("== FILE ==")
    add(f"name={p.name}")
    add(f"size={len(b)}")
    add(f"sha256={hashlib.sha256(b).hexdigest()}")

    add("")
    add("== PE STRUCTURE ==")
    if b[:2] != b"MZ" or len(b) < 0x40:
        add("valid_mz=false")
    else:
        pe = struct.unpack_from("<I", b, 0x3C)[0]
        add(f"pe_offset={pe}")
        if pe + 24 <= len(b) and b[pe:pe+4] == b"PE\0\0":
            machine, nsec, _, _, _, optsz, chars = struct.unpack_from("<HHIIIHH", b, pe + 4)
            add(f"machine=0x{machine:04x}")
            add(f"sections={nsec}")
            add(f"characteristics=0x{chars:04x}")
            sec_off = pe + 24 + optsz
            overlay = 0
            for i in range(nsec):
                off = sec_off + i * 40
                if off + 40 > len(b):
                    add(f"section[{i}]=TRUNCATED")
                    break
                name = b[off:off+8].rstrip(b"\0").decode("ascii", "replace")
                vsize, vaddr, rawsize, rawptr = struct.unpack_from("<IIII", b, off + 8)
                overlay = max(overlay, rawptr + rawsize)
                add(f"section[{i}]={name} raw_offset={rawptr} raw_size={rawsize} virtual_size={vsize}")
            add(f"overlay_offset={overlay}")
            add(f"overlay_size={max(0, len(b) - overlay)}")
            if 0 <= overlay < len(b):
                add(f"overlay_head_hex={b[overlay:overlay+64].hex()}")
        else:
            add("valid_pe_signature=false")

    add("")
    add("== SIGNATURE OFFSETS ==")
    signatures = {
        "zip_local": b"PK\x03\x04",
        "zip_eocd": b"PK\x05\x06",
        "7z": b"7z\xbc\xaf\x27\x1c",
        "rar4": b"Rar!\x1a\x07\x00",
        "rar5": b"Rar!\x1a\x07\x01\x00",
        "cab": b"MSCF",
        "nsis": b"Nullsoft",
        "inno": b"Inno Setup",
        "electron": b"Electron",
        "asar": b"app.asar",
    }
    for name, sig in signatures.items():
        add(f"{name}={find_all(b, sig)}")

    add("")
    add("== MARKERS ==")
    markers = [
        "app.asar", r"resources\app.asar", "electron", "Electron",
        "napi-cutils", "cutils.node",
        "api-grp.manjuu.com", "api-grp-l2.manjuu.com",
        "packageconfig", "gameversion", "downloadMode",
        "gamePackageList", "cdnBaseUrls", "launchEntry", "pre/check",
        "manjuu", "Manjuu", "Yostar", "YOSTAR", "yo-star",
        "AzurPromilia", "launcherId", "channelCode", "secretKey", "SC000",
    ]
    for marker in markers:
        a = b.find(marker.encode("utf-8"))
        u = b.find(marker.encode("utf-16le"))
        add(f"{marker}: ascii={a} utf16le={u}")

    add("")
    add("== URLS / HOSTS SAMPLE ==")
    urls = []
    seen = set()
    for m in re.finditer(rb"https?://[A-Za-z0-9._~:/?#@!$&()*+,;=%-]{8,300}", b):
        url = m.group().decode("ascii", "ignore").rstrip(".,;)>]")
        if url not in seen:
            seen.add(url)
            urls.append(url)
        if len(urls) >= 120:
            break
    for url in sorted(urls):
        add(url)

    pathlib.Path(args.out).write_text("\n".join(lines) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
