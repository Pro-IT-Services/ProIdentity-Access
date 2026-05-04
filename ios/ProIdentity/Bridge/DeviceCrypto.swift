import Foundation
import CryptoKit

/// Crypto primitives — replaces DeviceCrypto.kt
/// X25519 key exchange, HKDF-SHA256 derivation, AES-256-GCM encryption
class DeviceCrypto {
    static let shared = DeviceCrypto()

    // MARK: - X25519 key pair
    func generateKeyPair() -> (privateKey: String, publicKey: String) {
        let key = Curve25519.KeyAgreement.PrivateKey()
        let priv = key.rawRepresentation.base64EncodedString()
        let pub  = key.publicKey.rawRepresentation.base64EncodedString()
        return (priv, pub)
    }

    // MARK: - ECDH + HKDF → shared AES key
    // Matches Android: HKDF-SHA256(ikm=sharedSecret, salt=32zeros, info="proidentity-device-v1")
    func deriveSharedKey(privateKeyB64: String, serverPublicKeyB64: String) throws -> SymmetricKey {
        guard let privData = Data(base64Encoded: privateKeyB64),
              let pubData  = Data(base64Encoded: serverPublicKeyB64) else {
            throw CryptoError.invalidKey
        }
        let privKey = try Curve25519.KeyAgreement.PrivateKey(rawRepresentation: privData)
        let pubKey  = try Curve25519.KeyAgreement.PublicKey(rawRepresentation: pubData)
        let shared  = try privKey.sharedSecretFromKeyAgreement(with: pubKey)
        // RFC 5869: null salt = 32 zero bytes (SHA256 HashLen)
        let derived = shared.hkdfDerivedSymmetricKey(
            using: SHA256.self,
            salt: Data(count: 32),
            sharedInfo: "proidentity-device-v1".data(using: .utf8)!,
            outputByteCount: 32
        )
        return derived
    }

    // MARK: - AES-256-GCM encrypt → {"ct":"base64(nonce12||ciphertext+tag)"}
    // aad = device ID bytes (matches Android: deviceID.toByteArray())
    func encrypt(plaintext: String, key: SymmetricKey, aad: Data = Data()) throws -> String {
        let data = plaintext.data(using: .utf8)!
        var box: AES.GCM.SealedBox
        if aad.isEmpty {
            box = try AES.GCM.seal(data, using: key)
        } else {
            box = try AES.GCM.seal(data, using: key, authenticating: aad)
        }
        let combined = box.nonce + box.ciphertext + box.tag
        let b64 = combined.base64EncodedString()
        return "{\"ct\":\"\(b64)\"}"
    }

    // MARK: - AES-256-GCM decrypt from {"ct":"base64(nonce12||ciphertext+tag)"}
    func decrypt(envelope: String, key: SymmetricKey, aad: Data = Data()) throws -> String {
        guard let data = envelope.data(using: .utf8),
              let obj  = try? JSONSerialization.jsonObject(with: data) as? [String: String],
              let ct   = obj["ct"],
              let combined = Data(base64Encoded: ct),
              combined.count > 28 else {
            throw CryptoError.invalidCiphertext
        }
        let nonce      = try AES.GCM.Nonce(data: combined.prefix(12))
        let ciphertext = combined[12..<(combined.count - 16)]
        let tag        = combined.suffix(16)
        let sealedBox  = try AES.GCM.SealedBox(nonce: nonce, ciphertext: ciphertext, tag: tag)
        let plain: Data
        if aad.isEmpty {
            plain = try AES.GCM.open(sealedBox, using: key)
        } else {
            plain = try AES.GCM.open(sealedBox, using: key, authenticating: aad)
        }
        return String(data: plain, encoding: .utf8) ?? ""
    }

    // Convenience: encrypt JSON object with AAD
    func encryptJSON(_ obj: Any, key: SymmetricKey, aad: Data = Data()) throws -> String {
        let data = try JSONSerialization.data(withJSONObject: obj)
        return try encrypt(plaintext: String(data: data, encoding: .utf8)!, key: key, aad: aad)
    }
}

enum CryptoError: Error {
    case invalidKey, invalidCiphertext
}
