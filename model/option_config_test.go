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
	oldGitHubOAuthEnabled := config.GitHubOAuthEnabled
	oldGitHubClientID := config.GitHubClientId
	oldGitHubClientSecret := config.GitHubClientSecret
	oldGoogleClientID := config.GoogleClientId
	oldGoogleClientSecret := config.GoogleClientSecret

	config.GlobalOption = config.NewOptionManager()
	config.ServerAddress = "http://localhost:3000"
	config.GitHubOAuthEnabled = false
	config.GoogleOAuthEnabled = false
	config.GoogleOnlyRegisterEnabled = false
	config.PasswordLoginEnabled = true
	config.GitHubClientId = ""
	config.GitHubClientSecret = ""
	config.GoogleClientId = ""
	config.GoogleClientSecret = ""

	defer func() {
		config.GlobalOption = oldGlobalOption
		config.ServerAddress = oldServerAddress
		config.GitHubOAuthEnabled = oldGitHubOAuthEnabled
		config.GoogleOAuthEnabled = oldGoogleOAuthEnabled
		config.GoogleOnlyRegisterEnabled = oldGoogleOnlyRegisterEnabled
		config.PasswordLoginEnabled = oldPasswordLoginEnabled
		config.GitHubClientId = oldGitHubClientID
		config.GitHubClientSecret = oldGitHubClientSecret
		config.GoogleClientId = oldGoogleClientID
		config.GoogleClientSecret = oldGoogleClientSecret
		viper.Reset()
	}()

	config.GlobalOption.RegisterString("ServerAddress", &config.ServerAddress)
	config.GlobalOption.RegisterBool("GitHubOAuthEnabled", &config.GitHubOAuthEnabled)
	config.GlobalOption.RegisterBool("GoogleOAuthEnabled", &config.GoogleOAuthEnabled)
	config.GlobalOption.RegisterBool("GoogleOnlyRegisterEnabled", &config.GoogleOnlyRegisterEnabled)
	config.GlobalOption.RegisterBool("PasswordLoginEnabled", &config.PasswordLoginEnabled)
	config.GlobalOption.RegisterString("GitHubClientId", &config.GitHubClientId)
	config.GlobalOption.RegisterString("GitHubClientSecret", &config.GitHubClientSecret)
	config.GlobalOption.RegisterString("GoogleClientId", &config.GoogleClientId)
	config.GlobalOption.RegisterString("GoogleClientSecret", &config.GoogleClientSecret)

	viper.Set("server_address", "https://tokilake.example.com")
	viper.Set("github_oauth_enabled", true)
	viper.Set("google_oauth_enabled", true)
	viper.Set("google_only_register_enabled", true)
	viper.Set("password_login_enabled", false)
	viper.Set("github_client_id", "github-client-id")
	viper.Set("github_client_secret", "github-client-secret")
	viper.Set("google_client_id", "google-client-id")
	viper.Set("google_client_secret", "google-client-secret")

	loadOptionsFromConfig()

	assert.Equal(t, "https://tokilake.example.com", config.ServerAddress)
	assert.True(t, config.GitHubOAuthEnabled)
	assert.True(t, config.GoogleOAuthEnabled)
	assert.True(t, config.GoogleOnlyRegisterEnabled)
	assert.False(t, config.PasswordLoginEnabled)
	assert.Equal(t, "github-client-id", config.GitHubClientId)
	assert.Equal(t, "github-client-secret", config.GitHubClientSecret)
	assert.Equal(t, "google-client-id", config.GoogleClientId)
	assert.Equal(t, "google-client-secret", config.GoogleClientSecret)
}
