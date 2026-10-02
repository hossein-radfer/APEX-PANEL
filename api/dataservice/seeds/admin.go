package seeds

import (
	"fmt"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/utils"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func AdminSeed(db *gorm.DB) error {
	var adminCount int64
	err := db.Find(&model.Admin{}).Count(&adminCount).Error
	if err != nil {
		fmt.Printf("Error when count admins: %s\n", err.Error())
		return err
	}

	if adminCount > 0 {
		return nil
	}

	adminConfig := config.GetAdminConfig()

	// minAdminPasswordLength MUST match loginRequestSchema's own minimum in
	// ui/src/schema/authentication.ts -- a confirmed gap this closes: an
	// operator who sets ADMIN_PASSWORD to anything shorter in their own
	// .env (e.g. "admin", 5 characters) used to get it seeded and hashed
	// here with no complaint, but the panel's own login FORM then refuses
	// to even submit a password that short, locking them out of their own
	// fresh install with no error from the server to explain why -- the
	// request never reaches the backend to produce one. Treating a
	// too-short configured password exactly like an unset one (generate a
	// safe random one instead, loudly) guarantees the credentials printed
	// below always actually work against the login form.
	const minAdminPasswordLength = 7
	tooShort := adminConfig.Password != "" && len(adminConfig.Password) < minAdminPasswordLength

	// Generate a random password and print it once, loudly, to the startup
	// log when ADMIN_PASSWORD isn't set (or is unusably short) -- the
	// operator retrieves it from the log/journal right after first
	// install, rather than every install sharing one fixed default
	// password.
	generatedPassword := ""
	if adminConfig.Password == "" || tooShort {
		generatedPassword = utils.RandomString(16)
		adminConfig.Password = generatedPassword
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminConfig.Password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("Error when hashing admin password: %s\n", err.Error())
		return err
	}

	admin := []model.Admin{
		{
			Username: adminConfig.Username,
			Password: string(hashedPassword),
		},
	}

	err = db.Save(&admin).Error
	if err != nil {
		fmt.Printf("Error when create admin record: %s\n", err.Error())
		return err
	}

	if generatedPassword != "" {
		fmt.Printf("\n================================================================\n")
		if tooShort {
			fmt.Printf(" The configured ADMIN_PASSWORD was too short (the panel's own\n")
			fmt.Printf(" login form requires at least %d characters) and was REJECTED --\n", minAdminPasswordLength)
			fmt.Printf(" generated a random one instead for the initial admin account.\n")
		} else {
			fmt.Printf(" No ADMIN_PASSWORD was configured -- generated a random one for\n")
			fmt.Printf(" the initial admin account.\n")
		}
		fmt.Printf(" Save this now; it is never shown\n")
		fmt.Printf(" again and is NOT recoverable from the database (only its bcrypt\n")
		fmt.Printf(" hash is stored):\n\n")
		fmt.Printf("   username: %s\n", adminConfig.Username)
		fmt.Printf("   password: %s\n", generatedPassword)
		fmt.Printf("\n Change it from Settings after your first login.\n")
		fmt.Printf("================================================================\n\n")
	}

	return nil
}
