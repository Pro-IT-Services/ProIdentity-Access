package com.proitservices.proidentity.access.ui.design

import android.provider.Settings
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.path
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.ui.theme.Brand

// Geometry of the ProIdentity Access aperture mark (256-unit viewBox),
// identical to client/frontend/src/components/brand/LogoMark.tsx.
private val Hexagon = listOf(
    Offset(178f, 128f), Offset(153f, 171.3f), Offset(103f, 171.3f),
    Offset(78f, 128f), Offset(103f, 84.7f), Offset(153f, 84.7f),
)
private val BladeEnds = listOf(
    Offset(192.2f, 199.3f), Offset(98.3f, 219.3f), Offset(34.1f, 148.0f),
    Offset(63.8f, 56.7f), Offset(157.7f, 36.7f), Offset(221.9f, 108.0f),
)
private val Center = Offset(128f, 128f)

/**
 * Static brand mark for app bars and headers. Stroke weight is heavier than the
 * large-format mark so it stays legible at 20–28dp; tint it via Icon(tint = …).
 */
val ApertureMark: ImageVector by lazy {
    ImageVector.Builder(
        name = "ApertureMark",
        defaultWidth = 24.dp,
        defaultHeight = 24.dp,
        viewportWidth = 256f,
        viewportHeight = 256f,
    ).apply {
        path(
            stroke = SolidColor(Color.Black),
            strokeLineWidth = 15f,
            strokeLineCap = StrokeCap.Round,
            strokeLineJoin = StrokeJoin.Round,
        ) {
            moveTo(Hexagon[0].x, Hexagon[0].y)
            Hexagon.drop(1).forEach { lineTo(it.x, it.y) }
            close()
        }
        path(
            stroke = SolidColor(Color.Black),
            strokeLineWidth = 12f,
            strokeLineCap = StrokeCap.Round,
        ) {
            Hexagon.zip(BladeEnds).forEach { (from, to) ->
                moveTo(from.x, from.y)
                lineTo(to.x, to.y)
            }
        }
        path(fill = SolidColor(Color.Black)) {
            moveTo(Center.x - 14f, Center.y)
            arcToRelative(14f, 14f, 0f, true, true, 28f, 0f)
            arcToRelative(14f, 14f, 0f, true, true, -28f, 0f)
            close()
        }
    }.build()
}

enum class ApertureState { Idle, Connecting, Connected, Error }

@Composable
private fun animationsEnabled(): Boolean {
    val context = LocalContext.current
    return remember {
        Settings.Global.getFloat(context.contentResolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f) != 0f
    }
}

/**
 * The connection indicator: the brand aperture, alive.
 *  - Idle: muted outline.
 *  - Connecting: the aperture turns and an arc sweeps the perimeter.
 *  - Connected: filled in the connected color with a soft glow.
 *  - Error: warning color.
 * Only transform/alpha animate; everything is static when the system has
 * animations turned off (Settings > Accessibility > Remove animations).
 */
@Composable
fun StatusAperture(
    state: ApertureState,
    modifier: Modifier = Modifier,
    size: Dp = 168.dp,
) {
    val status = Brand.status
    val target = when (state) {
        ApertureState.Idle -> MaterialTheme.colorScheme.onSurfaceVariant
        ApertureState.Connecting -> MaterialTheme.colorScheme.primary
        ApertureState.Connected -> status.connected
        ApertureState.Error -> status.warning
    }
    val tint by animateColorAsState(target, tween(300), label = "apertureTint")
    val ringTrack = MaterialTheme.colorScheme.outlineVariant

    val animate = animationsEnabled() && state == ApertureState.Connecting
    val transition = rememberInfiniteTransition(label = "aperture")
    val spin by if (animate) transition.animateFloat(
        0f, 360f, infiniteRepeatable(tween(4200, easing = LinearEasing)), label = "spin",
    ) else remember { androidx.compose.runtime.mutableFloatStateOf(0f) }
    val sweep by if (animate) transition.animateFloat(
        0f, 360f, infiniteRepeatable(tween(1400, easing = LinearEasing)), label = "sweep",
    ) else remember { androidx.compose.runtime.mutableFloatStateOf(0f) }
    val pulse by if (animate) transition.animateFloat(
        0.55f, 1f, infiniteRepeatable(tween(900), RepeatMode.Reverse), label = "pulse",
    ) else remember { androidx.compose.runtime.mutableFloatStateOf(1f) }

    val description = when (state) {
        ApertureState.Idle -> "Not connected"
        ApertureState.Connecting -> "Connecting"
        ApertureState.Connected -> "Connected"
        ApertureState.Error -> "Connection error"
    }

    Canvas(
        modifier
            .size(size)
            .semantics { contentDescription = "Connection status: $description" },
    ) {
        val unit = this.size.minDimension / 256f
        scale(unit, pivot = Offset.Zero) {
            if (state == ApertureState.Connected) {
                drawCircle(
                    brush = Brush.radialGradient(
                        listOf(tint.copy(alpha = 0.30f), Color.Transparent),
                        center = Center, radius = 128f,
                    ),
                    radius = 128f, center = Center,
                )
            }
            // Perimeter ring + the connecting sweep.
            drawCircle(ringTrack, radius = 104f, center = Center, style = Stroke(width = 3f))
            if (state == ApertureState.Connecting) {
                drawArc(
                    color = tint,
                    startAngle = sweep - 90f, sweepAngle = 70f, useCenter = false,
                    topLeft = Offset(Center.x - 104f, Center.y - 104f), size = Size(208f, 208f),
                    style = Stroke(width = 6f, cap = StrokeCap.Round),
                )
            }
            rotate(spin, pivot = Center) { drawMark(state, tint, pulse) }
        }
    }
}

private fun DrawScope.drawMark(state: ApertureState, tint: Color, pulse: Float) {
    val hex = Path().apply {
        moveTo(Hexagon[0].x, Hexagon[0].y)
        Hexagon.drop(1).forEach { lineTo(it.x, it.y) }
        close()
    }
    if (state == ApertureState.Connected) drawPath(hex, tint.copy(alpha = 0.16f))
    val hexAlpha = if (state == ApertureState.Connecting) pulse else 1f
    drawPath(
        hex, tint.copy(alpha = hexAlpha),
        style = Stroke(width = 9f, cap = StrokeCap.Round, join = StrokeJoin.Round),
    )
    Hexagon.zip(BladeEnds).forEach { (from, to) ->
        drawLine(tint, from, to, strokeWidth = 7f, cap = StrokeCap.Round)
    }
    drawCircle(tint, radius = 12f, center = Center)
}
