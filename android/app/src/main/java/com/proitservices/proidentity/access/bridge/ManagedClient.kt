package com.proitservices.proidentity.access.bridge

import android.util.Base64
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONArray
import org.json.JSONObject
import java.net.URI
import java.util.concurrent.TimeUnit

data class RegisterResponse(val deviceId: String, val serverPublicKey: String)
data class LoginResponse(
    val token: String,
    val userId: String,
    val username: String,
    val isAdmin: Boolean,
    val requireTotp: Boolean,
    val totpEnabled: Boolean,
    val pushAuthEnabled: Boolean = false,
    val pushRequestId: String = ""
)
data class InfoResponse(val vpnName: String)
data class ServerInfo(
    val id: String,
    val name: String,
    val endpoint: String,
    val port: Int,
    val publicKey: String,
    val subnet: String,
    val dns: String?
)
data class EndpointCandidate(
    val role: String,
    val host: String,
    val ip: String,
    val port: Int,
    val priority: Int,
    val endpoint: String
)
data class SessionResponse(
    val sessionId: String,
    val assignedIp: String,
    val serverId: String,
    val wgConfig: String,
    val vpnName: String,
    val endpoints: List<EndpointCandidate> = emptyList(),
    val requireTotp: Boolean,
    val pushAuthEnabled: Boolean = false
)
data class UserConfigInfo(val id: String, val name: String, val createdAt: String)

class DeviceRevokedException : Exception("device revoked")
class AuthInvalidException : Exception("auth invalid")

