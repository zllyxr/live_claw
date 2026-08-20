package com.claw.remote

import android.content.Context
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import org.json.JSONObject

internal object SecureConfigStore {
    private const val STORE = "claw_remote_secure_v1"
    private const val CONFIG = "config"
    private const val TOKEN = "device_token"

    private fun preferences(context: Context) = EncryptedSharedPreferences.create(
        context,
        STORE,
        MasterKey.Builder(context).setKeyScheme(MasterKey.KeyScheme.AES256_GCM).build(),
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
    )

    fun save(context: Context, config: HostConfig, token: String) {
        require(token.length >= 32 && config.backendUrl.startsWith("https://"))
        preferences(context).edit().putString(CONFIG, config.toJSON().toString()).putString(TOKEN, token).apply()
    }

    fun config(context: Context): HostConfig? = try {
        preferences(context).getString(CONFIG, null)?.let { HostConfig.fromJSON(JSONObject(it)) }
    } catch (_: Throwable) { null }

    fun token(context: Context): String = preferences(context).getString(TOKEN, "").orEmpty()

    fun clear(context: Context) { preferences(context).edit().clear().apply() }
}
