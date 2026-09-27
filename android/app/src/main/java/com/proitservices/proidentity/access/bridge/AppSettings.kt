package com.proitservices.proidentity.access.bridge

import android.content.Context
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import java.io.File

private const val PREFS_NAME = "wg_client_secure_prefs"

class AppSettings(context: Context) {

    private val masterKey = MasterKey.Builder(context)
        .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
        .build()

    private val prefs = try {
        EncryptedSharedPreferences.create(
            context,
            PREFS_NAME,
            masterKey,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
        )
    } catch (_: Exception) {
        // Keystore key was invalidated (reinstall, backup/restore, etc.).
        // Delete the corrupted file and start with a clean slate.
        File(context.filesDir.parent, "shared_prefs/$PREFS_NAME.xml").delete()
        EncryptedSharedPreferences.create(
            context,
            PREFS_NAME,
            masterKey,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
        )
    }

    // --- Accessors ---

    var serverURL: String
        get() = prefs.getString("serverURL", "") ?: ""
        set(v) = prefs.edit().putString("serverURL", v).apply()

    var token: String
        get() = prefs.getString("token", "") ?: ""
        set(v) = prefs.edit().putString("token", v).apply()

    var username: String
        get() = prefs.getString("username", "") ?: ""
        set(v) = prefs.edit().putString("username", v).apply()

    var isAdmin: Boolean
        get() = prefs.getBoolean("isAdmin", false)
        set(v) = prefs.edit().putBoolean("isAdmin", v).apply()

    var vpnName: String
        get() = prefs.getString("vpnName", "") ?: ""
        set(v) = prefs.edit().putString("vpnName", v).apply()

    var totpEnabled: Boolean
        get() = prefs.getBoolean("totpEnabled", false)
        set(v) = prefs.edit().putBoolean("totpEnabled", v).apply()

    var deviceID: String
        get() = prefs.getString("deviceID", "") ?: ""
        set(v) = prefs.edit().putString("deviceID", v).apply()

    var clientPrivateKey: String
        get() = prefs.getString("clientPrivateKey", "") ?: ""
        set(v) = prefs.edit().putString("clientPrivateKey", v).apply()

    var serverPublicKey: String
        get() = prefs.getString("serverPublicKey", "") ?: ""
        set(v) = prefs.edit().putString("serverPublicKey", v).apply()

    var mode: String
        get() = prefs.getString("mode", "") ?: ""
        set(v) = prefs.edit().putString("mode", v).apply()

    var setupDone: Boolean
        get() = prefs.getBoolean("setupDone", false)
        set(v) = prefs.edit().putBoolean("setupDone", v).apply()

    /** Key of the connection the user last connected ("server:<id>" or "tunnel:<id>"). */
    var lastConnection: String
        get() = prefs.getString("lastConnection", "") ?: ""
        set(v) = prefs.edit().putString("lastConnection", v).apply()

    /** Imported (standalone) configs as JSON [{id, name, config}]; encrypted at rest. */
    var importedTunnels: String
        get() = prefs.getString("importedTunnels", "") ?: ""
        set(v) = prefs.edit().putString("importedTunnels", v).apply()

    /**
     * Ends the login session only. Server URL, device registration (ID + keys),
     * username and setup state are kept so the user can sign back in with just
     * a password / 2FA — the soft re-login path.
     */
    fun clearSession() {
        prefs.edit()
            .putString("token", "")
            .putBoolean("isAdmin", false)
            .apply()
    }

    fun wipeAll() {
        prefs.edit().clear().apply()
    }
}
