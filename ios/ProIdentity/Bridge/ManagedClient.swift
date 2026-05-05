import Foundation
import CryptoKit

/// HTTP client for the management server — replaces ManagedClient.kt
class ManagedClient {
    static let shared = ManagedClient()
    private let session: URLSession

    init() {
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 15
        self.session = URLSession(configuration: config)
    }

    // MARK: - Request builder
    private func makeRequest(_ path: String, method: String = "GET", body: Any? = nil,
                              token: String? = nil, deviceID: String? = nil,
                              aesKey: SymmetricKey? = nil) throws -> URLRequest {
        let serverURL = AppSettings.shared.serverURL
        try validateBaseURL(serverURL)
        guard let url = URL(string: serverURL.trimmingCharacters(in: .whitespaces) + "/api/v1" + path) else {
            throw APIError.invalidURL
        }
        var req = URLRequest(url: url)
        req.httpMethod = method
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.setValue("ProIdentity-Access", forHTTPHeaderField: "User-Agent")
        if let t = token { req.setValue("Bearer \(t)", forHTTPHeaderField: "Authorization") }
        if let d = deviceID { req.setValue(d, forHTTPHeaderField: "X-Device-ID") }

        if let body = body {
            if let key = aesKey {
                let aad = (deviceID ?? AppSettings.shared.deviceID).data(using: .utf8) ?? Data()
                let encrypted = try DeviceCrypto.shared.encryptJSON(body, key: key, aad: aad)
                req.httpBody = encrypted.data(using: .utf8)
            } else {
                req.httpBody = try JSONSerialization.data(withJSONObject: body)
            }
        }
        return req
    }

    private func perform(_ req: URLRequest, aesKey: SymmetricKey? = nil) async throws -> Any {
        let (data, response) = try await session.data(for: req)
        guard let http = response as? HTTPURLResponse else { throw APIError.invalidResponse }
        let authenticatedRequest = req.value(forHTTPHeaderField: "Authorization")?.isEmpty == false

        // Check for device revocation
        if http.statusCode == 401,
           let json = try? JSONSerialization.jsonObject(with: data) as? [String: String],
           let error = json["error"]?.lowercased() {
            if error.contains("device revoked") || error.contains("unknown device") {
                throw APIError.deviceRevoked
            }
            if authenticatedRequest {
                throw APIError.authInvalid
            }
        }
        if authenticatedRequest && (http.statusCode == 401 || http.statusCode == 403) {
            throw APIError.authInvalid
        }
        guard (200..<300).contains(http.statusCode) else {
            let msg = String(data: data, encoding: .utf8) ?? "HTTP \(http.statusCode)"
            throw APIError.serverError(msg)
        }

        if let key = aesKey {
            let envelope = String(data: data, encoding: .utf8) ?? ""
            let aad = AppSettings.shared.deviceID.data(using: .utf8) ?? Data()
            let plain = try DeviceCrypto.shared.decrypt(envelope: envelope, key: key, aad: aad)
            return try JSONSerialization.jsonObject(with: plain.data(using: .utf8)!)
        }
        return try JSONSerialization.jsonObject(with: data)
    }

    // MARK: - Endpoints
    func register(serverURL: String, deviceName: String, publicKey: String) async throws -> [String: Any] {
        try validateBaseURL(serverURL)
        guard let url = URL(string: serverURL.trimmingCharacters(in: .whitespaces) + "/api/v1/register") else {
            throw APIError.invalidURL
        }
        var req = URLRequest(url: url)
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try JSONSerialization.data(withJSONObject: [
            "device_name": deviceName,
            "client_public_key": publicKey
        ])
        let (data, _) = try await session.data(for: req)
        return try JSONSerialization.jsonObject(with: data) as? [String: Any] ?? [:]
    }

    func login(username: String, password: String, totpCode: String, pushAuthID: String = "", aesKey: SymmetricKey) async throws -> [String: Any] {
        var body: [String: Any] = ["username": username, "password": password]
        if !totpCode.isEmpty { body["totp_code"] = totpCode }
        if !pushAuthID.isEmpty { body["push_auth_request_id"] = pushAuthID }
        let req = try makeRequest("/auth/login", method: "POST", body: body,
                                   deviceID: AppSettings.shared.deviceID, aesKey: aesKey)
        return try await perform(req, aesKey: aesKey) as? [String: Any] ?? [:]
    }

    func getInfo(aesKey: SymmetricKey) async throws -> [String: Any] {
        let req = try makeRequest("/info", token: AppSettings.shared.token,
                                   deviceID: AppSettings.shared.deviceID, aesKey: aesKey)
        return try await perform(req, aesKey: aesKey) as? [String: Any] ?? [:]
    }

