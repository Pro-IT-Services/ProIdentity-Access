package auth

import (
	"encoding/json"
	"fmt"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"proidentity/internal/model"
)

// WebAuthnUser adapts our User model to the webauthn.User interface.
type WebAuthnUser struct {
	User     *model.User
	Passkeys []*model.Passkey
}

func (u *WebAuthnUser) WebAuthnID() []byte            { return []byte(u.User.ID) }
func (u *WebAuthnUser) WebAuthnName() string           { return u.User.Username }
func (u *WebAuthnUser) WebAuthnDisplayName() string    { return u.User.Username }
func (u *WebAuthnUser) WebAuthnIcon() string           { return "" }
func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, len(u.Passkeys))
	for i, pk := range u.Passkeys {
		creds[i] = webauthn.Credential{
			ID:              pk.CredentialID,
			PublicKey:       pk.PublicKey,
			AttestationType: "none",
			Authenticator: webauthn.Authenticator{
				SignCount: pk.SignCount,
			},
		}
	}
	return creds
}

// NewWebAuthn creates a configured WebAuthn instance from DB settings.
func NewWebAuthn(rpID, rpName, origin string) (*webauthn.WebAuthn, error) {
	wn, err := webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: rpName,
		RPOrigins:     []string{origin},
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn init: %w", err)
	}
	return wn, nil
}

// SessionData is serialisable WebAuthn session state (stored server-side between begin/finish).
type SessionData struct {
	Challenge        string `json:"challenge"`
	UserID           []byte `json:"user_id"`
	AllowedCredentials []protocol.CredentialDescriptor `json:"allowed_credentials,omitempty"`
	UserVerification string `json:"user_verification"`
}

func MarshalSession(sd *webauthn.SessionData) ([]byte, error) {
	return json.Marshal(sd)
}

func UnmarshalSession(data []byte) (*webauthn.SessionData, error) {
	var sd webauthn.SessionData
	if err := json.Unmarshal(data, &sd); err != nil {
		return nil, err
	}
	return &sd, nil
}
