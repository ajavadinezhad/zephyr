package com.zyrln.relay

import android.animation.AnimatorInflater
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.animation.ArgbEvaluator
import android.animation.ValueAnimator
import androidx.appcompat.app.AlertDialog
import android.content.BroadcastReceiver
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.SharedPreferences
import android.content.res.ColorStateList
import android.graphics.Typeface
import android.net.VpnService
import android.os.Bundle
import android.provider.Settings
import android.util.Log
import android.view.Gravity
import android.view.View
import android.view.animation.Animation
import android.view.animation.AnimationUtils
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.appcompat.app.AppCompatDelegate
import androidx.cardview.widget.CardView
import androidx.core.content.ContextCompat
import androidx.core.os.LocaleListCompat
import com.zyrln.relay.databinding.ActivityMainBinding
import mobile.Mobile
import android.net.Uri
import java.io.File
import java.util.Locale

class MainActivity : AppCompatActivity() {

    companion object {
        // Survives activity recreation (theme/language change)
        private val logCache = android.text.SpannableStringBuilder()
    }

    private lateinit var binding: ActivityMainBinding
    private lateinit var prefs: SharedPreferences
    @Volatile private var activeUrl: String? = null
    @Volatile private var activeKey: String? = null
    private var selectedUrl: String? = null
    private var selectedKey: String? = null
    private var directOnlySelected: Boolean = false

