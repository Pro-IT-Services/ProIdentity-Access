#!/usr/bin/env python3
import uuid, os

def uid(): return uuid.uuid4().hex[:24].upper()

sources = [
    "ProIdentityApp.swift",
    "Bridge/AppSettings.swift",
    "Bridge/DeviceCrypto.swift",
    "Bridge/ManagedClient.swift",
    "VPN/WireGuardConfig.swift",
    "VPN/VPNManager.swift",
    "Views/UI.swift",
    "Views/SetupView.swift",
    "Views/ServerListView.swift",
    "Views/TunnelListView.swift",
    "Views/SettingsView.swift",
]

PROJECT_ID = uid(); TARGET_ID = uid(); MAIN_GROUP_ID = uid()
PRODUCTS_GROUP_ID = uid(); BUILD_CONFIG_LIST_PROJECT = uid()
BUILD_CONFIG_LIST_TARGET = uid(); DEBUG_CONFIG_PROJECT = uid()
RELEASE_CONFIG_PROJECT = uid(); DEBUG_CONFIG_TARGET = uid()
RELEASE_CONFIG_TARGET = uid(); SOURCES_PHASE_ID = uid()
FRAMEWORKS_PHASE_ID = uid(); RESOURCES_PHASE_ID = uid()
PRODUCT_REF_ID = uid(); INFO_PLIST_REF = uid()
ASSETS_REF = uid(); ASSETS_BF = uid(); INFO_BF = uid()

file_refs = {s: uid() for s in sources}
build_files = {s: uid() for s in sources}

lines = []
def w(s=""): lines.append(s)

w("// !$*UTF8*$!")
w("{")
w("\tarchiveVersion = 1;")
w("\tclasses = {};")
w("\tobjectVersion = 56;")
w("\tobjects = {")
w()

# PBXBuildFile
w("/* Begin PBXBuildFile section */")
for p, bf in build_files.items():
    name = os.path.basename(p)
    w(f"\t\t{bf} /* {name} in Sources */ = {{isa = PBXBuildFile; fileRef = {file_refs[p]} /* {name} */; }};")
w(f"\t\t{ASSETS_BF} /* Assets.xcassets in Resources */ = {{isa = PBXBuildFile; fileRef = {ASSETS_REF} /* Assets.xcassets */; }};")
w("/* End PBXBuildFile section */")
w()

# PBXFileReference
w("/* Begin PBXFileReference section */")
for p, fr in file_refs.items():
    name = os.path.basename(p)
    w(f"\t\t{fr} /* {name} */ = {{isa = PBXFileReference; lastKnownFileType = sourcecode.swift; path = \"{name}\"; sourceTree = \"<group>\"; }};")
w(f"\t\t{INFO_PLIST_REF} /* Info.plist */ = {{isa = PBXFileReference; lastKnownFileType = text.plist.xml; path = Info.plist; sourceTree = \"<group>\"; }};")
w(f"\t\t{ASSETS_REF} /* Assets.xcassets */ = {{isa = PBXFileReference; lastKnownFileType = folder.assetcatalog; path = Assets.xcassets; sourceTree = \"<group>\"; }};")
w(f"\t\t{PRODUCT_REF_ID} /* ProIdentity.app */ = {{isa = PBXFileReference; explicitFileType = wrapper.application; includeInIndex = 0; path = ProIdentity.app; sourceTree = BUILT_PRODUCTS_DIR; }};")
w("/* End PBXFileReference section */")
w()

# Frameworks
w("/* Begin PBXFrameworksBuildPhase section */")
w(f"\t\t{FRAMEWORKS_PHASE_ID} /* Frameworks */ = {{")
w(f"\t\t\tisa = PBXFrameworksBuildPhase; buildActionMask = 2147483647; files = (); runOnlyForDeploymentPostprocessing = 0;")
w(f"\t\t}};")
w("/* End PBXFrameworksBuildPhase section */")
w()

