package apptoken

import (
	"testing"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestEmailToken(t *testing.T) {
	cases := []struct {
		name         string
		issuer1      string
		issuer2      string
		login        string
		passwordHash string
		exp          time.Duration
		key          string
		err          error
	}{
		{
			"Correct args",
			"some issuer",
			"some issuer",
			"sompe login",
			"some hash",
			1 * time.Hour,
			"some secret",
			nil,
		},
		{
			"Expired",
			"some issuer",
			"some issuer",
			"sompe login",
			"some hash",
			-1 * time.Hour,
			"some secret",
			&jwt.ValidationError{Errors: jwt.ValidationErrorExpired},
		},
		{
			"Illegal issuer",
			"some issuer",
			"some other issuer",
			"sompe login",
			"some hash",
			1 * time.Hour,
			"some secret",
			InvalidTokenError,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(st *testing.T) {
			assert := assert.New(st)

			key := []byte(c.key)
			m, err := NewEmailTokenString(c.issuer1, c.login, c.passwordHash, c.exp, key)
			assert.NoError(err)
			assert.NotZero(m)

			token, err := ParseEmailToken(c.issuer2, m, key)
			if c.err != nil {
				assert.Error(err)
				cause := errors.Cause(err)
				assert.IsType(c.err, cause)
				if e, ok := c.err.(*jwt.ValidationError); ok {
					assert.Equal(e.Errors, cause.(*jwt.ValidationError).Errors)
				}
			} else {
				assert.NoError(err)
				assert.NotZero(token)
				assert.Equal(c.login, token.Login())
				assert.Equal(c.passwordHash, token.PasswordHash())
			}
		})
	}
}
