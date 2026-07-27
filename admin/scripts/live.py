#!/usr/bin/env python3
import argparse
import contextlib
import http.cookiejar
import json
import os
import re
import signal
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

try:
    import fcntl
except ImportError:
    fcntl = None

from live_douyin import DouyinResolver

DEFAULT_PAGE = 'https://live.douyin.com/'
DEFAULT_UA = (
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) '
    'AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126 Safari/537.36'
)
PROXY_CONFIG_CANDIDATES = (
    '/var/www/html/admin/data/live_proxy.conf',
    '/opt/claw/admin/data/live_proxy.conf',
)
STOP_REQUESTED = False
CURRENT_PROCESS = None
_PROXY_LOADED = False
_PROXY_URL = ''
DOUYIN_FETCH_LOCK = os.environ.get(
    'DOUYIN_FETCH_LOCK',
    '/tmp/claw-douyin-fetch/lock',
)
DOUYIN_FETCH_STATE = os.environ.get(
    'DOUYIN_FETCH_STATE',
    '/tmp/claw-douyin-fetch/state.json',
)


def stop_process(process):
    if process is None or process.poll() is not None:
        return
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except (OSError, ProcessLookupError):
        process.terminate()
    try:
        process.wait(timeout=8)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except (OSError, ProcessLookupError):
            process.kill()
        process.wait(timeout=5)


def request_stop(signum, frame):
    del signum, frame
    global STOP_REQUESTED
    STOP_REQUESTED = True
    stop_process(CURRENT_PROCESS)


def install_signal_handlers():
    signal.signal(signal.SIGTERM, request_stop)
    signal.signal(signal.SIGINT, request_stop)


def sleep_with_stop(seconds):
    deadline = time.time() + max(0, seconds)
    while not STOP_REQUESTED and time.time() < deadline:
        time.sleep(min(1, deadline - time.time()))


def read_proxy_config_file(path):
    try:
        with open(path, 'r', encoding='utf-8') as handle:
            for raw_line in handle:
                line = raw_line.strip()
                if not line or line.startswith('#'):
                    continue
                if '=' in line:
                    key, value = line.split('=', 1)
                    if key.strip() not in {'PROXY_URL', 'PAGE_PROXY_URL', 'LIVE_PROXY_URL'}:
                        continue
                    line = value.strip()
                return line.strip().strip('"').strip("'")
    except OSError:
        return ''
    return ''


def normalize_proxy_url(value):
    value = (value or '').strip()
    if not value:
        return ''
    if '://' not in value:
        value = 'http://' + value
    return value


def load_proxy_url():
    for name in ('PAGE_PROXY_URL', 'LIVE_PROXY_URL', 'PROXY_URL'):
        value = normalize_proxy_url(os.environ.get(name, ''))
        if value:
            return value

    config_path = os.environ.get('LIVE_PROXY_CONFIG', '')
    candidates = [config_path] if config_path else []
    candidates.extend(PROXY_CONFIG_CANDIDATES)
    for path in candidates:
        value = normalize_proxy_url(read_proxy_config_file(path))
        if value:
            return value
    return ''


def get_proxy_url():
    global _PROXY_LOADED, _PROXY_URL
    if not _PROXY_LOADED:
        _PROXY_URL = load_proxy_url()
        _PROXY_LOADED = True
    return _PROXY_URL


def proxy_label(proxy_url):
    if not proxy_url:
        return 'disabled'
    parts = urllib.parse.urlsplit(proxy_url)
    host = parts.hostname or ''
    port = f':{parts.port}' if parts.port else ''
    return f'enabled {host}{port}' if host else 'enabled'


def red_text(value):
    sensitive_names = (
        'token',
        'session',
        'sign',
        't_id',
        'volcSecret',
        'volcTime',
        'wsSecret',
        'wsTime',
        'auth_key',
    )
    names = '|'.join(re.escape(name) for name in sensitive_names)
    value = re.sub(
        rf'((?:{names})=)[^&\s\]\)\'"]+',
        r'\1<redacted>',
        value,
        flags=re.I,
    )
    value = re.sub(r'(https?://)[^:/\s]+:[^@\s]+@', r'\1<redacted>:<redacted>@', value)
    value = re.sub(r'(Cookie:).*', r'\1 <redacted>', value)
    return value


