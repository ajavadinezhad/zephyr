package com.zyrln.relay

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.ProxyInfo
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.util.Log
import androidx.core.app.NotificationCompat
import mobile.Mobile
import java.io.File

class RelayVpnService : VpnService() {

    private var vpnInterface: ParcelFileDescriptor? = null

    companion object {
        const val TAG = "RelayVpnService"
        const val ACTION_START = "com.zyrln.relay.START"
        const val ACTION_STOP = "com.zyrln.relay.STOP"
        const val ACTION_ERROR = "com.zyrln.relay.ERROR"
        const val EXTRA_URL = "url"
        const val EXTRA_KEY = "key"
        const val EXTRA_ERROR = "error"
        const val NOTIF_ID = 1
        const val CHANNEL_ID = "zyrln_vpn"
        private const val PROXY_PORT = 8085

        // Google's published IPv4 ranges (gstatic.com/ipranges/goog.json).
        // Routed into TUN so tun2socks captures Cronet/app traffic to Google.
        private val GOOGLE_CIDRS = arrayOf(
            "8.8.4.0/24", "8.8.8.0/24", "8.34.208.0/20", "8.35.192.0/20",
            "23.236.48.0/20", "23.251.128.0/19",
            "34.0.0.0/15", "34.2.0.0/16", "34.3.0.0/23", "34.3.3.0/24",
            "34.3.4.0/24", "34.3.8.0/21", "34.3.16.0/20", "34.3.32.0/19",
            "34.3.64.0/18", "34.4.0.0/14", "34.8.0.0/13", "34.16.0.0/12",
            "34.32.0.0/11", "34.64.0.0/10", "34.128.0.0/10",
            "35.184.0.0/13", "35.192.0.0/14", "35.196.0.0/15", "35.198.0.0/16",
            "35.199.0.0/17", "35.199.128.0/18", "35.200.0.0/13", "35.208.0.0/12",
            "35.224.0.0/12", "35.240.0.0/13", "35.252.0.0/14",
            "64.15.112.0/20", "64.233.160.0/19",
            "66.102.0.0/20", "66.249.64.0/19",
            "70.32.128.0/19", "72.14.192.0/18",
            "74.114.24.0/21", "74.125.0.0/16",
            "104.154.0.0/15", "104.196.0.0/14", "104.237.160.0/19",
            "107.167.160.0/19", "107.178.192.0/18",
            "108.59.80.0/20", "108.170.192.0/18", "108.177.0.0/17",
            "130.211.0.0/16",
            "136.22.2.0/23", "136.22.4.0/23", "136.22.8.0/22",
            "136.22.160.0/20", "136.22.176.0/21", "136.22.184.0/23",
            "136.22.186.0/24", "136.23.48.0/20", "136.23.64.0/18",
            "136.64.0.0/11", "136.107.0.0/16", "136.108.0.0/14",
            "136.112.0.0/13", "136.120.0.0/22", "136.124.0.0/15",
            "142.250.0.0/15", "142.251.0.0/16",
            "146.148.0.0/17",
            "162.120.128.0/17", "162.216.148.0/22", "162.222.176.0/21",
            "172.110.32.0/21", "172.217.0.0/16", "172.253.0.0/16",
            "173.194.0.0/16", "173.255.112.0/20",
            "192.104.160.0/23", "192.158.28.0/22", "192.178.0.0/15",
            "193.186.4.0/24",
            "199.36.154.0/23", "199.36.156.0/24",
            "199.192.112.0/22", "199.223.232.0/21",
            "207.175.0.0/16", "207.223.160.0/20",
            "208.65.152.0/22", "208.68.108.0/22", "208.81.188.0/22",
            "208.117.224.0/19", "209.85.128.0/17",
            "216.58.192.0/19", "216.73.80.0/20", "216.239.32.0/19",
            "216.252.220.0/22"
        )
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_STOP) {
            stopRelay()
            return START_NOT_STICKY
        }

