package config

import (
	"time"

	"golang.org/x/oauth2"
)

type Config interface {
	OAuth2State() OAuth2State
	OAuth2Configs() map[string]oauth2.Config
	OAuth2Providers() []OAuth2Provider
	ModTime() time.Time
}

type OAuth2State interface {
	TokenIssuer() string
	TokenSignKey() []byte
	Expiration() time.Duration
}

type OAuth2Provider interface {
	ID() string
	Name() string
	ClientId() string
	ClientSecret() string
	PublicKey() string
	Scopes() []string
	IconURL() string
	TokenURL() string
	AuthURL() string
}