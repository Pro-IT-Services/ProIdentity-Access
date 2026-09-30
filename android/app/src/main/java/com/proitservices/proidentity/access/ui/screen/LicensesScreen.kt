package com.proitservices.proidentity.access.ui.screen

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.automirrored.outlined.OpenInNew
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.ui.design.InfoCard
import com.proitservices.proidentity.access.ui.design.SectionLabel

private const val SOURCE_CODE = "https://github.com/Pro-IT-Services/ProIdentity-Access"

/** A piece of software in the app and its license. */
private data class Component(
    val name: String,
    val detail: String,
    val license: String,
    /** File in assets/licenses/ (without .txt). */
    val file: String,
    /** Shown above a shared license text (Apache-2.0). */
    val copyright: String? = null,
)

private val components = listOf(
    Component(
        "WireGuard tunnel library", "com.wireguard.android:tunnel", "Apache-2.0", "apache-2.0",
        "Copyright © 2017-2025 WireGuard LLC. All Rights Reserved.",
    ),
    Component("wireguard-go", "WireGuard implementation", "MIT", "wireguard-go"),
    Component("Go", "Go runtime, standard library and golang.org/x", "BSD-3-Clause", "go"),
    Component("OkHttp and Okio", "HTTP client", "Apache-2.0", "apache-2.0", "Copyright 2019 Square, Inc."),
    Component("Bouncy Castle", "Cryptography provider", "MIT", "bouncycastle"),
    Component(
        "Kotlin and kotlinx.coroutines", "Language runtime", "Apache-2.0", "apache-2.0",
        "Copyright 2000-2025 JetBrains s.r.o. and Kotlin Programming Language contributors.",
    ),
    Component(
        "AndroidX and Jetpack Compose", "Including Security Crypto and Material icons", "Apache-2.0", "apache-2.0",
        "Copyright The Android Open Source Project.",
    ),
    Component("Tink and Gson", "Google libraries used by Security Crypto", "Apache-2.0", "apache-2.0", "Copyright Google LLC."),
)

private val appComponent = Component("ProIdentity Access", "This app", "Free Internal Use License 1.0", "proidentity-access")

/**
 * Third-party software in the app with the notices its licenses (MIT, BSD,
 * Apache-2.0) require to ship with it, plus the app's own license.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LicensesScreen(onBack: () -> Unit) {
    var open by rememberSaveable { mutableStateOf<String?>(null) }
    val shown = (components + appComponent).firstOrNull { it.name == open }
    BackHandler(enabled = shown != null) { open = null }
    val uri = LocalUriHandler.current

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(shown?.name ?: "Open-source licenses") },
                navigationIcon = {
                    IconButton(onClick = { if (shown != null) open = null else onBack() }) {
                        Icon(Icons.AutoMirrored.Outlined.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
    ) { padding ->
        if (shown != null) {
            LicenseText(shown, Modifier.padding(padding))
            return@Scaffold
        }
        Column(
            Modifier
                .padding(padding)
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp)
                .navigationBarsPadding(),
        ) {
            SectionLabel("Included software")
            InfoCard {
                components.forEach { c -> ComponentRow(c) { open = c.name } }
            }
            Text(
                "WireGuard is a registered trademark of Jason A. Donenfeld.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
            )
            Spacer(Modifier.height(12.dp))
            SectionLabel("This app")
            InfoCard {
                ComponentRow(appComponent) { open = appComponent.name }
                ListItem(
                    headlineContent = { Text("Source code") },
                    trailingContent = { Icon(Icons.AutoMirrored.Outlined.OpenInNew, contentDescription = null) },
                    colors = ListItemDefaults.colors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
                    modifier = Modifier.clickable(role = Role.Button) { uri.openUri(SOURCE_CODE) },
                )
            }
            Spacer(Modifier.height(24.dp))
        }
    }
}

@Composable
private fun ComponentRow(c: Component, onClick: () -> Unit) {
    ListItem(
        headlineContent = { Text(c.name) },
        supportingContent = { Text("${c.detail} · ${c.license}") },
        colors = ListItemDefaults.colors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
        modifier = Modifier.clickable(role = Role.Button, onClick = onClick),
    )
}

@Composable
private fun LicenseText(c: Component, modifier: Modifier) {
    val context = LocalContext.current
    val text = remember(c.file) {
        runCatching {
            context.assets.open("licenses/${c.file}.txt").bufferedReader().use { it.readText() }
        }.getOrDefault("License text not found.")
    }
    SelectionContainer(
        modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(16.dp)
            .navigationBarsPadding(),
    ) {
        Text(
            (c.copyright?.let { "$it\n\n" } ?: "") + text,
            style = MaterialTheme.typography.bodySmall,
            fontFamily = FontFamily.Monospace,
        )
    }
}