        val url = intent?.getStringExtra(EXTRA_URL) ?: return START_NOT_STICKY
        val key = intent.getStringExtra(EXTRA_KEY) ?: return START_NOT_STICKY

        startForeground(NOTIF_ID, buildNotification())
        startRelay(url, key)
        return START_STICKY
    }

    private fun startRelay(url: String, key: String) {
        val err = if (url.isEmpty()) {
            Mobile.startDirect("127.0.0.1:$PROXY_PORT")
        } else {
            val certDir = File(filesDir, "certs").also { it.mkdirs() }
            val certPath = File(certDir, "ca.pem").absolutePath
            val keyPath = File(certDir, "ca.key").absolutePath
            if (!File(certPath).exists() || !File(keyPath).exists()) {
                failStart(getString(R.string.error_ca_required))
                return
            }
            Mobile.start(url, key, "127.0.0.1:$PROXY_PORT", certPath, keyPath)
        }
        if (err.isNotEmpty()) {
            Log.e(TAG, "relay start failed: $err")
            failStart(getString(R.string.error_relay_start_failed, err))
            return
        }
        Log.i(TAG, "relay proxy started on 127.0.0.1:$PROXY_PORT")

        val builder = Builder()
            .setSession("Zyrln")
            .addAddress("10.99.0.2", 32)
            .setHttpProxy(ProxyInfo.buildDirectProxy("127.0.0.1", PROXY_PORT))

        // TUN routes only needed when relay is active (captures Cronet/app traffic
        // that ignores system proxy). Direct-only mode relies solely on setHttpProxy.
        val useTun = url.isNotEmpty()
        if (useTun) {
            for (cidr in GOOGLE_CIDRS) {
                val parts = cidr.split("/")
                builder.addRoute(parts[0], parts[1].toInt())
            }
        }

        try {
            vpnInterface = builder.establish()
            Log.i(TAG, "VPN interface established")
            if (useTun) {
                Mobile.startTun(vpnInterface!!.fd.toLong(), "http://127.0.0.1:$PROXY_PORT")
            }
            sendBroadcast(Intent("com.zyrln.relay.STARTED"))
        } catch (e: Exception) {
            Log.e(TAG, "VPN establish failed: ${e.message}")
            Mobile.stop()
            vpnInterface?.close()
            vpnInterface = null
            stopSelf()
        }
    }

    private fun failStart(message: String) {
        Log.e(TAG, message)
        Mobile.stop()
        vpnInterface?.close()
        vpnInterface = null
        stopForeground(STOP_FOREGROUND_REMOVE)
        sendBroadcast(Intent(ACTION_ERROR).putExtra(EXTRA_ERROR, message))
        stopSelf()
    }

    private fun stopRelay() {
        Log.i(TAG, "stopping relay")
        Mobile.stopTun()
        Mobile.stop()
        vpnInterface?.close()
        vpnInterface = null
        stopForeground(STOP_FOREGROUND_REMOVE)
        sendBroadcast(Intent("com.zyrln.relay.STOPPED"))
        stopSelf()
    }

    override fun onDestroy() {
        Mobile.stopTun()
        Mobile.stop()
        vpnInterface?.close()
        sendBroadcast(Intent("com.zyrln.relay.STOPPED"))
        super.onDestroy()
    }

    private fun buildNotification(): Notification {
        createNotificationChannel()

        val openIntent = PendingIntent.getActivity(
            this, 0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        val stopIntent = PendingIntent.getService(
            this, 0,
            Intent(this, RelayVpnService::class.java).apply { action = ACTION_STOP },
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(getString(R.string.vpn_notification_title))
            .setContentText(getString(R.string.vpn_notification_text))
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setContentIntent(openIntent)
            .addAction(android.R.drawable.ic_media_pause, "Stop", stopIntent)
            .setOngoing(true)
            .build()
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                getString(R.string.channel_name),
                NotificationManager.IMPORTANCE_LOW
            )
            getSystemService(NotificationManager::class.java)
                .createNotificationChannel(channel)
        }
    }
}
