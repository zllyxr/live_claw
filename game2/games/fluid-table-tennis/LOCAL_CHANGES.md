# Local collection changes

Upstream: <https://github.com/anirudhjoshi/fluid_table_tennis>  
Pinned commit: `2a7e2a8e509a215f48d30d34bc257bbfd765863b`

## Privacy changes

- Removed the legacy Google Analytics loader and page-view tracking.
- Removed the Google+, Twitter, and Facebook share widgets and their remote scripts.
- Replaced the remotely loaded Google Play badge with a normal text link.
- Removed the remotely loaded GitHub ribbon; the existing source-code link remains.
- Preserved the author credit, references, and ordinary external links. Those links make no request until selected.

## Local build

`build/` is regenerated from `src/` with the upstream `utils/quick_build.sh` script. The local favicon is copied into the build so the entry page has no missing local resource.

The core game requires no network connection. This local copy does not actively load third-party scripts, analytics, social widgets, fonts, or images.
