package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/service"
	validate "github.com/maahdima/mwp/api/utils/validate"
)

// TestWalletCredit_PersianDescription is a live end-to-end reproduction
// attempt for a reported bug: "crediting/debiting a reseller's wallet WITH
// a description/title errors; works fine without one." Drives the actual
// Echo route (WalletController.Credit), not just the service layer, so any
// binding/validation-level rejection of the description field would show
// up here exactly as it would over real HTTP.
func TestWalletCredit_PersianDescription(t *testing.T) {
	dsn := fmt.Sprintf("file:wallet_persian_desc_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Reseller{}, &model.Wallet{}, &model.LedgerEntry{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	reseller := model.Reseller{Name: "test", Username: "wallet-desc-test"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	walletService := service.NewWallet(db)
	resellerService := service.NewReseller(db, nil)
	walletController := NewWalletController(walletService, resellerService, nil)

	e := echo.New()
	e.Validator = &validate.CustomValidator{Validator: validator.New()}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("user", &jwt.Token{Claims: jwt.MapClaims{"role": "admin"}})
			return next(c)
		}
	})
	e.POST("/api/wallet/reseller/:reseller_id/credit", walletController.Credit)

	descriptions := []string{
		"شارژ اولیه",
		"شارژ به مناسبت تخفیف ویژه با کاراکترهای خاص !@#$%^&*()",
		"یک توضیح خیلی خیلی خیلی خیلی خیلی خیلی خیلی خیلی خیلی خیلی خیلی خیلی طولانی فارسی برای تست محدودیت طول احتمالی رشته که شاید سرور آن را رد کند و ما اینجا داریم بررسی می‌کنیم که آیا واقعا چنین اتفاقی می‌افتد یا نه",
		"", // empty description, baseline
	}

	for i, desc := range descriptions {
		t.Run(fmt.Sprintf("description_case_%d", i), func(t *testing.T) {
			body := map[string]interface{}{
				"amount":      1000,
				"description": desc,
			}
			b, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/wallet/reseller/%d/credit", reseller.ID), bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("expected 201 Created for description %q, got %d: %s", desc, rec.Code, rec.Body.String())
			}
		})
	}
}
