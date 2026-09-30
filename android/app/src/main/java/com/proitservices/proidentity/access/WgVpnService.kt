package com.proitservices.proidentity.access

import android.Manifest
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.net.VpnService
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.app.ServiceCompat
import com.proitservices.proidentity.access.bridge.AppSettings

class WgVpnService : VpnService() {
    companion object {
        var instance: WgVpnService? = null
        val stateListeners = mutableListOf<(String, String) -> Unit>() // (tunnelId, state)

        /**
         * Disconnect requested from the notification. Set by the UI so managed
         * connections also end their server session; without it the tunnel is
         * simply taken down.
         */
        var disconnectHandler: ((tunnelId: String) -> Unit)? = null

        const val ACTION_DISCONNECT = "com.proitservices.proidentity.access.DISCONNECT"
        const val EXTRA_TUNNEL_ID = "tunnelId"
        // Default importance so it is shown expanded (low importance lands in the
        // collapsed "Silent" group); sound and vibration are off on the channel.
        private const val CHANNEL_ID = "vpn_connection"
        private const val NOTIFICATION_ID = 1
    }

    data class TunnelEntry(
        val id: String,
        val name: String,
        val config: String,  // raw WireGuard config text
        var state: String = "disconnected"  // "connected", "disconnected", "error"
    )

    private var backend: com.wireguard.android.backend.GoBackend? = null
    private val activeTunnels = mutableMapOf<String, com.wireguard.android.backend.Tunnel>()
    private val tunnelEntries = mutableMapOf<String, TunnelEntry>()  // id -> entry
    private val connectedAt = mutableMapOf<String, Long>()
    private var inForeground = false

    /** Ids of imported tunnels that are saved across restarts (server tunnels are per-session). */
    private val persistedIds = mutableSetOf<String>()