    func listServers(aesKey: SymmetricKey) async throws -> [[String: Any]] {
        let req = try makeRequest("/servers", token: AppSettings.shared.token,
                                   deviceID: AppSettings.shared.deviceID, aesKey: aesKey)
        return try await perform(req, aesKey: aesKey) as? [[String: Any]] ?? []
    }

    func createSession(serverID: String, clientPublicKey: String, totpCode: String, pushAuthID: String = "", aesKey: SymmetricKey) async throws -> [String: Any] {
        var body: [String: Any] = ["server_id": serverID, "client_public_key": clientPublicKey]
        if !totpCode.isEmpty { body["totp_code"] = totpCode }
        if !pushAuthID.isEmpty { body["push_auth_request_id"] = pushAuthID }
        let req = try makeRequest("/sessions", method: "POST", body: body,
                                   token: AppSettings.shared.token,
                                   deviceID: AppSettings.shared.deviceID, aesKey: aesKey)
        return try await perform(req, aesKey: aesKey) as? [String: Any] ?? [:]
    }

    func keepalive(sessionID: String, aesKey: SymmetricKey) async throws {
        let req = try makeRequest("/sessions/\(sessionID)/keepalive", method: "POST", body: [:],
                                   token: AppSettings.shared.token,
                                   deviceID: AppSettings.shared.deviceID, aesKey: aesKey)
        _ = try await perform(req, aesKey: aesKey)
    }

    func deleteSession(sessionID: String, aesKey: SymmetricKey) async {
        let req = try? makeRequest("/sessions/\(sessionID)", method: "DELETE",
                                    token: AppSettings.shared.token,
                                    deviceID: AppSettings.shared.deviceID, aesKey: aesKey)
        if let req = req { _ = try? await perform(req, aesKey: aesKey) }
    }

    func deleteAuthSession(aesKey: SymmetricKey) async throws {
        let req = try makeRequest("/auth/logout", method: "POST",
                                   token: AppSettings.shared.token,
                                   deviceID: AppSettings.shared.deviceID, aesKey: aesKey)
        _ = try? await perform(req, aesKey: aesKey)
    }

    // Push auth — public poll (no auth/device headers, plain JSON)
    func pollPushStatus(requestID: String) async throws -> String {
        let serverURL = AppSettings.shared.serverURL
        guard let url = URL(string: serverURL.trimmingCharacters(in: .whitespaces) + "/api/v1/auth/push-status/" + requestID) else {
            throw APIError.invalidURL
        }
        var req = URLRequest(url: url)
        req.httpMethod = "GET"
        req.setValue("ProIdentity-Access", forHTTPHeaderField: "User-Agent")
        let (data, _) = try await session.data(for: req)
        let json = try JSONSerialization.jsonObject(with: data) as? [String: Any] ?? [:]
        return json["status"] as? String ?? "pending"
    }

    // Push auth — create request (authenticated, for connect flow)
    func createPushAuth(context: String, aesKey: SymmetricKey) async throws -> [String: Any] {
        let req = try makeRequest("/auth/push", method: "POST", body: ["context": context],
                                   token: AppSettings.shared.token,
                                   deviceID: AppSettings.shared.deviceID, aesKey: aesKey)
        return try await perform(req, aesKey: aesKey) as? [String: Any] ?? [:]
    }

    // Auth check — verifies token is still valid
    func checkAuth(aesKey: SymmetricKey) async throws {
        let req = try makeRequest("/auth/me", token: AppSettings.shared.token,
                                   deviceID: AppSettings.shared.deviceID, aesKey: aesKey)
        _ = try await perform(req, aesKey: aesKey)
    }

    // Helper: build AES key from stored credentials
    func aesKey() throws -> SymmetricKey {
        try DeviceCrypto.shared.deriveSharedKey(
            privateKeyB64: AppSettings.shared.clientPrivateKey,
            serverPublicKeyB64: AppSettings.shared.serverPublicKey
        )
    }

    private func validateBaseURL(_ value: String) throws {
        guard let comps = URLComponents(string: value.trimmingCharacters(in: .whitespacesAndNewlines)),
              let scheme = comps.scheme?.lowercased(),
              let host = comps.host,
              !host.isEmpty else {
            throw APIError.invalidURL
        }
        let localhost = host == "localhost" || host == "127.0.0.1" || host == "::1"
        guard scheme == "https" || (scheme == "http" && localhost) else {
            throw APIError.serverError("Server URL must use HTTPS")
        }
        guard comps.query == nil && comps.fragment == nil else {
            throw APIError.invalidURL
        }
    }
}

enum APIError: LocalizedError {
    case invalidURL, invalidResponse, deviceRevoked, authInvalid, serverError(String)
    var errorDescription: String? {
        switch self {
        case .invalidURL: return "Invalid URL"
        case .invalidResponse: return "Invalid response"
        case .deviceRevoked: return "Device revoked"
        case .authInvalid: return "Login expired or revoked"
        case .serverError(let m): return m
        }
    }
}
