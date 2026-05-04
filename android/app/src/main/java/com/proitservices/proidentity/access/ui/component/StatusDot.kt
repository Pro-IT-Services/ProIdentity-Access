package com.proitservices.proidentity.access.ui.component

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.model.TunnelStatus

@Composable
fun StatusDot(status: TunnelStatus, modifier: Modifier = Modifier) {
    val color = when (status) {
        TunnelStatus.CONNECTED    -> MaterialTheme.colorScheme.secondary
        TunnelStatus.CONNECTING   -> MaterialTheme.colorScheme.primary
        TunnelStatus.ERROR        -> MaterialTheme.colorScheme.error
        TunnelStatus.DISCONNECTED -> MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.4f)
    }
    if (status == TunnelStatus.CONNECTING) {
        CircularProgressIndicator(modifier = modifier, strokeWidth = 1.5.dp, color = color)
    } else {
        Box(modifier = modifier.clip(CircleShape).background(color))
    }
}
