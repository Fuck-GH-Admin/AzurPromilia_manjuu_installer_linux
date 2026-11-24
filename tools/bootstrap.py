#!/usr/bin/env python3
"""One-time metadata bootstrap using the vendor runtime from your own installer."""
import argparse
import io
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import zipfile

p = argparse.ArgumentParser()
p.add_argument("--installer", required=True, type=Path)
p.add_argument("--out", default=Path("packageconfig.json"), type=Path)
p.add_argument("--wine", default="wine")
a = p.parse_args()
source = a.installer.resolve()
out = a.out.resolve()
if out.exists():
    p.error("output already exists; move it aside before refreshing")
data = source.read_bytes()
end = data.rfind(b"PK\x05\x06")
while end >= 0:
    try:
        archive = zipfile.ZipFile(io.BytesIO(data[:end + 22]))
        if "resources/app.asar" in archive.namelist():
            break
        archive.close()
    except zipfile.BadZipFile:
        pass
    end = data.rfind(b"PK\x05\x06", 0, end)
else:
    p.error("embedded launcher ZIP not found")
del data
with tempfile.TemporaryDirectory(prefix="azurpromilia-bootstrap-") as tmp:
    root = Path(tmp)
    with archive:
        for entry in archive.infolist():
            target = (root / entry.filename).resolve()
            if root not in target.parents:
                p.error("unsafe archive member")
            if entry.filename.startswith(("res/", "resources/")) or "/" not in entry.filename:
                archive.extract(entry, root)
    shutil.copyfile(Path(__file__).with_name("fetch_packageconfig.cjs"), root / "fetch_packageconfig.cjs")
    env = dict(os.environ, ELECTRON_RUN_AS_NODE="1")
    cmd = [str(root / "launcher.exe"), "fetch_packageconfig.cjs"]
    if os.name != "nt":
        cmd = [a.wine, "./launcher.exe", "fetch_packageconfig.cjs"]
    print("Requesting anonymous package metadata with the original vendor runtime; no game download.")
    result = subprocess.run(cmd, cwd=root, env=env, timeout=120)
    response = root / "packageconfig.json"
    if result.returncode or not response.is_file():
        raise SystemExit("Bootstrap failed: no successful official response. Check Wine/runtime and server status.")
    with out.open("xb") as f:
        f.write(response.read_bytes())
    try:
        out.chmod(0o600)
    except OSError:
        pass
    print("Saved official package response:", out)