# Groups - build from source paths
group_ids = {}
def ensure_group(d):
    if d in group_ids: return group_ids[d]
    gid = uid()
    group_ids[d] = gid
    return gid

root_gid = ensure_group("")
for p in sources:
    d = os.path.dirname(p)
    if d: ensure_group(d)

# Build children lists
children = {d: [] for d in group_ids}
for p, fr in file_refs.items():
    d = os.path.dirname(p)
    children[d].append((fr, os.path.basename(p)))
# Sub-groups parented to root
for d in group_ids:
    if d and os.path.dirname(d) == "":
        children[""].append((group_ids[d], d))

# Add resources to root
children[""].append((INFO_PLIST_REF, "Info.plist"))
children[""].append((ASSETS_REF, "Assets.xcassets"))

w("/* Begin PBXGroup section */")
for d, gid in group_ids.items():
    name = os.path.basename(d) if d else "ProIdentity"
    w(f"\t\t{gid} /* {name} */ = {{")
    w(f"\t\t\tisa = PBXGroup; children = (")
    for cid, cname in children.get(d, []):
        w(f"\t\t\t\t{cid} /* {cname} */,")
    w(f"\t\t\t);")
    w(f"\t\t\t{'path = ' + (os.path.basename(d) if d else 'ProIdentity') + ';'}")
    w(f"\t\t\tsourceTree = \"<group>\";")
    w(f"\t\t}};")

# Main group
w(f"\t\t{MAIN_GROUP_ID} /* ProIdentityApp */ = {{")
w(f"\t\t\tisa = PBXGroup; children = ({root_gid} /* ProIdentity */, {PRODUCTS_GROUP_ID} /* Products */,);")
w(f"\t\t\tsourceTree = \"<group>\";")
w(f"\t\t}};")
w(f"\t\t{PRODUCTS_GROUP_ID} /* Products */ = {{")
w(f"\t\t\tisa = PBXGroup; children = ({PRODUCT_REF_ID} /* ProIdentity.app */,); name = Products; sourceTree = \"<group>\";")
w(f"\t\t}};")
w("/* End PBXGroup section */")
w()

# NativeTarget
w("/* Begin PBXNativeTarget section */")
w(f"\t\t{TARGET_ID} /* ProIdentity */ = {{")
w(f"\t\t\tisa = PBXNativeTarget;")
w(f"\t\t\tbuildConfigurationList = {BUILD_CONFIG_LIST_TARGET};")
w(f"\t\t\tbuildPhases = ({SOURCES_PHASE_ID} /* Sources */, {FRAMEWORKS_PHASE_ID} /* Frameworks */, {RESOURCES_PHASE_ID} /* Resources */,);")
w(f"\t\t\tbuildRules = (); dependencies = ();")
w(f"\t\t\tname = ProIdentity; productName = ProIdentity;")
w(f"\t\t\tproductReference = {PRODUCT_REF_ID}; productType = \"com.apple.product-type.application\";")
w(f"\t\t}};")
w("/* End PBXNativeTarget section */")
w()

# Project
w("/* Begin PBXProject section */")
w(f"\t\t{PROJECT_ID} /* Project object */ = {{")
w(f"\t\t\tisa = PBXProject;")
w(f"\t\t\tattributes = {{ BuildIndependentTargetsInParallel = 1; LastSwiftUpdateCheck = 1540; LastUpgradeCheck = 1540; TargetAttributes = {{ {TARGET_ID} = {{ CreatedOnToolsVersion = 15.4; }}; }}; }};")
w(f"\t\t\tbuildConfigurationList = {BUILD_CONFIG_LIST_PROJECT};")
w(f"\t\t\tcompatibilityVersion = \"Xcode 14.0\"; developmentRegion = en; hasScannedForEncodings = 0;")
w(f"\t\t\tknownRegions = (en, Base,); mainGroup = {MAIN_GROUP_ID};")
w(f"\t\t\tproductRefGroup = {PRODUCTS_GROUP_ID}; projectDirPath = \"\"; projectRoot = \"\";")
w(f"\t\t\ttargets = ({TARGET_ID} /* ProIdentity */,);")
w(f"\t\t}};")
w("/* End PBXProject section */")
w()

