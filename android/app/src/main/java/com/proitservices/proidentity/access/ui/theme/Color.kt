package com.proitservices.proidentity.access.ui.theme

import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.ui.graphics.Color

// Brand tokens ported from the desktop client (client/frontend/src/index.css).
// Dark is the brand showcase; light keeps the same hues at AA contrast.

// ── Dark ──────────────────────────────────────────────────────────────────
private val NavyBackground = Color(0xFF0F1115)   // hsl(222 18% 7%)
private val NavySurface = Color(0xFF15181E)      // hsl(222 16% 10%)
private val NavyRaised = Color(0xFF1F2229)       // hsl(222 14% 14%)
private val NavyHigh = Color(0xFF23272E)         // hsl(222 14% 16%)
private val NavyBorder = Color(0xFF282B34)       // hsl(222 14% 18%)
private val Ink = Color(0xFFF3F5F7)              // hsl(210 20% 96%)
private val InkMuted = Color(0xFF8B97A8)         // hsl(215 14% 56%), lifted for AA
private val BrandBlue = Color(0xFF3B82F6)        // hsl(217 91% 60%)
private val BrandBlueInk = Color(0xFF0F1729)     // hsl(222 47% 11%) — text on blue

// ── Light ─────────────────────────────────────────────────────────────────
private val PaperBackground = Color(0xFFF6F7F9)
private val PaperSurface = Color(0xFFFFFFFF)
private val PaperRaised = Color(0xFFEEF1F5)
private val PaperBorder = Color(0xFFD9DEE6)
private val InkNavy = Color(0xFF0F1729)
private val InkSlate = Color(0xFF4A5568)          // 7.5:1 on white
private val BrandBlueDeep = Color(0xFF2563EB)     // 5.2:1 on white (blue-500 is only 3.7:1)

val DarkColors = darkColorScheme(
    primary = BrandBlue,
    onPrimary = BrandBlueInk,
    primaryContainer = Color(0xFF16305A),
    onPrimaryContainer = Color(0xFFDBEAFE),
    secondary = Color(0xFF94A3B8),
    onSecondary = BrandBlueInk,
    secondaryContainer = NavyHigh,
    onSecondaryContainer = Ink,
    tertiary = Color(0xFF22C55E),
    onTertiary = Color(0xFF052E16),
    tertiaryContainer = Color(0xFF14532D),
    onTertiaryContainer = Color(0xFFDCFCE7),
    background = NavyBackground,
    onBackground = Ink,
    surface = NavyBackground,
    onSurface = Ink,
    surfaceVariant = NavyRaised,
    onSurfaceVariant = InkMuted,
    surfaceContainerLowest = Color(0xFF0B0D10),
    surfaceContainerLow = Color(0xFF12151A),
    surfaceContainer = NavySurface,
    surfaceContainerHigh = NavyRaised,
    surfaceContainerHighest = NavyHigh,
    outline = Color(0xFF3A3F4A),
    outlineVariant = NavyBorder,
    error = Color(0xFFF87171),
    onError = Color(0xFF450A0A),
    errorContainer = Color(0xFF5B1717),
    onErrorContainer = Color(0xFFFEE2E2),
    inverseSurface = Ink,
    inverseOnSurface = NavyBackground,
    inversePrimary = BrandBlueDeep,
    scrim = Color(0xFF000000),
)

val LightColors = lightColorScheme(
    primary = BrandBlueDeep,
    onPrimary = Color.White,
    primaryContainer = Color(0xFFDBEAFE),
    onPrimaryContainer = Color(0xFF172554),
    secondary = Color(0xFF475569),
    onSecondary = Color.White,
    secondaryContainer = PaperRaised,
    onSecondaryContainer = InkNavy,
    tertiary = Color(0xFF15803D),
    onTertiary = Color.White,
    tertiaryContainer = Color(0xFFDCFCE7),
    onTertiaryContainer = Color(0xFF052E16),
    background = PaperBackground,
    onBackground = InkNavy,
    surface = PaperBackground,
    onSurface = InkNavy,
    surfaceVariant = PaperRaised,
    onSurfaceVariant = InkSlate,
    surfaceContainerLowest = Color.White,
    surfaceContainerLow = Color(0xFFFBFCFD),
    surfaceContainer = PaperSurface,
    surfaceContainerHigh = PaperRaised,
    surfaceContainerHighest = Color(0xFFE4E8EE),
    outline = Color(0xFFB8C0CC),
    outlineVariant = PaperBorder,
    error = Color(0xFFB91C1C),
    onError = Color.White,
    errorContainer = Color(0xFFFEE2E2),
    onErrorContainer = Color(0xFF450A0A),
    inverseSurface = InkNavy,
    inverseOnSurface = PaperBackground,
    inversePrimary = BrandBlue,
    scrim = Color(0xFF000000),
)

/** Semantic colors Material 3 has no slot for. */
data class StatusColors(
    val connected: Color,
    val onConnected: Color,
    val connectedContainer: Color,
    val warning: Color,
    val warningContainer: Color,
    /** Brand orange — reserved for Disconnect, matching the desktop client. */
    val disconnect: Color,
    val onDisconnect: Color,
    val disconnectContainer: Color,
)

val DarkStatus = StatusColors(
    connected = Color(0xFF22C55E),
    onConnected = Color(0xFF052E16),
    connectedContainer = Color(0xFF113A22),
    warning = Color(0xFFF59E0B),
    warningContainer = Color(0xFF3B2A08),
    disconnect = Color(0xFFF97316),
    onDisconnect = Color(0xFF1C0A02),
    disconnectContainer = Color(0xFF3A1D0B),
)

val LightStatus = StatusColors(
    connected = Color(0xFF15803D),
    onConnected = Color.White,
    connectedContainer = Color(0xFFDCFCE7),
    warning = Color(0xFFB45309),
    warningContainer = Color(0xFFFEF3C7),
    disconnect = Color(0xFFC2410C),
    onDisconnect = Color.White,
    disconnectContainer = Color(0xFFFFEDD5),
)
