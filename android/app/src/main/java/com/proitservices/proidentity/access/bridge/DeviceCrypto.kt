package com.proitservices.proidentity.access.bridge

import android.util.Base64
import org.bouncycastle.crypto.agreement.X25519Agreement
import org.bouncycastle.crypto.generators.X25519KeyPairGenerator
import org.bouncycastle.crypto.params.X25519KeyGenerationParameters
import org.bouncycastle.crypto.params.X25519PrivateKeyParameters
import org.bouncycastle.crypto.params.X25519PublicKeyParameters
import org.json.JSONObject
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.Mac
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

object DeviceCrypto {

    private val rng = SecureRandom()

    /**
     * Generate an X25519 key pair with the standard WireGuard clamping:
     *   priv[0] &= 248
     *   priv[31] &= 127
     *   priv[31] |= 64
     * Returns (privB64, pubB64) both standard Base64.
     */
    fun generateX25519KeyPair(): Pair<String, String> {
        val privBytes = ByteArray(32)
        rng.nextBytes(privBytes)
        clamp(privBytes)

        val privParams = X25519PrivateKeyParameters(privBytes, 0)
        val pubParams = privParams.generatePublicKey()
        val pubBytes = pubParams.encoded

        return Pair(
            Base64.encodeToString(privBytes, Base64.NO_WRAP),
            Base64.encodeToString(pubBytes, Base64.NO_WRAP)
        )
    }

    /** Same clamping for WireGuard session keys. */
    fun generateWireGuardKeypair(): Pair<String, String> = generateX25519KeyPair()

    /**
     * X25519 ECDH then HKDF-SHA256(ikm=sharedSecret, salt=nil, info="wg-manager-device-v1") â†’ 32-byte AES key.
     */
    fun deriveAESKey(privB64: String, peerPubB64: String): ByteArray {
        val privBytes = Base64.decode(privB64, Base64.NO_WRAP)
        val pubBytes = Base64.decode(peerPubB64, Base64.NO_WRAP)

        val privParams = X25519PrivateKeyParameters(privBytes, 0)
        val pubParams = X25519PublicKeyParameters(pubBytes, 0)

        val agreement = X25519Agreement()
        agreement.init(privParams)
        val shared = ByteArray(agreement.agreementSize)
        agreement.calculateAgreement(pubParams, shared, 0)

        return hkdfSha256(shared, null, "proidentity-device-v1".toByteArray(Charsets.UTF_8), 32)
    }

    /**
     * AES-256-GCM encrypt.
     * Output: JSON {"ct":"base64(nonce12||ciphertext+tag)"}
     * AAD = aad bytes
     */
    fun encryptBody(key: ByteArray, plaintext: ByteArray, aad: ByteArray): String {
        val nonce = ByteArray(12)
        rng.nextBytes(nonce)

        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, SecretKeySpec(key, "AES"), GCMParameterSpec(128, nonce))
        cipher.updateAAD(aad)
        val ct = cipher.doFinal(plaintext)

        // nonce || ciphertext+tag
        val combined = nonce + ct
        val b64 = Base64.encodeToString(combined, Base64.NO_WRAP)
        return JSONObject().apply { put("ct", b64) }.toString()
    }

    /**
     * AES-256-GCM decrypt.
     * Input: JSON {"ct":"base64(nonce12||ciphertext+tag)"}
     * AAD = aad bytes
     */
    fun decryptBody(key: ByteArray, envelope: String, aad: ByteArray): ByteArray {
        val obj = JSONObject(envelope)
        val combined = Base64.decode(obj.getString("ct"), Base64.NO_WRAP)

        require(combined.size >= 12) { "Ciphertext too short" }
        val nonce = combined.copyOfRange(0, 12)
        val ct = combined.copyOfRange(12, combined.size)

        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, SecretKeySpec(key, "AES"), GCMParameterSpec(128, nonce))
        cipher.updateAAD(aad)
        return cipher.doFinal(ct)
    }

    // --- Private helpers ---

    private fun clamp(priv: ByteArray) {
        priv[0] = (priv[0].toInt() and 248).toByte()
        priv[31] = (priv[31].toInt() and 127).toByte()
        priv[31] = (priv[31].toInt() or 64).toByte()
    }

    /**
     * HKDF-SHA256 as per RFC 5869.
     * salt = null means 32 zero bytes (as per spec).
     */
    private fun hkdfSha256(ikm: ByteArray, salt: ByteArray?, info: ByteArray, length: Int): ByteArray {
        val effectiveSalt = salt ?: ByteArray(32)

        // Extract
        val mac = Mac.getInstance("HmacSHA256")
        mac.init(SecretKeySpec(effectiveSalt, "HmacSHA256"))
        val prk = mac.doFinal(ikm)

        // Expand
        val result = ByteArray(length)
        var offset = 0
        var prev = ByteArray(0)
        var counter = 1
        while (offset < length) {
            mac.init(SecretKeySpec(prk, "HmacSHA256"))
            mac.update(prev)
            mac.update(info)
            mac.update(counter.toByte())
            prev = mac.doFinal()
            val toCopy = minOf(prev.size, length - offset)
            System.arraycopy(prev, 0, result, offset, toCopy)
            offset += toCopy
            counter++
        }
        return result
    }
}
