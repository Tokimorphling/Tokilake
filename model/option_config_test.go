package model

import (
	"testing"

	"one-api/common/config"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestLoadOptionsFromConfig(t *testing.T) {
	oldGlobalOption := config.GlobalOption
	oldServerAddress := config.ServerAddress
	oldGoogleOAuthEnabled := config.GoogleOAuthEnabled
	oldGoogleOnlyRegisterEnabled := config.GoogleOnlyRegisterEnabled
	oldPasswordLoginEnabled := config.PasswordLoginEnabled
	oldGoogleClientID := config.GoogleClientId
	oldGoogleClientSecret := config.GoogleClientSecret

	config.GlobalOption = config.NewOptionManager()
	config.ServerAddress = "http://localhost:3000"
	config.GoogleOAuthEnabled = false
	config.GoogleOnlyRegisterEnabled = false
	config.PasswordLoginEnabled = true
	config.GoogleClientId = ""
	config.GoogleClientSecret = ""

	defer func() {
		config.GlobalOption = oldGlobalOption
		config.ServerAddress = oldServerAddress
		config.GoogleOAuthEnabled = oldGoogleOAuthEnabled
		config.GoogleOnlyRegisterEnabled = oldGoogleOnlyRegisterEnabled
		config.PasswordLoginEnabled = oldPasswordLoginEnabled
		config.GoogleClientId = oldGoogleClientID
		config.GoogleClientSecret = oldGoogleClientSecret
		viper.Reset()
	}()

	config.GlobalOption.RegisterString("ServerAddress", &config.ServerAddress)
	config.GlobalOption.RegisterBool("GoogleOAuthEnabled", &config.GoogleOAuthEnabled)
	config.GlobalOption.RegisterBool("GoogleOnlyRegisterEnabled", &config.GoogleOnlyRegisterEnabled)
	config.GlobalOption.RegisterBool("PasswordLoginEnabled", &config.PasswordLoginEnabled)
	config.GlobalOption.RegisterString("GoogleClientId", &config.GoogleClientId)
	config.GlobalOption.RegisterString("GoogleClientSecret", &config.GoogleClientSecret)

	viper.Set("server_address", "https://tokilake.example.com")
	viper.Set("google_oauth_enabled", true)
	viper.Set("google_only_register_enabled", true)
	viper.Set("password_login_enabled", false)
	viper.Set("google_client_id", "google-client-id")
	viper.Set("google_client_secret", "google-client-secret")

	loadOptionsFromConfig()

	assert.Equal(t, "https://tokilake.example.com", config.ServerAddress)
	assert.True(t, config.GoogleOAuthEnabled)
	assert.True(t, config.GoogleOnlyRegisterEnabled)
	assert.False(t, config.PasswordLoginEnabled)
	assert.Equal(t, "google-client-id", config.GoogleClientId)
	assert.Equal(t, "google-client-secret", config.GoogleClientSecret)
}
