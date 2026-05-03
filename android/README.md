# ProIdentity Access Android

This directory contains the Android client built with Kotlin, Jetpack Compose,
and the WireGuard Android tunnel library.

Public repository:

- HTTPS: https://github.com/Pro-IT-Services/ProIdentity-Access
- SSH: git@github.com:Pro-IT-Services/ProIdentity-Access.git

## Requirements

- Android Studio or Android SDK command line tools
- JDK 17
- Gradle
- Node.js and npm if rebuilding the embedded frontend assets

## Build

```sh
cd android
./build-frontend.sh
gradle assembleDebug
```

For release builds, configure signing in Android Studio or your Gradle
environment, then run:

```sh
gradle assembleRelease
```

The app version is configured in `app/build.gradle.kts`.

Do not commit `local.properties`, signing keys, keystores, generated APK/AAB
files, or IDE workspace state.

## License

ProIdentity Access is source-available under the PolyForm Noncommercial
License 1.0.0 for noncommercial use. Commercial, enterprise, MSP, resale,
hosted-service, or other revenue-generating use requires a separate written
commercial license from Pro-IT-Services. See the repository root `LICENSE`.
