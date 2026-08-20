# Licensed RustDesk 1.4.9 adapter contract

The checked-in Host SDK deliberately contains no AGPL RustDesk client source or binary. The separately licensed fork must remain based on the locked RustDesk 1.4.9 upstream commit, be pinned to its own reviewed `LICENSED_FORK_COMMIT`, export one class named `com.claw.remote.generated.RustDesk149Adapter` implementing `com.claw.remote.CoreAdapter`, and package `lib/arm64-v8a/librustdesk.so`.

The adapter owns the RustDesk implementation of capture frame delivery, input/accessibility, clipboard, files, system audio, chat and voice. It must not depend on `FlutterActivity`, `MainApplication`, a Flutter engine, or a second application runtime. Calls that upstream sends through `MainActivity.flutterMethodChannel` must instead emit typed Host SDK events. The upstream boot receiver and `EXT_INIT_FROM_BOOT` projection path must be deleted.

Required behavior:

- `initialize`: configure ID, relay, API and public key, retain the supplied `HostEventSink`, and start the Rust server without starting MediaProjection.
- `attachAccessibilityService` / `detachAccessibilityService`: retain only the currently connected `RemoteInputService` instance and route authorized gesture/text input through it; never synthesize input while the service is disconnected.
- `start`: accept only the fresh `MediaProjection` supplied after `ProjectionPermissionActivity` succeeds.
- `setTemporaryPassword`: set the 20-character password until the supplied deadline without logging it.
- `rotateParkingPassword`: generate the replacement inside Android/Rust code, never return or upload it, and run at service startup, first successful connection, credential timeout and service stop.
- `rustDeskId`: return only the server-assigned ID.
- `stop`: terminate capture, sessions, clipboard listeners, audio, file jobs and Rust server callbacks.

The fork must report request/authorization/connect/disconnect/file/chat/voice events through the Host SDK event sink without filenames, clipboard data, audio data, passwords or controller-provided free text. The checked-in reporter applies a second metadata allowlist and rotates the parking password synchronously when it receives `connected`.
