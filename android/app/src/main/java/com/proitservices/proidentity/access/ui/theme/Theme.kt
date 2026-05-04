package com.proitservices.proidentity.access.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

private val Background  = Color(0xFF0F1117)
private val Surface     = Color(0xFF1A1D27)
private val SurfaceVar  = Color(0xFF252836)
private val Primary     = Color(0xFF6366F1)   // indigo
private val OnPrimary   = Color(0xFFFFFFFF)
private val Secondary   = Color(0xFF10B981)   // emerald green â€“ connected
private val Error       = Color(0xFFEF4444)
private val OnSurface   = Color(0xFFE2E8F0)
private val OnSurfaceVar= Color(0xFF94A3B8)

private val DarkColors = darkColorScheme(
    primary          = Primary,
    onPrimary        = OnPrimary,
    secondary        = Secondary,
    background       = Background,
    surface          = Surface,
    surfaceVariant   = SurfaceVar,
    error            = Error,
    onBackground     = OnSurface,
    onSurface        = OnSurface,
    onSurfaceVariant = OnSurfaceVar,
)

@Composable
fun WgClientTheme(content: @Composable () -> Unit) {
    MaterialTheme(colorScheme = DarkColors, content = content)
}