    private val vpnPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == RESULT_OK) {
            launchVpnService()
        } else {
            activeUrl = null
            activeKey = null
            Toast.makeText(this, R.string.error_vpn_permission, Toast.LENGTH_SHORT).show()
            refreshList(running = false)
        }
    }

    private val createDocumentLauncher = registerForActivityResult(
        ActivityResultContracts.CreateDocument("application/x-x509-ca-cert")
    ) { uri ->
        uri?.let { saveCertToUri(it) }
    }

    private val startedReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            startUptimeTicker()
            updateUI(running = true)
        }
    }

    private val logPollHandler = Handler(Looper.getMainLooper())
    private val logPollTick = object : Runnable {
        override fun run() {
            val raw = Mobile.pollLogs()
            if (raw.isNotEmpty()) {
                raw.split('\n').forEach { line ->
                    val tab = line.indexOf('\t')
                    if (tab >= 0) appendLog(line.substring(0, tab), line.substring(tab + 1))
                }
            }
            logPollHandler.postDelayed(this, 500)
        }
    }

    private fun startLogPolling() {
        logPollHandler.removeCallbacks(logPollTick)
        logPollHandler.post(logPollTick)
    }

    private fun stopLogPolling() {
        logPollHandler.removeCallbacks(logPollTick)
    }

    private val stopReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            activeUrl = null
            activeKey = null
            stopUptimeTicker()
            updateUI(running = false)
        }
    }

    private val errorReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            activeUrl = null
            activeKey = null
            stopUptimeTicker()
            updateUI(running = false)
            val message = intent?.getStringExtra(RelayVpnService.EXTRA_ERROR)
                ?: getString(R.string.error_relay_start_generic)
            Toast.makeText(this@MainActivity, message, Toast.LENGTH_LONG).show()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)

        prefs = getSharedPreferences("config", Context.MODE_PRIVATE)
        Mobile.setCacheDir(filesDir.absolutePath)
        binding.versionTag.text = "v${BuildConfig.VERSION_NAME}"

        binding.btnImportConfig.setOnClickListener { importConfig() }
        binding.btnInstallCA.setOnClickListener { installCACert() }
        binding.btnLanguage.setOnClickListener { toggleLanguage() }
        binding.btnTheme.setOnClickListener { toggleTheme() }
        binding.btnConnect.setOnClickListener { onConnectClicked() }
        binding.btnDirect.setOnClickListener { toggleDirectMode() }
        binding.btnShareLog.setOnClickListener { shareLog() }
        binding.btnPing.setOnClickListener {
            // Direct-only mode: measure a fragmented connection to gstatic, no relay needed.
            if (directOnlySelected || (Mobile.isRunning() && activeUrl == null)) {
                binding.pingResult.visibility = View.VISIBLE
                binding.pingResult.text = "…"
                Thread {
                    val result = Mobile.pingDirect()
                    runOnUiThread { binding.pingResult.text = result }
                }.start()
                return@setOnClickListener
            }
            val configs = loadConfigs()
            if (configs.isEmpty()) {
                Toast.makeText(this, R.string.empty_configs, Toast.LENGTH_SHORT).show()
                return@setOnClickListener
            }
            // Use the selected row, or active row, or first config
            val (pingUrl, pingKey) = when {
                selectedUrl != null -> Pair(selectedUrl!!, selectedKey ?: "")
                activeUrl != null -> Pair(activeUrl!!, activeKey ?: "")
                else -> configs[0]
            }
            // Auto-select if nothing selected yet
            if (selectedUrl == null && activeUrl == null) {
                selectedUrl = configs[0].first
                selectedKey = configs[0].second
                refreshList()
            }
            binding.pingResult.visibility = View.VISIBLE
            binding.pingResult.text = "…"
            Thread {
                val result = Mobile.ping(pingUrl, pingKey)
                runOnUiThread { binding.pingResult.text = result }
            }.start()
        }

        // Press scale animation on primary buttons
        val pressAnim = AnimatorInflater.loadStateListAnimator(this, R.animator.btn_press)
        binding.btnConnect.stateListAnimator = pressAnim
        binding.btnImportConfig.stateListAnimator =
            AnimatorInflater.loadStateListAnimator(this, R.animator.btn_press)
        binding.btnInstallCA.stateListAnimator =
            AnimatorInflater.loadStateListAnimator(this, R.animator.btn_press)

        if (Mobile.isRunning()) {
            activeUrl = prefs.getString("url", null)
            activeKey = prefs.getString("key", null)
        }
        // Restore direct mode state — persisted so reconnect uses the same mode.
        directOnlySelected = prefs.getBoolean("direct_only", activeUrl == null && activeKey == null)
        Mobile.setDirectEnabled(prefs.getBoolean("direct_enabled", directOnlySelected))
        updateUI(running = Mobile.isRunning())
        updateLanguageButton()
        updateThemeButton()
        updateDirectBtn()

        // Restore log cache after recreation (theme/language change)
        if (logCache.isNotEmpty() && Mobile.isRunning()) {
            binding.logOutput.text = logCache
            binding.logScroll.post { binding.logScroll.fullScroll(View.FOCUS_DOWN) }
        }

        // Entrance animation — only animate views that are actually visible
        val running = Mobile.isRunning()
        val panelViews = if (running)
            listOf<View?>(binding.statusCard, binding.logCard)
        else
            listOf<View?>(binding.statusCard, binding.bottomActions)
        panelViews.forEachIndexed { i, v ->
            v ?: return@forEachIndexed
            val anim = AnimationUtils.loadAnimation(this, R.anim.card_enter)
            anim.startOffset = i * 60L
            v.startAnimation(anim)
        }

        val savedTheme = prefs.getInt("theme_mode", AppCompatDelegate.MODE_NIGHT_FOLLOW_SYSTEM)
        if (AppCompatDelegate.getDefaultNightMode() != savedTheme) {
            AppCompatDelegate.setDefaultNightMode(savedTheme)
        }
    }

    override fun onResume() {
        super.onResume()
        registerReceiver(startedReceiver, IntentFilter("com.zyrln.relay.STARTED"), RECEIVER_NOT_EXPORTED)
        registerReceiver(stopReceiver, IntentFilter("com.zyrln.relay.STOPPED"), RECEIVER_NOT_EXPORTED)
        registerReceiver(errorReceiver, IntentFilter(RelayVpnService.ACTION_ERROR), RECEIVER_NOT_EXPORTED)
        startLogPolling()
        if (Mobile.isRunning() && activeUrl == null) {
            val savedUrl = prefs.getString("url", null)
            if (!savedUrl.isNullOrEmpty()) {
                activeUrl = savedUrl
                activeKey = prefs.getString("key", null)
            }
        }
        if (Mobile.isRunning()) resumeUptimeTicker() else stopUptimeTicker()
        updateUI(running = Mobile.isRunning())
    }

    override fun onPause() {
        super.onPause()
        unregisterReceiver(startedReceiver)
        unregisterReceiver(stopReceiver)
        unregisterReceiver(errorReceiver)
        stopLogPolling()
        uptimeHandler.removeCallbacks(uptimeTick)
    }

    override fun onDestroy() {
        colorAnimator?.cancel()
        stopLogPolling()
        uptimeHandler.removeCallbacks(uptimeTick)
        super.onDestroy()
    }

    private fun onConnectClicked() {
        val running = Mobile.isRunning()
        if (running) {
            playMotion(binding.btnConnect, R.anim.motion_soft)
            stopVpn()
            return
        }
        when {
            directOnlySelected -> connectDirect()
            selectedUrl != null -> connectConfig(selectedUrl!!, selectedKey ?: "")
            else -> { /* button should be disabled — no-op */ }
        }
    }

    private var connectBtnCurrentColor: Int = 0
    private var colorAnimator: ValueAnimator? = null
    private var uptimeStart: Long = 0L
    private val uptimeHandler = Handler(Looper.getMainLooper())
    private val uptimeTick = object : Runnable {
        override fun run() {
            val elapsed = SystemClock.elapsedRealtime() - uptimeStart
            val h = elapsed / 3_600_000
            val m = (elapsed % 3_600_000) / 60_000
            val s = (elapsed % 60_000) / 1_000
            binding.uptimeValue.text = "%02d:%02d:%02d".format(h, m, s)
            uptimeHandler.postDelayed(this, 1_000)
        }
    }

    private fun startUptimeTicker() {
        uptimeStart = SystemClock.elapsedRealtime()
        prefs.edit().putLong("uptime_start_wall", System.currentTimeMillis()).apply()
        uptimeHandler.removeCallbacks(uptimeTick)
        uptimeHandler.post(uptimeTick)
    }

    private fun resumeUptimeTicker() {
        val wallStart = prefs.getLong("uptime_start_wall", 0L)
        if (wallStart == 0L) {
            startUptimeTicker()
            return
        }
        val elapsedSinceStart = System.currentTimeMillis() - wallStart
        uptimeStart = SystemClock.elapsedRealtime() - elapsedSinceStart
        uptimeHandler.removeCallbacks(uptimeTick)
        uptimeHandler.post(uptimeTick)
    }

    private fun stopUptimeTicker() {
        uptimeHandler.removeCallbacks(uptimeTick)
        prefs.edit().remove("uptime_start_wall").apply()
        binding.uptimeValue.text = "00:00:00"
    }

    private fun updateUI(running: Boolean) {
        runOnUiThread {
            binding.statusText.setText(if (running) R.string.status_running else R.string.status_stopped)
            binding.btnConnect.setImageResource(if (running) R.drawable.ic_pause else R.drawable.ic_connect)

            val targetColor = ContextCompat.getColor(
                this, if (running) R.color.accent_error else R.color.accent_success
            )
            if (connectBtnCurrentColor == 0) {
                connectBtnCurrentColor = targetColor
                binding.btnConnect.imageTintList = ColorStateList.valueOf(targetColor)
            } else if (connectBtnCurrentColor != targetColor) {
                colorAnimator?.cancel()
                colorAnimator = ValueAnimator.ofObject(ArgbEvaluator(), connectBtnCurrentColor, targetColor).apply {
                    duration = 220
                    addUpdateListener { animator ->
                        val color = animator.animatedValue as Int
                        binding.btnConnect.imageTintList = ColorStateList.valueOf(color)
                    }
                    start()
                }
                connectBtnCurrentColor = targetColor
            }

            binding.btnImportConfig.isEnabled = !running
            binding.btnInstallCA.isEnabled = !running
            binding.btnImportConfig.alpha = if (running) 0.5f else 1f
            binding.btnInstallCA.alpha = if (running) 0.5f else 1f
            binding.logCard.visibility = if (running) View.VISIBLE else View.GONE
            binding.bottomActions.visibility = if (running) View.GONE else View.VISIBLE
            if (!running) { binding.logOutput.text = ""; logCache.clear() }
            refreshList(running)
            updateDirectBtn()
        }
    }

    private fun toggleLanguage() {
        val currentLocale = AppCompatDelegate.getApplicationLocales()[0]?.language ?: Locale.getDefault().language
        val newLocale = if (currentLocale == "fa") "en" else "fa"
        AppCompatDelegate.setApplicationLocales(LocaleListCompat.forLanguageTags(newLocale))
    }

    private fun toggleTheme() {
        val isNight = (resources.configuration.uiMode and android.content.res.Configuration.UI_MODE_NIGHT_MASK) ==
                android.content.res.Configuration.UI_MODE_NIGHT_YES
        val nextMode = if (isNight) AppCompatDelegate.MODE_NIGHT_NO else AppCompatDelegate.MODE_NIGHT_YES
        AppCompatDelegate.setDefaultNightMode(nextMode)
        prefs.edit().putInt("theme_mode", nextMode).apply()
        updateThemeButton()
    }

    private fun updateLanguageButton() {
        val currentLocale = AppCompatDelegate.getApplicationLocales()[0]?.language ?: Locale.getDefault().language
        binding.btnLanguage.text = if (currentLocale == "fa") "EN" else "FA"
    }

    private fun updateThemeButton() {
        val isNight = (resources.configuration.uiMode and android.content.res.Configuration.UI_MODE_NIGHT_MASK) ==
                android.content.res.Configuration.UI_MODE_NIGHT_YES
        binding.btnTheme.setImageResource(if (isNight) R.drawable.ic_sun else R.drawable.ic_moon)
        binding.btnTheme.imageTintList = ContextCompat.getColorStateList(this, R.color.icon)
    }

    private fun refreshList(running: Boolean = Mobile.isRunning()) {
        val configs = loadConfigs()
        binding.configList.removeAllViews()

        // Connect is always enabled — direct mode works without any config
        binding.btnConnect.visibility = View.VISIBLE
        binding.btnConnect.isEnabled = true
        binding.btnConnect.alpha = 1f

        if (running) {
            binding.configScroll.visibility = View.GONE
            binding.emptyState.visibility = View.GONE
            return
        }

        binding.configScroll.visibility = View.VISIBLE
        binding.emptyState.visibility = View.GONE

        val dp = resources.displayMetrics.density

        // Auto-select direct card if nothing else is selected
        if (!directOnlySelected && selectedUrl == null && activeUrl == null) {
            directOnlySelected = true
        }

        // Direct-only card — always shown at the top
        run {
            val isSelected = directOnlySelected
            val card = CardView(this).apply {
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { bottomMargin = (10 * dp).toInt() }
                radius = 14 * dp
                cardElevation = 0f
                setCardBackgroundColor(ContextCompat.getColor(this@MainActivity, R.color.card_bg))
            }
            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                layoutDirection = View.LAYOUT_DIRECTION_LTR
                val p = (14 * dp).toInt()
                setPadding(p, p, p, p)
                background = ContextCompat.getDrawable(this@MainActivity,
                    if (isSelected) R.drawable.bg_card_selected else R.drawable.bg_card)
            }
            val label = TextView(this).apply {
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
                text = "⚡ ${getString(R.string.direct_only_label)}"
                textSize = 15f
                setTextColor(ContextCompat.getColor(this@MainActivity, R.color.title))
                if (isSelected) setTypeface(null, android.graphics.Typeface.BOLD)
            }
            row.addView(label)
            card.addView(row)
            row.setOnClickListener {
                directOnlySelected = true
                selectedUrl = null
                selectedKey = null
                refreshList(running = false)
                updateDirectBtn()
            }
            binding.configList.addView(card)
        }

        val iconButtonSize = (40 * dp).toInt()
        val iconButtonPadding = (8 * dp).toInt()
        val hostnames = configs.map { (url, _) -> configLabel(url) }

        configs.forEach { (url, key) ->
            val isActive = url == activeUrl && key == activeKey
            val isSelected = !isActive && url == selectedUrl && key == selectedKey

            val card = CardView(this).apply {
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { bottomMargin = (10 * dp).toInt() }
                radius = 14 * dp
                cardElevation = 0f
                setCardBackgroundColor(ContextCompat.getColor(this@MainActivity, R.color.card_bg))
            }

            val rowBg = when {
                isActive -> R.drawable.bg_card_active
                isSelected -> R.drawable.bg_card_selected
                else -> R.drawable.bg_card
            }
            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                layoutDirection = View.LAYOUT_DIRECTION_LTR
                val p = (14 * dp).toInt()
                setPadding(p, p, p, p)
                background = ContextCompat.getDrawable(this@MainActivity, rowBg)
            }

            val baseLabel = configLabel(url)
            val displayLabel = if (hostnames.count { it == baseLabel } > 1)
                "$baseLabel …${key.takeLast(4)}" else baseLabel

            val label = TextView(this).apply {
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
                text = displayLabel
                textSize = 15f
                maxLines = 1
                ellipsize = android.text.TextUtils.TruncateAt.END
                textAlignment = View.TEXT_ALIGNMENT_VIEW_START
                textDirection = View.TEXT_DIRECTION_LTR
                setTextColor(ContextCompat.getColor(this@MainActivity, R.color.title))
                if (isActive || isSelected) setTypeface(null, Typeface.BOLD)
            }

            val urlList = url.split(",").map { it.trim() }.filter { it.isNotEmpty() }
            val infoBtn = android.widget.ImageButton(this).apply {
                visibility = if (urlList.size > 1) View.VISIBLE else View.GONE
                setImageDrawable(ContextCompat.getDrawable(this@MainActivity, R.drawable.ic_info))
                background = null
                imageTintList = ContextCompat.getColorStateList(this@MainActivity, R.color.text_dim)
                scaleType = android.widget.ImageView.ScaleType.CENTER
                setPadding(iconButtonPadding, iconButtonPadding, iconButtonPadding, iconButtonPadding)
                layoutParams = LinearLayout.LayoutParams(iconButtonSize, iconButtonSize)
                    .apply { marginEnd = (4 * dp).toInt() }
            }
            infoBtn.setOnClickListener {
                val lines = urlList.mapIndexed { i, u ->
                    val id = u.substringAfter("/macros/s/", "").substringBefore("/")
                    val short = if (id.length >= 6) "…${id.takeLast(10)}" else u.substringAfter("://").substringBefore("/")
                    "${i + 1}. $short"
                }.joinToString("\n")
                AlertDialog.Builder(this@MainActivity, R.style.Dialog_Zyrln)
                    .setTitle(R.string.btn_ok)
                    .setMessage(lines)
                    .setPositiveButton(R.string.btn_ok, null)
                    .show()
            }

            val deleteBtn = android.widget.ImageButton(this).apply {
                layoutParams = LinearLayout.LayoutParams(iconButtonSize, iconButtonSize)
                    .apply { marginStart = (6 * dp).toInt() }
                setImageDrawable(ContextCompat.getDrawable(this@MainActivity, R.drawable.ic_delete))
                background = null
                imageTintList = ContextCompat.getColorStateList(this@MainActivity, R.color.text_dim)
                scaleType = android.widget.ImageView.ScaleType.CENTER
                setPadding(iconButtonPadding, iconButtonPadding, iconButtonPadding, iconButtonPadding)
            }

            row.addView(label)
            row.addView(infoBtn)
            row.addView(deleteBtn)
            card.addView(row)

            card.setOnClickListener {
                if (isActive && running) {
                    stopVpn()
                } else if (!running) {
                    // Select this row; deselect if already selected
                    if (isSelected) {
                        selectedUrl = null
                        selectedKey = null
                    } else {
                        selectedUrl = url
                        selectedKey = key
                        directOnlySelected = false
                        Mobile.setDirectEnabled(false)
                        prefs.edit().putBoolean("direct_only", false).putBoolean("direct_enabled", false).apply()
                    }
                    refreshList(running = false)
                    updateDirectBtn()
                }
            }

            deleteBtn.setOnClickListener {
                AlertDialog.Builder(this, R.style.Dialog_Zyrln_Destructive)
                    .setTitle(R.string.dialog_remove_title)
                    .setMessage(if (isActive && running)
                        getString(R.string.dialog_remove_active, displayLabel)
                    else
                        getString(R.string.dialog_remove_inactive, displayLabel))
                    .setPositiveButton(R.string.btn_remove) { _, _ ->
                        val deleteAnim = AnimationUtils.loadAnimation(this, R.anim.motion_delete)
                        deleteAnim.setAnimationListener(object : Animation.AnimationListener {
                            override fun onAnimationStart(a: Animation?) {}
                            override fun onAnimationRepeat(a: Animation?) {}
                            override fun onAnimationEnd(a: Animation?) {
                                if (isActive && running) stopVpn()
                                deleteConfig(url, key)
                                if (selectedUrl == url && selectedKey == key) {
                                    selectedUrl = null
                                    selectedKey = null
                                }
                                refreshList(running = Mobile.isRunning())
                            }
                        })
                        card.startAnimation(deleteAnim)
                    }
                    .setNegativeButton(R.string.btn_cancel, null)
                    .show()
            }

            binding.configList.addView(card)

            // Staggered entrance: each card slides up with a small delay
            val enterAnim = AnimationUtils.loadAnimation(this, R.anim.card_enter)
            val configIndex = configs.indexOf(Pair(url, key))
            enterAnim.startOffset = (configIndex * 40L).coerceAtMost(120L)
            card.startAnimation(enterAnim)
        }
    }

    private fun connectConfig(url: String, key: String) {
        directOnlySelected = false
        prefs.edit().putBoolean("direct_only", false).apply()
        if (!hasInstalledCA()) {
            activeUrl = null
            activeKey = null
            updateUI(running = false)
            Toast.makeText(this, R.string.error_ca_required, Toast.LENGTH_LONG).show()
            return
        }
        activeUrl = url
        activeKey = key
        selectedUrl = null
        selectedKey = null
        prefs.edit().putString("url", url).putString("key", key).apply()
        refreshList(running = false)
        val vpnIntent = VpnService.prepare(this)
        if (vpnIntent != null) vpnPermissionLauncher.launch(vpnIntent) else launchVpnService()
    }

    private fun connectDirect() {
        Mobile.setDirectEnabled(true)
        directOnlySelected = true
        prefs.edit().putString("url", "").putString("key", "")
            .putBoolean("direct_only", true).putBoolean("direct_enabled", true).apply()
        updateDirectBtn()
        val vpnIntent = VpnService.prepare(this)
        if (vpnIntent != null) vpnPermissionLauncher.launch(vpnIntent) else launchVpnService()
    }

    private fun hasInstalledCA(): Boolean {
        val certDir = File(filesDir, "certs")
        return File(certDir, "ca.pem").exists() && File(certDir, "ca.key").exists()
    }

    private fun appendLog(level: String, msg: String) {
        if (level == "debug") return
        runOnUiThread {
            val time = java.text.SimpleDateFormat("HH:mm:ss", java.util.Locale.US)
                .format(java.util.Date())
            val color = when (level) {
                "error" -> ContextCompat.getColor(this, R.color.accent_error)
                "system" -> ContextCompat.getColor(this, R.color.primary)
                else -> ContextCompat.getColor(this, R.color.text_dim)
            }
            val line = "[$time] $msg\n"
            val spannable = android.text.SpannableString(line)
            spannable.setSpan(
                android.text.style.ForegroundColorSpan(color),
                0, line.length,
                android.text.Spannable.SPAN_EXCLUSIVE_EXCLUSIVE
            )
            logCache.append(spannable)
            binding.logOutput.append(spannable)
            // Count newlines; if over 200 lines trim from the front until 200 remain
            var newlines = 0
            for (i in logCache.indices) if (logCache[i] == '\n') newlines++
            if (newlines > 200) {
                var toRemove = newlines - 200
                var pos = 0
                while (toRemove > 0 && pos < logCache.length) {
                    if (logCache[pos] == '\n') toRemove--
                    pos++
                }
                logCache.delete(0, pos)
                binding.logOutput.text = logCache
            }
            binding.logScroll.post { binding.logScroll.fullScroll(android.view.View.FOCUS_DOWN) }
        }
    }

    private fun launchVpnService() {
        val url = prefs.getString("url", "") ?: ""
        val key = prefs.getString("key", "") ?: ""
        playMotion(binding.btnConnect, R.anim.motion_confirm)
        startUptimeTicker()
        startLogPolling()
        updateUI(running = true)
        ContextCompat.startForegroundService(this, Intent(this, RelayVpnService::class.java).apply {
            action = RelayVpnService.ACTION_START
            putExtra(RelayVpnService.EXTRA_URL, url)
            putExtra(RelayVpnService.EXTRA_KEY, key)
        })
    }

    private fun stopVpn() {
        startService(Intent(this, RelayVpnService::class.java).apply { action = RelayVpnService.ACTION_STOP })
        activeUrl = null
        activeKey = null
        stopUptimeTicker()
        updateUI(running = false)
    }

    private fun importConfig() {
        val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        val rawText = clipboard.primaryClip?.getItemAt(0)?.getText()?.toString()?.trim()
        if (rawText.isNullOrEmpty()) {
            Toast.makeText(this, R.string.msg_clipboard_empty, Toast.LENGTH_SHORT).show()
            return
        }
        try {
            val (url, key) = ConfigUtils.parseImportText(rawText)
            if (saveConfig(url, key)) {
                refreshList()
                playMotion(binding.btnImportConfig, R.anim.motion_confirm)
                Toast.makeText(this, R.string.msg_config_saved_connect, Toast.LENGTH_SHORT).show()
            } else {
                Toast.makeText(this, R.string.msg_already_exists, Toast.LENGTH_SHORT).show()
            }
        } catch (e: Exception) {
            Log.e("MainActivity", "Import failed: ${e.message}")
            Toast.makeText(this, getString(R.string.msg_invalid_config), Toast.LENGTH_LONG).show()
        }
    }

    private fun loadConfigs(): List<Pair<String, String>> {
        val raw = prefs.getString("configs", "[]") ?: "[]"
        return try {
            val arr = JSONArray(raw)
            (0 until arr.length()).map { i ->
                val o = arr.getJSONObject(i)
                Pair(o.getString("url"), o.getString("key"))
            }
        } catch (e: JSONException) { emptyList() }
    }

    private fun saveConfig(url: String, key: String): Boolean {
        val existing = loadConfigs()
        if (existing.any { it.first == url && it.second == key }) return false
        val arr = JSONArray()
        existing.forEach { (u, k) -> arr.put(JSONObject().put("url", u).put("key", k)) }
        arr.put(JSONObject().put("url", url).put("key", key))
        prefs.edit().putString("configs", arr.toString()).apply()
        return true
    }

    private fun deleteConfig(url: String, key: String) {
        val arr = JSONArray()
        loadConfigs().filter { it.first != url || it.second != key }
            .forEach { (u, k) -> arr.put(JSONObject().put("url", u).put("key", k)) }
        prefs.edit().putString("configs", arr.toString()).apply()
    }

    private fun playMotion(v: View?, animRes: Int) {
        v ?: return
        val anim = AnimationUtils.loadAnimation(this, animRes)
        v.startAnimation(anim)
    }

    private fun configLabel(url: String) = ConfigUtils.configLabel(url)

    private fun installCACert() {
        val certDir = File(filesDir, "certs")
        certDir.mkdirs()
        val certFile = File(certDir, "ca.pem")
        val keyFile = File(certDir, "ca.key")
        val wasGenerated = prefs.getBoolean("ca_generated", false)
        if (!certFile.exists() || !keyFile.exists()) {
            val err = Mobile.generateCA(certFile.absolutePath, keyFile.absolutePath)
            if (err.isNotEmpty()) {
                Toast.makeText(this, "CA generation failed: $err", Toast.LENGTH_LONG).show()
                return
            }
            prefs.edit().putBoolean("ca_generated", true).apply()
            // If a CA was previously generated but files are gone (reinstall),
            // warn the user that the old trusted cert is now invalid.
            if (wasGenerated) {
                AlertDialog.Builder(this)
                    .setTitle(getString(R.string.dialog_ca_title))
                    .setMessage(getString(R.string.dialog_ca_reinstall_message))
                    .setPositiveButton(android.R.string.ok) { _, _ ->
                        createDocumentLauncher.launch("zyrln-ca.pem")
                    }
                    .show()
                return
            }
        }
        createDocumentLauncher.launch("zyrln-ca.pem")
    }

    private fun shareLog() {
        // GetAllLogs includes debug-level entries (race probe failures etc.) that
        // are filtered from the display — important for diagnosing connectivity issues.
        val raw = Mobile.getAllLogs().trim()
        if (raw.isEmpty()) {
            Toast.makeText(this, R.string.log_empty, Toast.LENGTH_SHORT).show()
            return
        }
        val lines = raw.split('\n').joinToString("\n") { line ->
            val tab = line.indexOf('\t')
            if (tab >= 0) "[${line.substring(0, tab)}] ${line.substring(tab + 1)}" else line
        }
        val header = buildString {
            appendLine("Zyrln Android v${BuildConfig.VERSION_NAME} (${BuildConfig.VERSION_CODE})")
            appendLine("Device: ${android.os.Build.MANUFACTURER} ${android.os.Build.MODEL}")
            appendLine("Android: ${android.os.Build.VERSION.RELEASE} (SDK ${android.os.Build.VERSION.SDK_INT})")
            appendLine("Time: ${java.text.SimpleDateFormat("yyyy-MM-dd HH:mm:ss z", java.util.Locale.US).format(java.util.Date())}")
            appendLine("---")
        }
        val intent = Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            putExtra(Intent.EXTRA_SUBJECT, getString(R.string.log_share_subject))
            putExtra(Intent.EXTRA_TEXT, header + lines)
        }
        startActivity(Intent.createChooser(intent, getString(R.string.log_share_subject)))
    }

    private fun saveCertToUri(uri: Uri) {
        val certFile = File(File(filesDir, "certs"), "ca.pem")
        try {
            contentResolver.openOutputStream(uri)?.use { output ->
                certFile.inputStream().use { input -> input.copyTo(output) }
            }
            playMotion(binding.btnInstallCA, R.anim.motion_confirm)
            Toast.makeText(this, R.string.msg_cert_saved_success, Toast.LENGTH_SHORT).show()
            AlertDialog.Builder(this, R.style.Dialog_Zyrln)
                .setTitle(R.string.dialog_ca_title)
                .setMessage(R.string.dialog_ca_message_generic)
                .setPositiveButton(R.string.btn_open_settings) { _, _ ->
                    startActivity(Intent(Settings.ACTION_SECURITY_SETTINGS))
                }
                .setNegativeButton(R.string.btn_later, null)
                .show()
        } catch (e: Exception) {
            Log.e("MainActivity", "Failed to save cert: ${e.message}")
            Toast.makeText(this, "Save failed: ${e.message}", Toast.LENGTH_LONG).show()
        }
    }

    private fun toggleDirectMode() {
        // Only block toggling off while running in direct-only mode (no relay active)
        if (Mobile.isRunning() && directOnlySelected) return
        val next = !Mobile.isDirectEnabled()
        Mobile.setDirectEnabled(next)
        prefs.edit().putBoolean("direct_enabled", next).apply()
        updateDirectBtn()
    }

    private fun updateDirectBtn() {
        val on = Mobile.isDirectEnabled() || directOnlySelected
        val color = ContextCompat.getColor(
            this,
            if (on) R.color.accent_success else R.color.text_dim
        )
        binding.btnDirect.imageTintList = ColorStateList.valueOf(color)
        binding.btnDirect.alpha = 1f
        binding.btnDirect.isEnabled = true
    }
}
