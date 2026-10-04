package cli

import (
	"fmt"

	"github.com/maahdima/mwp/api/service"
)

// actionDisableOtp turns off the admin-login two-factor gate
// (BotSettings.OtpEnabled) directly from the server -- the only recovery
// path when an admin is locked out because the Telegram bot that delivers
// their login codes stopped working (token revoked, bot blocked, chat
// deleted) and they can no longer complete a login to turn it off from the
// web UI's own settings page.
func actionDisableOtp() {
	fmt.Println()
	db, err := connectDB()
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		return
	}

	botSettingsService := service.NewBotSettingsService(db)
	settings, err := botSettingsService.GetOrCreate()
	if err != nil {
		fmt.Printf("Failed to read bot settings: %v\n", err)
		return
	}

	if !settings.OtpEnabled {
		fmt.Println("Two-factor login is already disabled.")
		return
	}

	disabled := false
	if _, err := botSettingsService.UpdateSettings(service.UpdateSettingsInput{OtpEnabled: &disabled}); err != nil {
		fmt.Printf("Failed to disable two-factor login: %v\n", err)
		return
	}

	fmt.Println("Two-factor login disabled. Admins can now log in with just their username/password.")
	fmt.Println("Re-enable it from the web UI's bot settings page once Telegram delivery is working again.")
}
