# Local subpath build changes

This directory is based on LibreLudo `v2.1.0`, commit
`2ef28e27cdbcfd301f2638a3414a93713558fd69`.

The following small changes make the bundled game work when served from an
arbitrary directory such as `/game2/games/libreludo/dist/`, rather than only
from the root of a domain:

- `vite.config.ts` uses the relative Vite base `./`.
- `src/router.tsx` uses a hash router, so static hosting does not require URL
  rewrite rules and route links remain inside this directory.
- `src/pages/ErrorBoundary/ErrorBoundary.tsx` returns to setup through the URL
  hash.
- `src/pages/HomePage/HomePage.tsx` uses relative links for the bundled license
  files.
- `pwa.config.ts` uses relative start, icon, and navigation-fallback URLs.

## Rebuild

Requirements: Node.js and pnpm 11 or newer.

```sh
pnpm install --frozen-lockfile
pnpm build
```

The reproducible static entry point is `dist/index.html`. Serve the parent
directory over HTTP; opening the file directly with `file://` is not supported
by the service worker.

LibreLudo and these corresponding source changes remain licensed under
`AGPL-3.0-only`; see `LICENSE`. Generated third-party notices are written to
`dist/THIRD_PARTY_LICENSES.txt` during the build.
