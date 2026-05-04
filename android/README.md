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

ProIdentity Access is free for personal, internal, and company use under the
ProIdentity Access Free Internal Use License 1.0. Redistribution, resale,
hosted-service/MSP/provider use, white-labeling, and sharing modified builds
require prior written permission from Pro-IT-Services. See the repository root
`LICENSE`.