def _read_douyin_fetch_state():
    try:
        with open(DOUYIN_FETCH_STATE, 'r', encoding='utf-8') as handle:
            state = json.load(handle)
            return {
                'last_request': float(state.get('last_request', 0) or 0),
                'cooldown_until': float(state.get('cooldown_until', 0) or 0),
            }
    except (OSError, ValueError, TypeError):
        return {'last_request': 0.0, 'cooldown_until': 0.0}


def _write_douyin_fetch_state(state):
    temp_path = DOUYIN_FETCH_STATE + '.tmp'
    try:
        with open(temp_path, 'w', encoding='utf-8') as handle:
            json.dump(state, handle)
        os.replace(temp_path, DOUYIN_FETCH_STATE)
    except OSError:
        pass


def _prepare_douyin_fetch_dir():
    paths = (DOUYIN_FETCH_LOCK, DOUYIN_FETCH_STATE)
    for path in paths:
        directory = os.path.dirname(path)
        if not directory:
            continue
        try:
            os.makedirs(directory, mode=0o777, exist_ok=True)
            os.chmod(directory, 0o777)
        except OSError:
            pass


@contextlib.contextmanager
def douyin_fetch_slot(url):
    host = (urllib.parse.urlsplit(url).hostname or '').lower()
    if host != 'live.douyin.com' or fcntl is None:
        yield None
        return

    interval = max(0, env_int('DOUYIN_FETCH_INTERVAL_MS', 1200)) / 1000
    _prepare_douyin_fetch_dir()
    try:
        lock_handle = open(DOUYIN_FETCH_LOCK, 'a+', encoding='utf-8')
    except PermissionError:
        lock_handle = open(DOUYIN_FETCH_LOCK, 'r', encoding='utf-8')
    fcntl.flock(lock_handle.fileno(), fcntl.LOCK_EX)
    state = _read_douyin_fetch_state()
    wait_until = max(
        state.get('cooldown_until', 0),
        state.get('last_request', 0) + interval,
    )
    delay = wait_until - time.time()
    if delay > 0:
        time.sleep(delay)
    try:
        yield state
    finally:
        state['last_request'] = time.time()
        _write_douyin_fetch_state(state)
        fcntl.flock(lock_handle.fileno(), fcntl.LOCK_UN)
        lock_handle.close()


def fetch(url, headers=None, maxread=500000, cookie_jar=None):
    proxy_url = get_proxy_url()
    if proxy_url:
        handlers = [urllib.request.ProxyHandler({'http': proxy_url, 'https': proxy_url})]
    else:
        handlers = [urllib.request.ProxyHandler({})]
    if cookie_jar:
        handlers.append(urllib.request.HTTPCookieProcessor(cookie_jar))
    opener = (
        urllib.request.build_opener(*handlers)
    )
    req = urllib.request.Request(url, headers=headers or {})
    with douyin_fetch_slot(url) as state:
        try:
            with opener.open(req, timeout=25) as response:
                return response.read(maxread).decode('utf-8', 'replace')
        except urllib.error.HTTPError as exc:
            if exc.code == 444 and state is not None:
                cooldown = max(5, env_int('DOUYIN_444_COOLDOWN', 45))
                state['cooldown_until'] = time.time() + cooldown
            raise