# Resources
w("/* Begin PBXResourcesBuildPhase section */")
w(f"\t\t{RESOURCES_PHASE_ID} /* Resources */ = {{")
w(f"\t\t\tisa = PBXResourcesBuildPhase; buildActionMask = 2147483647;")
w(f"\t\t\tfiles = ({ASSETS_BF} /* Assets.xcassets in Resources */,);")
w(f"\t\t\trunOnlyForDeploymentPostprocessing = 0;")
w(f"\t\t}};")
w("/* End PBXResourcesBuildPhase section */")
w()

# Sources
w("/* Begin PBXSourcesBuildPhase section */")
w(f"\t\t{SOURCES_PHASE_ID} /* Sources */ = {{")
w(f"\t\t\tisa = PBXSourcesBuildPhase; buildActionMask = 2147483647; files = (")
for p, bf in build_files.items():
    w(f"\t\t\t\t{bf} /* {os.path.basename(p)} in Sources */,")
w(f"\t\t\t); runOnlyForDeploymentPostprocessing = 0;")
w(f"\t\t}};")
w("/* End PBXSourcesBuildPhase section */")
w()

# Build configs
common_project = """ALWAYS_SEARCH_USER_PATHS = NO; CLANG_ENABLE_MODULES = YES; CLANG_ENABLE_OBJC_ARC = YES; GCC_C_LANGUAGE_STANDARD = gnu17; IPHONEOS_DEPLOYMENT_TARGET = 16.0; MTL_FAST_MATH = YES; SDKROOT = iphoneos;"""
common_target  = f"""ASSETCATALOG_COMPILER_APPICON_NAME = AppIcon; CODE_SIGN_ENTITLEMENTS = ProIdentity/ProIdentity.entitlements; CODE_SIGN_STYLE = Automatic; CURRENT_PROJECT_VERSION = 18; DEVELOPMENT_TEAM = ""; GENERATE_INFOPLIST_FILE = NO; INFOPLIST_FILE = ProIdentity/Info.plist; IPHONEOS_DEPLOYMENT_TARGET = 16.0; MARKETING_VERSION = 0.5.18; PRODUCT_BUNDLE_IDENTIFIER = com.proidentity.ios; PRODUCT_NAME = "$(TARGET_NAME)"; SDKROOT = iphoneos; SWIFT_VERSION = 5.0; TARGETED_DEVICE_FAMILY = "1,2";"""

w("/* Begin XCBuildConfiguration section */")
for cid, name, extra in [
    (DEBUG_CONFIG_PROJECT,  "Debug",   "GCC_OPTIMIZATION_LEVEL = 0; ONLY_ACTIVE_ARCH = YES; SWIFT_OPTIMIZATION_LEVEL = \"-Onone\"; SWIFT_ACTIVE_COMPILATION_CONDITIONS = DEBUG;"),
    (RELEASE_CONFIG_PROJECT,"Release", "VALIDATE_PRODUCT = YES; SWIFT_OPTIMIZATION_LEVEL = \"-O\";"),
    (DEBUG_CONFIG_TARGET,   "Debug",   ""),
    (RELEASE_CONFIG_TARGET, "Release", ""),
]:
    base = common_project if cid in [DEBUG_CONFIG_PROJECT, RELEASE_CONFIG_PROJECT] else common_target
    w(f"\t\t{cid} /* {name} */ = {{ isa = XCBuildConfiguration; buildSettings = {{ {base} {extra} }}; name = {name}; }};")
w("/* End XCBuildConfiguration section */")
w()

