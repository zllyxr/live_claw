package com.claw.remote

import android.accessibilityservice.AccessibilityService
import android.content.Context
import android.graphics.Bitmap
import android.graphics.PixelFormat
import android.hardware.display.DisplayManager
import android.hardware.display.VirtualDisplay
import android.media.ImageReader
import android.media.projection.MediaProjection
import android.os.Handler
import android.os.HandlerThread
import android.view.Surface
import android.view.WindowManager
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong
import kotlin.math.max
import kotlin.math.roundToInt

internal class NativeCoreAdapter : CoreAdapter {
    private var context: Context? = null
    private var config: HostConfig? = null
    private var eventSink: HostEventSink? = null
    private var inputService: AccessibilityService? = null
    private var reader: ImageReader? = null
    private var display: VirtualDisplay? = null
    private var captureThread: HandlerThread? = null
    private val network = Executors.newSingleThreadExecutor()
    private val uploading = AtomicBoolean(false)
    private val sequence = AtomicLong(0)
    @Volatile private var lastCaptureMillis = 0L
    @Volatile private var rotationDegrees = 0

    override fun initialize(context: Context, config: HostConfig, eventSink: HostEventSink) {
        this.context = context.applicationContext
        this.config = config
        this.eventSink = eventSink
    }

    override fun attachAccessibilityService(service: AccessibilityService) {
        inputService = service
    }

    override fun detachAccessibilityService(service: AccessibilityService) {
        if (inputService === service) inputService = null
    }

    override fun start(projection: MediaProjection) {
        stopCapture()
        val appContext = context ?: return
        val metrics = appContext.resources.displayMetrics
        val scale = minOf(1.0, MAX_CAPTURE_EDGE.toDouble() / max(metrics.widthPixels, metrics.heightPixels).toDouble())
        val width = max(2, (metrics.widthPixels * scale).roundToInt() and -2)
        val height = max(2, (metrics.heightPixels * scale).roundToInt() and -2)
        @Suppress("DEPRECATION")
        val rotation = (appContext.getSystemService(Context.WINDOW_SERVICE) as WindowManager).defaultDisplay.rotation
        rotationDegrees = when (rotation) {
            Surface.ROTATION_90 -> 90
            Surface.ROTATION_180 -> 180
            Surface.ROTATION_270 -> 270
            else -> 0
        }
        val thread = HandlerThread("claw-remote-capture").also { it.start() }
        captureThread = thread
        val imageReader = ImageReader.newInstance(width, height, PixelFormat.RGBA_8888, 2)
        reader = imageReader
        imageReader.setOnImageAvailableListener({ source -> capture(source, width, height) }, Handler(thread.looper))
        display = projection.createVirtualDisplay(
            "ClawRemoteScreen", width, height, metrics.densityDpi,
            DisplayManager.VIRTUAL_DISPLAY_FLAG_AUTO_MIRROR,
            imageReader.surface, null, Handler(thread.looper),
        )
        eventSink?.emit(HostEvent("authorized"))
    }

    override fun stop() {
        stopCapture()
        eventSink?.emit(HostEvent("disconnected", metadata = mapOf("result" to "success")))
    }

    private fun stopCapture() {
        display?.release()
        display = null
        reader?.close()
        reader = null
        captureThread?.quitSafely()
        captureThread = null
        uploading.set(false)
    }

    private fun capture(source: ImageReader, width: Int, height: Int) {
        val image = source.acquireLatestImage() ?: return
        try {
            val now = System.currentTimeMillis()
            if (now - lastCaptureMillis < FRAME_INTERVAL_MS || !uploading.compareAndSet(false, true)) return
            lastCaptureMillis = now
            val plane = image.planes.firstOrNull() ?: run { uploading.set(false); return }
            val rowWidth = plane.rowStride / plane.pixelStride
            val padded = Bitmap.createBitmap(rowWidth, height, Bitmap.Config.ARGB_8888)
            plane.buffer.rewind()
            padded.copyPixelsFromBuffer(plane.buffer)
            val bitmap = if (rowWidth == width) padded else Bitmap.createBitmap(padded, 0, 0, width, height).also { padded.recycle() }
            val output = ByteArrayOutputStream(256 * 1024)
            bitmap.compress(Bitmap.CompressFormat.JPEG, JPEG_QUALITY, output)
            bitmap.recycle()
            val jpeg = output.toByteArray()
            val frameSequence = sequence.incrementAndGet()
            network.execute {
                try { uploadFrame(jpeg, width, height, rotationDegrees, frameSequence) }
                catch (_: Throwable) { }
                finally { uploading.set(false) }
            }
        } catch (_: Throwable) {
            uploading.set(false)
        } finally {
            image.close()
        }
    }

    private fun uploadFrame(jpeg: ByteArray, width: Int, height: Int, rotation: Int, frameSequence: Long) {
        val appContext = context ?: return
        val currentConfig = config ?: return
        val token = SecureConfigStore.token(appContext)
        if (token.isBlank() || jpeg.size > MAX_FRAME_BYTES) return
        val endpoint = currentConfig.backendUrl + "/remote/devices/frame"
        require(endpoint.startsWith("https://"))
        val connection = (URL(endpoint).openConnection() as HttpURLConnection).apply {
            requestMethod = "POST"
            connectTimeout = 5_000
            readTimeout = 8_000
            doOutput = true
            setFixedLengthStreamingMode(jpeg.size)
            useCaches = false
            instanceFollowRedirects = false
            setRequestProperty("Authorization", "Device $token")
            setRequestProperty("Content-Type", "image/jpeg")
            setRequestProperty("Cache-Control", "no-store")
            setRequestProperty("X-Frame-Width", width.toString())
            setRequestProperty("X-Frame-Height", height.toString())
            setRequestProperty("X-Frame-Rotation", rotation.toString())
            setRequestProperty("X-Frame-Sequence", frameSequence.toString())
        }
        try {
            connection.outputStream.use { it.write(jpeg) }
            if (connection.responseCode !in 200..299) throw IllegalStateException("frame rejected")
            connection.inputStream?.close()
        } finally {
            connection.disconnect()
        }
    }

    override fun deviceCode(): String = config?.deviceId.orEmpty()
    override fun tap(x: Double, y: Double): Boolean = (inputService as? RemoteInputService)?.tap(x, y) == true
    override fun swipe(x1: Double, y1: Double, x2: Double, y2: Double, durationMillis: Long): Boolean =
        (inputService as? RemoteInputService)?.swipe(x1, y1, x2, y2, durationMillis) == true
    override fun systemAction(action: String): Boolean = (inputService as? RemoteInputService)?.systemAction(action) == true
    override fun inputText(text: String): Boolean = (inputService as? RemoteInputService)?.inputText(text) == true
    override fun setClipboard(text: String): Boolean = (inputService as? RemoteInputService)?.setClipboard(text) == true

    companion object {
        private const val MAX_CAPTURE_EDGE = 960
        private const val FRAME_INTERVAL_MS = 450L
        private const val JPEG_QUALITY = 55
        private const val MAX_FRAME_BYTES = 768 shl 10
    }
}