def resolve_douyin_stream(args, max_height):
    cookie_jar = http.cookiejar.CookieJar()

    def douyin_fetch(url, headers=None):
        return fetch(
            url,
            headers=headers,
            maxread=8000000,
            cookie_jar=cookie_jar,
        )

    candidate = DouyinResolver(douyin_fetch, args.user_agent).resolve(
        args.page,
        max_height=max_height,
        pull_format=args.pull_format,
    )
    dossier = {
        '_provider': candidate.provider,
        '_room_list_page': candidate.source_page if candidate.source_page != candidate.selected_room_page else '',
        '_selected_room_page': candidate.selected_room_page,
        '_selected_username': candidate.nickname,
        '_selected_room_id': candidate.room_id,
        '_stream_format': candidate.format,
        '_resolution': candidate.resolution,
        '_height': candidate.height,
        '_unified_av': True,
        '_gender_filter': candidate.gender_filter,
        '_gender_verified': candidate.gender_verified,
        '_gender_source': candidate.gender_source,
        '_gender_confidence': candidate.gender_confidence,
    }
    selected = {
        'RESOLUTION': candidate.resolution,
        'BANDWIDTH': '',
        'FORMAT': candidate.format,
        'AUDIO': 'integrated',
    }
    return (
        cookie_jar,
        dossier,
        selected,
        candidate.stream_url,
        '',
        candidate.selected_room_page,
    )


def resolve_stream(args, max_height, max_bandwidth):
    del max_bandwidth
    return resolve_douyin_stream(args, max_height)


def cookie_header(cookie_jar):
    pairs = []
    for cookie in cookie_jar:
        pairs.append(f'{cookie.name}={cookie.value}')
    return '; '.join(pairs)


def env_int(name, default):
    value = os.environ.get(name)
    if value is None or value == '':
        return default
    return int(value)


def build_ffmpeg_cmd(
    child_hls,
    audio_hls,
    audio_mode,
    page,
    ua,
    cookies,
    push_url,
    duration,
    live_start_index=None,
    rw_timeout=15000000,
    hls_max_reload=100,
    hls_hold_counters=1000,
    unified_av=False,
    input_format='hls',
):
    parsed_page = urllib.parse.urlsplit(page)
    origin = f'{parsed_page.scheme}://{parsed_page.netloc}' if parsed_page.scheme and parsed_page.netloc else ''
    headers = f'Referer: {page}\r\n'
    if origin:
        headers += f'Origin: {origin}\r\n'
    headers += 'Accept: */*\r\n'
    if cookies:
        headers += f'Cookie: {cookies}\r\n'

    def input_options():
        opts = [
            '-reconnect',
            '1',
            '-reconnect_streamed',
            '1',
            '-reconnect_delay_max',
            '5',
            '-rw_timeout',
            str(rw_timeout),
            '-http_persistent',
            '0',
            '-http_multiple',
            '0',
        ]
        if input_format == 'hls':
            opts += [
                '-max_reload',
                str(hls_max_reload),
                '-m3u8_hold_counters',
                str(hls_hold_counters),
            ]
        if live_start_index is not None:
            opts += ['-live_start_index', str(live_start_index)]
        opts += [
            '-user_agent',
            ua,
            '-headers',
            headers,
        ]
        return opts

    cmd = [
        'ffmpeg',
        '-hide_banner',
        '-loglevel',
        'warning' if push_url else 'info',
        '-nostdin',
    ]
    source_audio = audio_mode == 'source' and bool(audio_hls)
    silent_audio = audio_mode == 'silent'

    cmd += input_options() + [
        '-i',
        child_hls,
    ]
    if source_audio:
        cmd += input_options() + [
            '-i',
            audio_hls,
        ]
    elif silent_audio:
        cmd += [
            '-f',
            'lavfi',
            '-i',
            'anullsrc=channel_layout=stereo:sample_rate=44100',
        ]
    if duration > 0:
        cmd += ['-t', str(duration)]
    if unified_av:
        cmd += ['-map', '0:v:0', '-map', '0:a:0?', '-c', 'copy']
    elif source_audio:
        cmd += ['-map', '0:v:0', '-map', '1:a:0']
        cmd += ['-c', 'copy']
    elif silent_audio:
        cmd += [
            '-map', '0:v:0',
            '-map', '1:a:0',
            '-c:v', 'copy',
            '-c:a', 'aac',
            '-ar', '44100',
            '-b:a', '64k',
        ]
    else:
        cmd += ['-map', '0:v:0', '-c:v', 'copy', '-an']
    cmd += ['-f', 'flv' if push_url else 'null', push_url or '-']
    return cmd


