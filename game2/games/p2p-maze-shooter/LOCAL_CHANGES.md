# Local collection changes

Upstream: <https://github.com/arifulislamat/p2p-maze-shooter>  
Pinned commit: `29d9adbbd1ddb56e3dcf796cf306d21b232c6808`

## Privacy changes

- Removed Google Analytics / Google Tag Manager (`gtag`) loading and tracking.
- Removed remote Google Fonts and their preconnect hints. Existing CSS system-font fallbacks are used instead.
- Preserved author attribution, project metadata, and ordinary external links.

## Necessary network access

Online matches still require these upstream runtime connections:

- jsDelivr loads PeerJS 1.5.5 from `cdn.jsdelivr.net`.
- PeerJS uses its default cloud signaling service to exchange connection metadata.
- WebRTC uses the configured Google STUN endpoints to discover peer routes.
- Optional voice chat uses the peer-to-peer WebRTC connection and microphone permission.

The game has no local build dependencies. Serve `src/` over HTTP; HTTPS or `localhost` is recommended when voice chat is used.