    override fun onCreate() {
        super.onCreate()
        backend = com.wireguard.android.backend.GoBackend(this)
        createChannel()
        loadPersisted()
        instance = this
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_DISCONNECT) {
            val ids = intent.getStringExtra(EXTRA_TUNNEL_ID)?.let { listOf(it) } ?: activeTunnels.keys.toList()
            ids.forEach { id ->
                val handler = disconnectHandler
                if (handler != null) handler(id)
                else Thread { runCatching { disconnectTunnel(id) } }.start()
            }
        }
        return super.onStartCommand(intent, flags, startId)
    }

    override fun onDestroy() {
        instance = null
        super.onDestroy()
    }

    // Returns tunnel entry. Throws if not found.
    fun importTunnel(id: String, name: String, config: String, persist: Boolean = false): TunnelEntry {
        val entry = TunnelEntry(id, name, config)
        tunnelEntries[id] = entry
        if (persist) {
            persistedIds += id
            savePersisted()
        }
        return entry
    }

    private fun loadPersisted() {
        val raw = AppSettings(this).importedTunnels
        if (raw.isEmpty()) return
        runCatching {
            val arr = org.json.JSONArray(raw)
            for (i in 0 until arr.length()) {
                val o = arr.getJSONObject(i)
                val id = o.getString("id")
                tunnelEntries[id] = TunnelEntry(id, o.getString("name"), o.getString("config"))
                persistedIds += id
            }
        }
    }

    private fun savePersisted() {
        val arr = org.json.JSONArray()
        persistedIds.mapNotNull { tunnelEntries[it] }.forEach { e ->
            arr.put(org.json.JSONObject().put("id", e.id).put("name", e.name).put("config", e.config))
        }
        AppSettings(this).importedTunnels = arr.toString()
    }

    fun connectTunnel(id: String) {
        val entry = tunnelEntries[id] ?: throw IllegalArgumentException("Tunnel not found: $id")
        val parsed = com.wireguard.config.Config.parse(java.io.BufferedReader(java.io.StringReader(entry.config)))
        val tunnel = object : com.wireguard.android.backend.Tunnel {
            override fun getName() = entry.name
            override fun onStateChange(newState: com.wireguard.android.backend.Tunnel.State) {
                entry.state = if (newState == com.wireguard.android.backend.Tunnel.State.UP) "connected" else "disconnected"
                if (entry.state == "disconnected") {
                    activeTunnels.remove(id)
                    connectedAt.remove(id)
                    // Server tunnels are one-time: once down, their config and
                    // keys are gone (a reconnect gets a new session and config).
                    if (id.startsWith("managed-")) tunnelEntries.remove(id)
                }
                stateListeners.forEach { it(id, entry.state) }
                updateNotification()
            }
        }
        activeTunnels[id] = tunnel
        backend?.setState(tunnel, com.wireguard.android.backend.Tunnel.State.UP, parsed)
        entry.state = "connected"
        connectedAt.getOrPut(id) { System.currentTimeMillis() }
        stateListeners.forEach { it(id, "connected") }
        updateNotification()
    }

    fun disconnectTunnel(id: String) {
        val tunnel = activeTunnels[id] ?: return
        backend?.setState(tunnel, com.wireguard.android.backend.Tunnel.State.DOWN, null)
        activeTunnels.remove(id)
        connectedAt.remove(id)
        tunnelEntries[id]?.state = "disconnected"
        stateListeners.forEach { it(id, "disconnected") }
        updateNotification()
    }

    fun deleteTunnel(id: String) {
        disconnectTunnel(id)
        tunnelEntries.remove(id)
        if (persistedIds.remove(id)) savePersisted()
    }

    fun clearAllTunnels() {
        tunnelEntries.keys.toList().forEach { deleteTunnel(it) }
    }

    fun listTunnels(): List<TunnelEntry> = tunnelEntries.values.toList()

    fun getStats(id: String): com.wireguard.android.backend.Statistics? {
        val tunnel = activeTunnels[id] ?: return null
        return backend?.getStatistics(tunnel)
    }

    // --- Ongoing notification -------------------------------------------------------------

    private fun createChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val channel = NotificationChannel(CHANNEL_ID, getString(R.string.vpn_channel_name), NotificationManager.IMPORTANCE_DEFAULT).apply {
            description = getString(R.string.vpn_channel_description)
            setShowBadge(false)
            setSound(null, null)
            enableVibration(false)
        }
        getSystemService(NotificationManager::class.java).apply {
            deleteNotificationChannel("vpn_status") // earlier low-importance channel
            createNotificationChannel(channel)
        }
    }

    /**
     * While any tunnel is up the service runs in the foreground with an ongoing
     * notification: which network is connected, for how long, and a Disconnect
     * button. It also keeps the service (and managed-session keepalives) alive
     * when the app is in the background.
     */
    @Synchronized
    private fun updateNotification() {
        val active = activeTunnels.keys.mapNotNull { tunnelEntries[it] }
        if (active.isEmpty()) {
            if (inForeground) ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE)
            NotificationManagerCompat.from(this).cancel(NOTIFICATION_ID)
            inForeground = false
            return
        }
        val notification = buildNotification(active)
        try {
            ServiceCompat.startForeground(
                this, NOTIFICATION_ID, notification,
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE)
                    ServiceInfo.FOREGROUND_SERVICE_TYPE_SYSTEM_EXEMPTED else 0,
            )
            inForeground = true
        } catch (_: Exception) {
            // Not allowed to go foreground right now (e.g. started from the
            // background): still show the ongoing notification.
            val allowed = Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU ||
                checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
            if (allowed) runCatching { NotificationManagerCompat.from(this).notify(NOTIFICATION_ID, notification) }
        }
    }

    private fun buildNotification(active: List<TunnelEntry>): Notification {
        val first = active.first()
        val title = if (active.size == 1) getString(R.string.vpn_notification_connected, first.name)
        else resources.getQuantityString(R.plurals.vpn_notification_connected_many, active.size, active.size)
        val detail = if (active.size == 1) addressOf(first.config) else active.joinToString(", ") { it.name }
        val since = active.mapNotNull { connectedAt[it.id] }.minOrNull() ?: System.currentTimeMillis()

        val open = PendingIntent.getActivity(
            this, 0,
            Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val disconnect = PendingIntent.getService(
            this, 1,
            Intent(this, WgVpnService::class.java).setAction(ACTION_DISCONNECT),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val public = NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_vpn)
            .setContentTitle(getString(R.string.vpn_notification_public))
            .build()

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_stat_vpn)
            .setColor(0xFF22C55E.toInt())
            .setContentTitle(title)
            .setContentText(detail)
            .setSubText(getString(R.string.app_name))
            .setWhen(since)
            .setShowWhen(true)
            .setUsesChronometer(true)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setSilent(true)
            .setCategory(NotificationCompat.CATEGORY_SERVICE)
            .setPriority(NotificationCompat.PRIORITY_DEFAULT)
            .setVisibility(NotificationCompat.VISIBILITY_PRIVATE)
            .setPublicVersion(public)
            .setForegroundServiceBehavior(NotificationCompat.FOREGROUND_SERVICE_IMMEDIATE)
            .setContentIntent(open)
            .addAction(0, getString(R.string.vpn_notification_disconnect), disconnect)
            .build()
    }

    private fun addressOf(config: String): String =
        config.lineSequence()
            .map { it.trim() }
            .firstOrNull { it.startsWith("Address", ignoreCase = true) && it.contains('=') }
            ?.substringAfter('=')?.split(',')?.firstOrNull()?.trim()
            ?: ""
}
