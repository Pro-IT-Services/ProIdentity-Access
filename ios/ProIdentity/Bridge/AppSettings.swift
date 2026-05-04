import Foundation
import Security

/// Secure storage using Keychain — replaces Android's EncryptedSharedPreferences
class AppSettings {
    static let shared = AppSettings()
    private let service = "com.proidentity.ios"

    // MARK: - Keys
    enum Key: String {
        case serverURL, token, username, isAdmin, vpnName, totpEnabled
        case deviceID, clientPrivateKey, serverPublicKey
        case mode, setupDone
    }

    // MARK: - Keychain helpers
    private func set(_ value: String, for key: Key) {
        let data = value.data(using: .utf8)!
        let query: [CFString: Any] = [
            kSecClass: kSecClassGenericPassword,
            kSecAttrService: service,
            kSecAttrAccount: key.rawValue
        ]
        SecItemDelete(query as CFDictionary)
        var add = query
        add[kSecValueData] = data
        add[kSecAttrAccessible] = kSecAttrAccessibleWhenUnlockedThisDeviceOnly
        SecItemAdd(add as CFDictionary, nil)
    }

    private func get(_ key: Key) -> String? {
        let query: [CFString: Any] = [
            kSecClass: kSecClassGenericPassword,
            kSecAttrService: service,
            kSecAttrAccount: key.rawValue,
            kSecReturnData: true,
            kSecMatchLimit: kSecMatchLimitOne
        ]
        var result: AnyObject?
        guard SecItemCopyMatching(query as CFDictionary, &result) == errSecSuccess,
              let data = result as? Data else { return nil }
        return String(data: data, encoding: .utf8)
    }

    private func delete(_ key: Key) {
        let query: [CFString: Any] = [
            kSecClass: kSecClassGenericPassword,
            kSecAttrService: service,
            kSecAttrAccount: key.rawValue
        ]
        SecItemDelete(query as CFDictionary)
    }

    // MARK: - Properties
    var serverURL: String    { get { get(.serverURL) ?? "" }       set { set(newValue, for: .serverURL) } }
    var token: String        { get { get(.token) ?? "" }           set { set(newValue, for: .token) } }
    var username: String     { get { get(.username) ?? "" }        set { set(newValue, for: .username) } }
    var isAdmin: Bool        { get { get(.isAdmin) == "true" }     set { set(newValue ? "true" : "false", for: .isAdmin) } }
    var vpnName: String      { get { get(.vpnName) ?? "" }         set { set(newValue, for: .vpnName) } }
    var totpEnabled: Bool    { get { get(.totpEnabled) == "true" } set { set(newValue ? "true" : "false", for: .totpEnabled) } }
    var deviceID: String     { get { get(.deviceID) ?? "" }        set { set(newValue, for: .deviceID) } }
    var clientPrivateKey: String { get { get(.clientPrivateKey) ?? "" } set { set(newValue, for: .clientPrivateKey) } }
    var serverPublicKey: String  { get { get(.serverPublicKey) ?? "" }  set { set(newValue, for: .serverPublicKey) } }
    var mode: String         { get { get(.mode) ?? "standalone" }  set { set(newValue, for: .mode) } }
    var setupDone: Bool      { get { get(.setupDone) == "true" }   set { set(newValue ? "true" : "false", for: .setupDone) } }

    func wipeAll() {
        Key.allCases.forEach { delete($0) }
    }

    func resetToRegister() {
        // Keep serverURL and mode, clear everything else
        [Key.token, .username, .isAdmin, .vpnName, .totpEnabled,
         .deviceID, .clientPrivateKey, .serverPublicKey, .setupDone].forEach { delete($0) }
    }
}

extension AppSettings.Key: CaseIterable {}
