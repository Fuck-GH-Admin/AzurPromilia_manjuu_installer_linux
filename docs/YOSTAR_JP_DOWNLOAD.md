# Yostar JP download implementation — 2026-11-27

## Verified source and static evidence

The exact official sample downloaded by Actions run 45868787331 was copied into the research container from artifact 15140919290. Local SHA-256 independently matched `dde29b4fa4a6eabfb664838c807460e65746f590f1a51754a33d976763bb1e3f`, size 179893048 bytes. The PE32 NSIS outer archive uses non-solid raw deflate blocks; the app-32.7z payload (SHA-256 8435b0019c80a46436952a22a6f0425218681704d07e507aaade5dbba60bf219) was independently extracted via libarchive. Inner resources/app.asar SHA-256: 4f013026736f6afa819a37f299caa9577140e88254ef4097b4d4589fa47f0818. Inner package.json identifies AZUPRO_JP_Gamelauncher 1.8.1. No installer execution was necessary.

Evidence pointers within the inner ASAR:
- out/main/index.js:863–944: anonymous HTTP client metadata header, manifest URL resolver, CDN resolver.
- node_modules/@launcher/utils/dist/index.cjs:2861–2882: local object hash and anonymous metadata signature.
- out/renderer/assets/main-4217f8b0.js:26–29 and 8080–8113: version, basis, executable name from game config.
- out/main/index.js:3917 onward: reflected CRC64 polynomial 0xc96c5795d7870f42; little-endian CRC complements the state. 19440–19531: decimal unsigned 64-bit hash strings. Standard CRC64/XZ check vector 123456789 = 11051210869376104954.
- out/main/index.js:19593–19648: manifest source and file array; comparison and repair.
- out/main/index-8b10f6cc.js:18675–18770: source + relative file path on selected CDN origin; HTTP Range and CRC64 verification.
- out/main/index-8b10f6cc.js:18902–18917: original local launcher manifest/config metadata.

## CLI draft

Build with `go build -o azurpromilia ./cmd/azurpromilia`. The Japanese channel is explicitly separate from SC000:

    ./azurpromilia yostar info
    ./azurpromilia yostar probe --root /tmp/azupro-jp-probe
    ./azurpromilia yostar install --root "$HOME/Games/AzurPromilia-JP" --workers 4
    ./azurpromilia yostar verify --root "$HOME/Games/AzurPromilia-JP"
    ./azurpromilia yostar repair --root "$HOME/Games/AzurPromilia-JP" --workers 4

Go fetches the released config, primary/backup CDN, and JSON manifest using the anonymous desktop-client metadata protocol. It converts decimal uint64 hashes to explicit crc64: checksums in the shared Entry representation. The legacy MD5 field is retained for SC000 compatibility; SC000 index parsing remains strictly MD5. The existing parallel download, partial resume, size limit, disk space preflight, cancellation and atomic file rename paths are reused. JP metadata is stored separately under .azurpromilia-yostar; SC000 state is refused at the JP root.

Install/repair converge to the latest manifest and do not remove unknown or obsolete files. Original Windows launcher metadata files are not yet emitted. No account access, proprietary executable runtime, GUI, Proton configuration, or auto-start. No claim of cross-version update compatibility or full install until live-tested.

## Verification

Local original-source analysis and unit test vectors are not official server evidence. The live Actions workflow must actually discover the released configuration, parse the manifest, download a nonempty official resource, and verify its CRC64 before marking the public protocol accepted. Full installation remains a separate acceptance test. Keep the PR as draft until verified; do not merge into main solely on static analysis.