def print_stream_info(
    args,
    dossier,
    selected,
    max_height,
    max_bandwidth,
    live_start_index,
    rw_timeout,
    hls_max_reload,
    hls_hold_counters,
    audio_mode,
    attempt=None,
):
    if attempt is not None:
        print('attempt:', attempt)
    print('provider:', dossier.get('_provider', 'douyin'))
    print('page:', args.page)
    if dossier.get('_room_list_page'):
        print('room_list_page:', dossier.get('_room_list_page'))
        print('room_list_candidates:', dossier.get('_room_list_candidates', ''))
    if dossier.get('_selected_room_page'):
        print('selected_room_page:', dossier.get('_selected_room_page', ''))
        print('selected_username:', dossier.get('_selected_username', ''))
    if dossier.get('_selected_room_id'):
        print('room_id:', dossier.get('_selected_room_id', ''))
        print('nickname:', dossier.get('_selected_username', ''))
    if dossier.get('_stream_format'):
        print(
            'selected_stream:',
            f"{dossier.get('_stream_format', '')} "
            f"{dossier.get('_resolution', '') or dossier.get('_height', '')}",
        )
    if dossier.get('_gender_filter'):
        print('gender_filter:', dossier.get('_gender_filter', ''))
        print('gender_verified:', 'yes' if dossier.get('_gender_verified') else 'no')
        print('gender_source:', dossier.get('_gender_source', ''))
        print('gender_confidence:', dossier.get('_gender_confidence', 0))
    print('room_status:', dossier.get('room_status', ''))
    print('room_uid:', dossier.get('room_uid', ''))
    print(
        'selected_variant:',
        'res={res} fps={fps} bandwidth={bandwidth}'.format(
            res=selected.get('RESOLUTION', ''),
            fps=selected.get('FRAME-RATE', ''),
            bandwidth=selected.get('BANDWIDTH', ''),
        ),
    )
    print('variant_filter:', f'max_height={max_height} max_bandwidth={max_bandwidth}')
    print('audio_variant:', 'yes' if selected.get('AUDIO') else 'no')
    print('audio_mode:', audio_mode)
    print('live_start_index:', '' if live_start_index is None else live_start_index)
    print(
        'ffmpeg_limits:',
        f'rw_timeout={rw_timeout} max_reload={hls_max_reload} hold_counters={hls_hold_counters}',
    )
    print('proxy:', proxy_label(get_proxy_url()))
    print('mode:', 'push' if args.push_url else 'test')


def run_ffmpeg_stream(cmd):
    global CURRENT_PROCESS
    env = None
    proxy_url = get_proxy_url()
    if proxy_url:
        env = os.environ.copy()
        env['http_proxy'] = proxy_url
        env['https_proxy'] = proxy_url
        env['HTTP_PROXY'] = proxy_url
        env['HTTPS_PROXY'] = proxy_url
    process = subprocess.Popen(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        env=env,
        text=True,
        bufsize=1,
        start_new_session=True,
    )
    CURRENT_PROCESS = process
    try:
        while process.poll() is None:
            line = process.stdout.readline()
            if line:
                print(red_text(line), end='')
            elif STOP_REQUESTED:
                stop_process(process)
                break
            else:
                time.sleep(0.2)

        for line in process.stdout:
            print(red_text(line), end='')
        return process.wait()
    finally:
        CURRENT_PROCESS = None


