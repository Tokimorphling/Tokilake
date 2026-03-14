package common

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"one-api/common/logger"
)

func TestInitUserTokenFallsBackOnInvalidHashidsSalt(t *testing.T) {
	logger.SetupLogger()

	originalTokenSecret := viper.GetString("user_token_secret")
	originalHashidsSalt := viper.GetString("hashids_salt")
	t.Cleanup(func() {
		viper.Set("user_token_secret", originalTokenSecret)
		viper.Set("hashids_salt", originalHashidsSalt)
	})

	viper.Set("user_token_secret", "test-user-token-secret")
	viper.Set("hashids_salt", "00112233445566778899aabbccddeeff")

	err := InitUserToken()
	require.NoError(t, err)

	token, err := GenerateToken(12, 34)
	require.NoError(t, err)

	tokenID, userID, err := ValidateToken(token)
	require.NoError(t, err)
	require.Equal(t, 12, tokenID)
	require.Equal(t, 34, userID)
}
