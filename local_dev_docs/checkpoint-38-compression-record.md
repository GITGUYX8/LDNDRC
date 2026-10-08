# Checkpoint 38 — Image Compression Record (detailed)

## Context

Consolidates the compression facts scattered across CP-30/33 into
one record: method, measured pairs, transfer math, and the options
verdict. No code or image changes this chunk.

## Method

Standard OCI/Docker gzip layer compression — nothing custom. All
transfer figures below are `docker save` tarballs of gzipped layers;
all unpacked figures are `docker images` sizes (uncompressed).

## Measured pairs (observed)

| Tag | Unpacked | Tarball | Ratio |
|---|---|---|---|
| `jazzy-harmonic-base` | 8.25GB | — (never shipped) | — |
| `jazzy-harmonic-eaaf8088` (pre-selkes workspace) | 10.2GB | 2.3GB | ~4.4x |
| `jazzy-harmonic-4aa29d15` (+ Selkies 2.0.0) | 10.8GB | 2.4GB | ~4.5x |
| `jazzy-harmonic-10f1f10c` (+ Fuel seed) | 10.8GB | 2.4GB | ~4.5x |

Import unpack times on the master (k3s containerd): 164s (first,
cold), 65s, 28s (warm, layer sharing). Tarball lives in `$HOME`,
not `/tmp` (7.7GB tmpfs too small and wiped on reboot).

## Transfer math per link (2.4GB file)

- 1Gbps wired: ~25s.
- Real-world Wi-Fi (100–300Mbps): ~1–4 min.
- Unpack on host: ~1–3 min, 1 CPU spiked.
- Per-host one-time total on Wi-Fi: ~5–10 min, then cached forever
  (`IfNotPresent`); 10.8GB pinned disk per host.

## Options verdict (analyzed, not implemented)

- **Join-time tarball import** (current plan): proven, offline-safe.
- **Registry mirror on master**: same bytes, less labor; worth it
  at ~5+ hosts.
- **Slimming**: measured candidates — system node stack ~330MB,
  Rust ~250–300MB, valgrind 75MB, clang/LLVM dupes ~200–400MB,
  `/usr/share/doc` 174MB, OpenJDK ~286MB (nothing outside Java
  requires it, needs smoke). Conservative yield ~1–1.5GB unpacked
  (~400–600MB transfer). VTK/PCL/RViz/Qt/Mesa stay (the product).
- **zstd**: untested; expect ~20–30% faster unpack, marginal size
  change. Needs containerd acceptance check first.
- **Lazy pulling (stargz)**: skipped — best latency on paper, worst
  complexity payoff at this scale.
- **Multi-stage builds**: wrong tool here — ~95% of the image is
  runtime payload, not compiled artifacts; savings come from
  deletion, not staging.
- **Light/desktop-light image**: parked by operator decision —
  stand-in covers flow checks (already imported), light only when a
  new host needs desktop proof without the 10GB pull.

## Dirty-tree discipline

Only this report (staged for commit). The 8 join/nodes files +
`join-cluster.sh` untouched/unstaged; untracked junk left alone.
