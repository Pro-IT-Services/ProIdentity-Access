-keep class com.wireguard.** { *; }
-keep class org.bouncycastle.** { *; }
-dontwarn org.bouncycastle.**

# Error Prone annotations are compile-time only, not needed at runtime
-dontwarn com.google.errorprone.annotations.**
