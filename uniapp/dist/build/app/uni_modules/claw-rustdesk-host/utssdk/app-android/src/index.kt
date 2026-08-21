@file:Suppress("UNCHECKED_CAST", "USELESS_CAST", "INAPPLICABLE_JVM_NAME", "UNUSED_ANONYMOUS_PARAMETER", "SENSELESS_COMPARISON", "NAME_SHADOWING", "UNNECESSARY_NOT_NULL_ASSERTION")
package uts.sdk.modules.clawRustdeskHost
import com.claw.remote.HostSdk
import io.dcloud.uniapp.*
import io.dcloud.uniapp.extapi.*
import io.dcloud.uts.*
import io.dcloud.uts.Map
import io.dcloud.uts.Set
import kotlin.properties.Delegates
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import io.dcloud.uts.UTSAndroid
open class RemoteHostStatus (
    @JsonNotNull
    open var available: Boolean = false,
    @JsonNotNull
    open var configured: Boolean = false,
    @JsonNotNull
    open var running: Boolean = false,
    open var device_code: String? = null,
    @JsonNotNull
    open var service_status: String,
    open var message: String? = null,
    @JsonNotNull
    open var permissions: UTSJSONObject,
) : UTSObject()
typealias RemoteHostCallback = (status: RemoteHostStatus) -> Unit
@JvmField
val status = fun(): RemoteHostStatus {
    val context = UTSAndroid.getAppContext()
    if (context == null) {
        return RemoteHostStatus(available = false, configured = false, running = false, service_status = "error", message = "Android Context 不可用", permissions = _uO())
    }
    return JSON.parse<RemoteHostStatus>(HostSdk.statusJson(context))!!
}
fun initialize(options: UTSJSONObject, callback: RemoteHostCallback): Unit {
    val context = UTSAndroid.getAppContext()
    if (context != null) {
        HostSdk.initialize(context, JSON.stringify(options))
    }
    callback(status())
}
fun start(callback: RemoteHostCallback): Unit {
    val context = UTSAndroid.getAppContext()
    if (context != null) {
        HostSdk.start(context)
    }
    callback(status())
}
fun stop(options: UTSJSONObject, callback: RemoteHostCallback): Unit {
    val context = UTSAndroid.getAppContext()
    if (context != null) {
        HostSdk.stop(context, options["clear_credentials"] == true)
    }
    callback(status())
}
fun getStatus(callback: RemoteHostCallback): Unit {
    callback(status())
}
fun openPermissionSettings(permission: String, callback: RemoteHostCallback): Unit {
    val context = UTSAndroid.getAppContext()
    if (context != null) {
        HostSdk.openPermissionSettings(context, permission)
    }
    callback(status())
}
fun initializeByJs(options: UTSJSONObject, callback: UTSCallback): Unit {
    return initialize(options, if (callback.fnJS != null) {
        callback.fnJS
    } else {
        callback.fnJS = fun(status: RemoteHostStatus){
            callback(status)
        }
        callback.fnJS
    }
     as (status: RemoteHostStatus) -> Unit)
}
fun startByJs(callback: UTSCallback): Unit {
    return start(if (callback.fnJS != null) {
        callback.fnJS
    } else {
        callback.fnJS = fun(status: RemoteHostStatus){
            callback(status)
        }
        callback.fnJS
    }
     as (status: RemoteHostStatus) -> Unit)
}
fun stopByJs(options: UTSJSONObject, callback: UTSCallback): Unit {
    return stop(options, if (callback.fnJS != null) {
        callback.fnJS
    } else {
        callback.fnJS = fun(status: RemoteHostStatus){
            callback(status)
        }
        callback.fnJS
    }
     as (status: RemoteHostStatus) -> Unit)
}
fun getStatusByJs(callback: UTSCallback): Unit {
    return getStatus(if (callback.fnJS != null) {
        callback.fnJS
    } else {
        callback.fnJS = fun(status: RemoteHostStatus){
            callback(status)
        }
        callback.fnJS
    }
     as (status: RemoteHostStatus) -> Unit)
}
fun openPermissionSettingsByJs(permission: String, callback: UTSCallback): Unit {
    return openPermissionSettings(permission, if (callback.fnJS != null) {
        callback.fnJS
    } else {
        callback.fnJS = fun(status: RemoteHostStatus){
            callback(status)
        }
        callback.fnJS
    }
     as (status: RemoteHostStatus) -> Unit)
}