# Config lists
w("/* Begin XCConfigurationList section */")
w(f"\t\t{BUILD_CONFIG_LIST_PROJECT} = {{ isa = XCConfigurationList; buildConfigurations = ({DEBUG_CONFIG_PROJECT} /* Debug */, {RELEASE_CONFIG_PROJECT} /* Release */,); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; }};")
w(f"\t\t{BUILD_CONFIG_LIST_TARGET} = {{ isa = XCConfigurationList; buildConfigurations = ({DEBUG_CONFIG_TARGET} /* Debug */, {RELEASE_CONFIG_TARGET} /* Release */,); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; }};")
w("/* End XCConfigurationList section */")
w()
w("\t};")
w(f"\trootObject = {PROJECT_ID} /* Project object */;")
w("}")

out = "\n".join(lines)
os.makedirs("ProIdentity.xcodeproj", exist_ok=True)
with open("ProIdentity.xcodeproj/project.pbxproj", "w") as f:
    f.write(out)

# Write scheme
scheme = f"""<?xml version="1.0" encoding="UTF-8"?>
<Scheme LastUpgradeVersion="1540" version="1.7">
   <BuildAction parallelizeBuildables="YES" buildImplicitDependencies="YES">
      <BuildActionEntries>
         <BuildActionEntry buildForTesting="YES" buildForRunning="YES" buildForProfiling="YES" buildForArchiving="YES" buildForAnalyzing="YES">
            <BuildableReference BuildableIdentifier="primary" BlueprintIdentifier="{TARGET_ID}" BuildableName="ProIdentity.app" BlueprintName="ProIdentity" ReferencedContainer="container:ProIdentity.xcodeproj"/>
         </BuildActionEntry>
      </BuildActionEntries>
   </BuildAction>
   <TestAction buildConfiguration="Debug" selectedDebuggerIdentifier="Xcode.DebuggerFoundation.Debugger.LLDB" selectedLauncherIdentifier="Xcode.DebuggerFoundation.Launcher.LLDB" shouldUseLaunchSchemeArgsEnv="YES"><Testables/></TestAction>
   <LaunchAction buildConfiguration="Debug" selectedDebuggerIdentifier="Xcode.DebuggerFoundation.Debugger.LLDB" selectedLauncherIdentifier="Xcode.DebuggerFoundation.Launcher.LLDB" launchStyle="0" useCustomWorkingDirectory="NO" ignoresPersistentStateOnLaunch="NO" debugDocumentVersioning="YES" debugServiceExtension="internal" allowLocationSimulation="YES">
      <BuildableProductRunnable runnableDebuggingMode="0">
         <BuildableReference BuildableIdentifier="primary" BlueprintIdentifier="{TARGET_ID}" BuildableName="ProIdentity.app" BlueprintName="ProIdentity" ReferencedContainer="container:ProIdentity.xcodeproj"/>
      </BuildableProductRunnable>
   </LaunchAction>
   <ProfileAction buildConfiguration="Release" shouldUseLaunchSchemeArgsEnv="YES" savedToolIdentifier="" useCustomWorkingDirectory="NO" debugDocumentVersioning="YES">
      <BuildableProductRunnable runnableDebuggingMode="0">
         <BuildableReference BuildableIdentifier="primary" BlueprintIdentifier="{TARGET_ID}" BuildableName="ProIdentity.app" BlueprintName="ProIdentity" ReferencedContainer="container:ProIdentity.xcodeproj"/>
      </BuildableProductRunnable>
   </ProfileAction>
   <AnalyzeAction buildConfiguration="Debug"/>
   <ArchiveAction buildConfiguration="Release" revealArchiveInOrganizer="YES"/>
</Scheme>"""

os.makedirs("ProIdentity.xcodeproj/xcshareddata/xcschemes", exist_ok=True)
with open("ProIdentity.xcodeproj/xcshareddata/xcschemes/ProIdentity.xcscheme", "w") as f:
    f.write(scheme)

print("Generated ProIdentity.xcodeproj")
print(f"  Target ID: {TARGET_ID}")
