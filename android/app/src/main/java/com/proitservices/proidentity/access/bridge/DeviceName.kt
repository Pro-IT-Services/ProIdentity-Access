package com.proitservices.proidentity.access.bridge

import android.os.Build

fun defaultDeviceName(): String {
    val manufacturer = Build.MANUFACTURER?.trim().orEmpty()
    val model = Build.MODEL?.trim().orEmpty()
    val name = when {
        model.isEmpty() -> manufacturer
        manufacturer.isEmpty() -> model
        model.lowercase().startsWith(manufacturer.lowercase()) -> model
        else -> "$manufacturer $model"
    }.trim()
    return name.ifEmpty { "Android Device" }
}
