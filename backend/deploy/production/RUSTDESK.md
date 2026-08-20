# RustDesk Server Pro production runbook

Production remains blocked until both the Server Pro subscription (at least 20 devices / 3 concurrent sessions) and written permission for proprietary Android client embedding are recorded by the release owner.

1. Create `/opt/claw-rustdesk/data` as a root-owned persistent directory and a separate encrypted backup target.
2. Put a versioned, digest-pinned image in `RUSTDESK_SERVER_PRO_IMAGE`; run `validate-rustdesk-env.sh`, then deploy `rustdesk-compose.yml`.
3. At the host firewall expose only `21115-21117/tcp` and `21116/udp`. Keep `21114/tcp` reachable from localhost/Docker Nginx only, and deny public `21118-21119`.
4. Configure `rd.tmpai2.com` as DNS-only. Configure `rd-admin.tmpai2.com` behind Cloudflare proxy and Cloudflare Access; the Nginx origin also accepts only Cloudflare source ranges.
5. On first login change the Pro administrator password, enable 2FA, and verify the license/device limit. Never put Pro credentials in Compose or Git.
6. Back up `/opt/claw-rustdesk/data` daily, test restore quarterly, and alert when either `21116/tcp` or `21117/tcp` is unavailable.

Emergency rollback: set `V2_REMOTE_ASSISTANCE_ENABLED=false`, restart API/admin, wait for the stop commands already issued to active devices, then stop the RustDesk Compose stack. Do not delete its data directory during rollback.

Before each Nginx rollout compare the allowlist in `nginx.conf` with Cloudflare's current published IPv4/IPv6 ranges. A stale origin allowlist can either interrupt console access or retain an address range Cloudflare no longer owns.

## Android release gate

1. Commit the reviewed proprietary adapter in the licensed fork, keep the pinned upstream commit as its base, and write the resulting commit to `LICENSED_FORK_COMMIT` in `native/rustdesk-host-sdk/rustdesk-upstream.lock`.
2. With the written embedding authorization on file, set `RUSTDESK_COMMERCIAL_EMBEDDING_LICENSE_ACK=accepted`, point `RUSTDESK_SOURCE_DIR` at that clean fork, and run `native/rustdesk-host-sdk/scripts/build-host-sdk.sh` using Rust 1.75.0, cargo-ndk 3.1.2 and NDK r28c.
3. Build the private ARM64 APK. If cloud packaging does not merge the AAR and Manifest, use DCloud Android offline packaging with the same UTS API rather than adding a second Flutter runtime.
4. Run `uniapp/scripts/verify-remote-apk.sh` against the signed APK, then execute the API 29/31/34/35/36 device matrix and the full two-hour/three-session PoC checklist before enabling any non-test account.

During the three-session soak test, stop the release if whole-host CPU stays at or above 70%, memory reaches 80%, or egress reaches 70% of the provider limit. Move `hbbr` to a separate host before general release if any threshold is crossed.
