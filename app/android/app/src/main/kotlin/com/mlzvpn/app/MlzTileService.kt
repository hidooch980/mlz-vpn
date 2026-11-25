package com.mlzvpn.app

import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.service.quicksettings.Tile
import android.service.quicksettings.TileService

/**
 * Android Quick Settings tile allowing one-tap toggle directly from the notification shade.
 */
class MlzTileService : TileService() {

    override fun onStartListening() {
        super.onStartListening()
        qsTile?.apply {
            state = if (MlzVpnService.isRunning) Tile.STATE_ACTIVE else Tile.STATE_INACTIVE
            label = "MLZ VPN"
            updateTile()
        }
    }

    override fun onClick() {
        super.onClick()
        if (MlzVpnService.isRunning) {
            val stop = Intent(this, MlzVpnService::class.java).apply { action = MlzVpnService.ACTION_STOP }
            startService(stop)
            qsTile?.state = Tile.STATE_INACTIVE
            qsTile?.updateTile()
            return
        }

        if (VpnService.prepare(this) != null) {
            val openApp = Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            startActivityAndCollapse(openApp)
            return
        }

        val start = Intent(this, MlzVpnService::class.java).apply {
            action = MlzVpnService.ACTION_START
            putExtra("engine_mode", "auto")
            putExtra("country_code", "auto")
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(start)
        } else {
            startService(start)
        }
        qsTile?.state = Tile.STATE_ACTIVE
        qsTile?.updateTile()
    }
}