class ManagedClient(
    val baseURL: String,
    val token: String = "",
    val deviceID: String = "",
    val aesKey: ByteArray? = null
) {
    init {
        validateBaseURL(baseURL)
    }

    private val http = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(15, TimeUnit.SECONDS)
        .build()

    private val JSON_TYPE = "application/json; charset=utf-8".toMediaType()

    private fun validateBaseURL(value: String) {
        val uri = try {
            URI(value.trim())
        } catch (_: Exception) {
            throw IllegalArgumentException("Invalid server URL")
        }
        val scheme = uri.scheme?.lowercase() ?: throw IllegalArgumentException("Server URL must include https://")
        val host = uri.host ?: throw IllegalArgumentException("Server URL must include a host")
        val localhost = host == "localhost" || host == "127.0.0.1" || host == "::1"
        if (scheme != "https" && !(scheme == "http" && localhost)) {
            throw IllegalArgumentException("Server URL must use HTTPS")
        }
        if (!uri.rawQuery.isNullOrEmpty() || !uri.rawFragment.isNullOrEmpty()) {
            throw IllegalArgumentException("Server URL must not include query or fragment")
        }
    }

    // --- Core request helpers ---

    private fun buildRequest(path: String, body: JSONObject? = null, method: String = if (body == null) "GET" else "POST"): Request {
        val url = "${baseURL.trimEnd('/')}/api/v1$path"
        val builder = Request.Builder().url(url)

        builder.header("User-Agent", "ProIdentity-Access")
        if (token.isNotEmpty()) builder.header("Authorization", "Bearer $token")
        if (deviceID.isNotEmpty()) builder.header("X-Device-ID", deviceID)

        if (body != null) {
            val bodyBytes = body.toString().toByteArray(Charsets.UTF_8)
            val finalBody = if (aesKey != null) {
                val encrypted = DeviceCrypto.encryptBody(aesKey, bodyBytes, deviceID.toByteArray(Charsets.UTF_8))
                encrypted.toByteArray(Charsets.UTF_8).toRequestBody(JSON_TYPE)
            } else {
                bodyBytes.toRequestBody(JSON_TYPE)
            }
            when (method) {
                "POST"   -> builder.post(finalBody)
                "PUT"    -> builder.put(finalBody)
                "PATCH"  -> builder.patch(finalBody)
                "DELETE" -> builder.delete(finalBody)
                else     -> builder.post(finalBody)
            }
        } else {
            when (method) {
                "DELETE" -> builder.delete()
                "GET"    -> builder.get()
                else     -> builder.get()
            }
        }

        return builder.build()
    }

    private fun execute(request: Request): String {
        val response = http.newCall(request).execute()
        val bodyStr = response.body?.string() ?: ""

        if (!response.isSuccessful) {
            // Check for device revocation
            if (response.code == 401) {
                try {
                    val errObj = JSONObject(bodyStr)
                    val errMsg = errObj.optString("error", "")
                    if (errMsg == "device revoked" || errMsg == "unknown device") {
                        throw DeviceRevokedException()
                    }
                    if (token.isNotEmpty()) {
                        throw AuthInvalidException()
                    }
                } catch (e: DeviceRevokedException) {
                    throw e
                } catch (e: AuthInvalidException) {
                    throw e
                } catch (_: Exception) {}
            }
            if (token.isNotEmpty() && (response.code == 401 || response.code == 403)) {
                throw AuthInvalidException()
            }
            throw RuntimeException("HTTP ${response.code}: $bodyStr")
        }

        if (bodyStr.isEmpty()) return "{}"

        return if (aesKey != null && deviceID.isNotEmpty()) {
            val decrypted = DeviceCrypto.decryptBody(aesKey, bodyStr, deviceID.toByteArray(Charsets.UTF_8))
            String(decrypted, Charsets.UTF_8)
        } else {
            bodyStr
        }
    }

    private fun get(path: String): String = execute(buildRequest(path))

    private fun post(path: String, body: JSONObject): String =
        execute(buildRequest(path, body, "POST"))

    private fun delete(path: String): String =
        execute(buildRequest(path, method = "DELETE"))

    private fun deleteWithBody(path: String, body: JSONObject): String =
        execute(buildRequest(path, body, "DELETE"))

    // --- API methods ---

    fun register(deviceName: String, clientPubKey: String): RegisterResponse {
        val body = JSONObject().apply {
            put("device_name", deviceName)
            put("client_public_key", clientPubKey)
        }
        // Register does not use encryption (no aesKey yet); send plain
        val url = "${baseURL.trimEnd('/')}/api/v1/register"
        val req = Request.Builder()
            .url(url)
            .post(body.toString().toByteArray().toRequestBody(JSON_TYPE))
            .build()
        val resp = http.newCall(req).execute()
        val respBody = resp.body?.string() ?: ""
        if (!resp.isSuccessful) throw RuntimeException("HTTP ${resp.code}: $respBody")
        val obj = JSONObject(respBody)
        return RegisterResponse(
            deviceId = obj.getString("device_id"),
            serverPublicKey = obj.getString("server_public_key")
        )
    }

    fun login(username: String, password: String, totpCode: String = "", pushAuthID: String = ""): LoginResponse {
        val body = JSONObject().apply {
            put("username", username)
            put("password", password)
            if (totpCode.isNotEmpty()) put("totp_code", totpCode)
            if (pushAuthID.isNotEmpty()) put("push_auth_request_id", pushAuthID)
        }
        val resp = JSONObject(post("/auth/login", body))
        return LoginResponse(
            token = resp.optString("token", ""),
            userId = resp.optString("user_id", ""),
            username = resp.optString("username", username),
            isAdmin = resp.optBoolean("is_admin", false),
            requireTotp = resp.optBoolean("require_totp", false),
            totpEnabled = resp.optBoolean("totp_enabled", false),
            pushAuthEnabled = resp.optBoolean("push_auth_enabled", false),
            pushRequestId = resp.optString("push_request_id", "")
        )
    }

    fun getInfo(): InfoResponse {
        val resp = JSONObject(get("/info"))
        return InfoResponse(vpnName = resp.optString("vpn_name", ""))
    }

    fun listServers(): List<ServerInfo> {
        val body = get("/servers")
        val arr = try { JSONArray(body) } catch (_: Exception) { return emptyList() }
        return (0 until arr.length()).map { i ->
            val obj = arr.getJSONObject(i)
            ServerInfo(
                id = obj.getString("id"),
                name = obj.getString("name"),
                endpoint = obj.getString("endpoint"),
                port = obj.getInt("port"),
                publicKey = obj.getString("public_key"),
                subnet = obj.getString("subnet"),
                dns = obj.optString("dns", "").takeIf { it.isNotEmpty() }
            )
        }
    }

    fun createSession(serverID: String, clientPubKey: String, totpCode: String = "", pushAuthID: String = ""): SessionResponse {
        val body = JSONObject().apply {
            put("server_id", serverID)
            put("client_public_key", clientPubKey)
            if (totpCode.isNotEmpty()) put("totp_code", totpCode)
            if (pushAuthID.isNotEmpty()) put("push_auth_request_id", pushAuthID)
        }
        val resp = JSONObject(post("/sessions", body))
        if (resp.optBoolean("require_totp", false)) {
            return SessionResponse(
                sessionId = "", assignedIp = "", serverId = serverID,
                wgConfig = "", vpnName = "", requireTotp = true,
                pushAuthEnabled = resp.optBoolean("push_auth_enabled", false)
            )
        }
        return SessionResponse(
            sessionId = resp.getString("session_id"),
            assignedIp = resp.getString("assigned_ip"),
            serverId = resp.optString("server_id", serverID),
            wgConfig = resp.getString("wg_config"),
            vpnName = resp.optString("vpn_name", ""),
            endpoints = parseEndpointCandidates(resp.optJSONArray("endpoints")),
            requireTotp = false
        )
    }

    fun keepalive(sessionID: String) {
        val body = JSONObject()
        post("/sessions/$sessionID/keepalive", body)
    }

    fun deleteSession(sessionID: String) {
        try {
            delete("/sessions/$sessionID")
        } catch (_: Exception) {}
    }

    fun getUserConfigKey(): ByteArray {
        val resp = JSONObject(get("/user/config-key"))
        val keyB64 = resp.getString("key")
        return Base64.decode(keyB64, Base64.NO_WRAP)
    }

    fun listUserConfigs(): List<UserConfigInfo> {
        val body = get("/user/configs")
        val arr = try { JSONArray(body) } catch (_: Exception) { return emptyList() }
        return (0 until arr.length()).map { i ->
            val obj = arr.getJSONObject(i)
            UserConfigInfo(
                id = obj.getString("id"),
                name = obj.getString("name"),
                createdAt = obj.optString("created_at", "")
            )
        }
    }

    fun uploadUserConfig(name: String, encryptedContent: String): String {
        val body = JSONObject().apply {
            put("name", name)
            put("encrypted_content", encryptedContent)
        }
        val resp = JSONObject(post("/user/configs", body))
        return resp.getString("id")
    }

    fun downloadUserConfig(id: String): ByteArray {
        val resp = JSONObject(get("/user/configs/$id"))
        val b64 = resp.getString("encrypted_content")
        return Base64.decode(b64, Base64.NO_WRAP)
    }

    fun deleteUserConfig(id: String) {
        try {
            delete("/user/configs/$id")
        } catch (_: Exception) {}
    }

    // Push auth â€” create request (authenticated)
    fun createPushAuth(context: String): Pair<String, String> {
        val body = JSONObject().apply { put("context", context) }
        val resp = JSONObject(post("/auth/push", body))
        return Pair(resp.optString("request_id", ""), resp.optString("status", "pending"))
    }

    // Push auth â€” poll status (public, works without token for login flow)
    fun pollPushStatus(requestID: String): String {
        val url = "${baseURL.trimEnd('/')}/api/v1/auth/push-status/$requestID"
        val req = Request.Builder().url(url)
            .header("User-Agent", "ProIdentity-Access")
            .get().build()
        val response = http.newCall(req).execute()
        val bodyStr = response.body?.string() ?: "{}"
        if (!response.isSuccessful) return "error"
        return JSONObject(bodyStr).optString("status", "pending")
    }

    // Auth check â€” verifies token is still valid
    fun checkAuth() {
        get("/auth/me")
    }

    private fun parseEndpointCandidates(arr: JSONArray?): List<EndpointCandidate> {
        if (arr == null) return emptyList()
        return (0 until arr.length()).mapNotNull { i ->
            val obj = arr.optJSONObject(i) ?: return@mapNotNull null
            EndpointCandidate(
                role = obj.optString("role", ""),
                host = obj.optString("host", ""),
                ip = obj.optString("ip", ""),
                port = obj.optInt("port", 0),
                priority = obj.optInt("priority", i),
                endpoint = obj.optString("endpoint", "")
            )
        }
    }
}