def run_push_loop(
    args,
    duration,
    max_height,
    max_bandwidth,
    live_start_index,
    rw_timeout,
    hls_max_reload,
    hls_hold_counters,
    audio_mode,
):
    max_retries = env_int('MAX_RETRIES', 0)
    retry_delay = env_int('RETRY_DELAY', 5)
    max_retry_delay = env_int('MAX_RETRY_DELAY', 60)
    attempt = 0
    last_code = 1

    while not STOP_REQUESTED:
        attempt += 1
        start = time.time()
        try:
            cookie_jar, dossier, selected, child_hls, audio_hls, room_page = resolve_stream(
                args,
                max_height,
                max_bandwidth,
            )
            print_stream_info(
                args,
                dossier,
                selected,
                max_height,
                max_bandwidth,
                live_start_index,
                rw_timeout,
                hls_max_reload,
                hls_hold_counters,
                audio_mode,
                attempt=attempt,
            )
            cmd = build_ffmpeg_cmd(
                child_hls,
                audio_hls,
                audio_mode,
                room_page,
                args.user_agent,
                cookie_header(cookie_jar),
                args.push_url,
                duration,
                live_start_index,
                rw_timeout,
                hls_max_reload,
                hls_hold_counters,
                unified_av=bool(dossier.get('_unified_av')),
                input_format=dossier.get('_stream_format', 'hls'),
            )
            last_code = run_ffmpeg_stream(cmd)
            print('ffmpeg_exit:', last_code)
            elapsed = time.time() - start
            print('elapsed_seconds:', round(elapsed, 1))
            if elapsed >= 30:
                attempt = 0
        except Exception as exc:
            last_code = 1
            print('resolve_error:', red_text(f'{type(exc).__name__}: {exc}'))
            print('elapsed_seconds:', round(time.time() - start, 1))

        if STOP_REQUESTED:
            return 143
        if duration > 0 and last_code == 0:
            return 0
        if max_retries > 0 and attempt >= max_retries:
            return last_code

        delay = min(max_retry_delay, retry_delay * min(attempt, 6))
        print('restart_after_seconds:', delay)
        sleep_with_stop(delay)

    return 143


def parse_args():
    parser = argparse.ArgumentParser(description='Pull an authorized Douyin PAGE stream and optionally restream it to RTMP.')
    parser.add_argument('--page', default=os.environ.get('PAGE') or os.environ.get('LIVE_PAGE') or DEFAULT_PAGE)
    parser.add_argument('--push-url', default=os.environ.get('PUSH_URL') or os.environ.get('LIVE_PUSH_URL') or '')
    parser.add_argument('--pull-format', choices=['hls', 'flv'], default=os.environ.get('PULL_FORMAT') or 'hls')
    parser.add_argument('--resolve-only', action='store_true')
    parser.add_argument('--duration', type=int, default=None)
    parser.add_argument('--max-height', type=int, default=None)
    parser.add_argument('--max-bandwidth', type=int, default=None)
    parser.add_argument('--live-start-index', type=int, default=None)
    parser.add_argument('--rw-timeout', type=int, default=None)
    parser.add_argument('--hls-max-reload', type=int, default=None)
    parser.add_argument('--hls-hold-counters', type=int, default=None)
    parser.add_argument('--audio-mode', choices=['source', 'silent', 'none'], default=os.environ.get('AUDIO_MODE') or 'source')
    parser.add_argument('--user-agent', default=os.environ.get('USER_AGENT') or DEFAULT_UA)
    return parser.parse_args()


def main():
    try:
        sys.stdout.reconfigure(line_buffering=True)
    except Exception:
        pass
    install_signal_handlers()

    args = parse_args()
    env_duration = os.environ.get('DURATION')
    if args.duration is not None:
        duration = args.duration
    elif env_duration is not None:
        duration = int(env_duration)
    else:
        duration = 0 if args.push_url else 60
    if args.max_height is not None:
        max_height = args.max_height
    elif os.environ.get('MAX_HEIGHT') is not None:
        max_height = int(os.environ.get('MAX_HEIGHT') or 0)
    else:
        max_height = 720 if args.push_url else 0
    if args.max_bandwidth is not None:
        max_bandwidth = args.max_bandwidth
    elif os.environ.get('MAX_BANDWIDTH') is not None:
        max_bandwidth = int(os.environ.get('MAX_BANDWIDTH') or 0)
    else:
        max_bandwidth = 0
    if args.live_start_index is not None:
        live_start_index = args.live_start_index
    elif os.environ.get('LIVE_START_INDEX') is not None:
        live_start_index = int(os.environ.get('LIVE_START_INDEX') or 0)
    else:
        live_start_index = -2 if args.push_url else None
    rw_timeout = args.rw_timeout if args.rw_timeout is not None else env_int('RW_TIMEOUT_US', 15000000)
    if args.hls_max_reload is not None:
        hls_max_reload = args.hls_max_reload
    else:
        hls_max_reload = env_int('HLS_MAX_RELOAD', 20 if args.push_url else 15)
    if args.hls_hold_counters is not None:
        hls_hold_counters = args.hls_hold_counters
    else:
        hls_hold_counters = env_int('HLS_HOLD_COUNTERS', 20 if args.push_url else 15)
    audio_mode = (args.audio_mode or 'source').lower()

    if args.resolve_only:
        cookie_jar, dossier, selected, child_hls, audio_hls, room_page = resolve_stream(
            args,
            max_height,
            max_bandwidth,
        )
        del cookie_jar, child_hls, audio_hls, room_page
        print_stream_info(
            args,
            dossier,
            selected,
            max_height,
            max_bandwidth,
            live_start_index,
            rw_timeout,
            hls_max_reload,
            hls_hold_counters,
            audio_mode,
        )
        print('resolve_status: ok')
        return 0

    if args.push_url:
        return run_push_loop(
            args,
            duration,
            max_height,
            max_bandwidth,
            live_start_index,
            rw_timeout,
            hls_max_reload,
            hls_hold_counters,
            audio_mode,
        )

    cookie_jar, dossier, selected, child_hls, audio_hls, room_page = resolve_stream(
        args,
        max_height,
        max_bandwidth,
    )
    print_stream_info(
        args,
        dossier,
        selected,
        max_height,
        max_bandwidth,
        live_start_index,
        rw_timeout,
        hls_max_reload,
        hls_hold_counters,
        audio_mode,
    )

    start = time.time()
    cmd = build_ffmpeg_cmd(
        child_hls,
        audio_hls,
        audio_mode,
        room_page,
        args.user_agent,
        cookie_header(cookie_jar),
        args.push_url,
        duration,
        live_start_index,
        rw_timeout,
        hls_max_reload,
        hls_hold_counters,
        unified_av=bool(dossier.get('_unified_av')),
        input_format=dossier.get('_stream_format', 'hls'),
    )

    try:
        process = subprocess.run(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=duration + 45,
        )
    except subprocess.TimeoutExpired as exc:
        elapsed = time.time() - start
        stdout = exc.stdout or ''
        stderr = exc.stderr or ''
        if isinstance(stdout, bytes):
            stdout = stdout.decode('utf-8', 'replace')
        if isinstance(stderr, bytes):
            stderr = stderr.decode('utf-8', 'replace')
        output = red_text(stdout + stderr)
        print('returncode: timeout')
        print('elapsed_seconds:', round(elapsed, 1))
        for line in output.splitlines()[-25:]:
            print(line)
        return 124
    elapsed = time.time() - start
    output = red_text((process.stdout or '') + (process.stderr or ''))
    print('returncode:', process.returncode)
    print('elapsed_seconds:', round(elapsed, 1))

    interesting = []
    for line in output.splitlines():
        if (
            any(
                key in line
                for key in [
                    'Input #',
                    'Stream #',
                    'frame=',
                    'video:',
                    'audio:',
                    'time=',
                    'bitrate=',
                    'speed=',
                    'Error',
                    'HTTP error',
                    'Forbidden',
                ]
            )
            or line.startswith('  Duration:')
        ):
            interesting.append(line)
    for line in interesting[-25:]:
        print(line)
    return process.returncode


if __name__ == '__main__':
    raise SystemExit(main())
