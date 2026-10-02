package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	validate "github.com/maahdima/mwp/api/utils/validate"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/service"
)

func openTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:reseller_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	// UserManagerAccount is included because DeleteReseller's service logic
	// also detaches any User Manager accounts owned by the reseller being
	// deleted (mirrors the same detach-on-delete behavior for Peer) -- an
	// out-of-date migration list here was previously masking this test
	// even compiling (see NewReseller's call-site fix in this same
	// commit), so this table was never added when that behavior shipped.
	if err := db.AutoMigrate(&model.Admin{}, &model.Reseller{}, &model.Peer{}, &model.UserManagerAccount{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func TestResellerCRUD(t *testing.T) {
	db := openTestDB(t)

	// nil adaptor: this test never exercises a User Manager group/profile
	// assignment path that would need a live RouterOS round-trip -- see
	// service.NewUserManagerService's test helper for the identical
	// nil-adaptor pattern used elsewhere in this package's test suite.
	resellerService := service.NewReseller(db, nil)
	// nil billing service: this test never exercises the billing-price
	// endpoints, mirroring the nil-adaptor pattern immediately above.
	resellerController := NewResellerController(resellerService, nil)

	e := echo.New()
	e.Validator = &validate.CustomValidator{Validator: validator.New()}
	e.POST("/api/reseller", resellerController.CreateReseller)
	e.GET("/api/reseller", resellerController.ListResellers)
	e.GET("/api/reseller/:id", resellerController.GetReseller)
	e.PUT("/api/reseller/:id", resellerController.UpdateReseller)
	e.DELETE("/api/reseller/:id", resellerController.DeleteReseller)

	// Create
	uniqueUsername := fmt.Sprintf("reseller_test_user_%d", time.Now().UnixNano())
	createBody := map[string]interface{}{
		"name":       "test",
		"username":   uniqueUsername,
		"password":   "StrongPass123",
		"quotaBytes": 1000,
	}
	b, _ := json.Marshal(createBody)
	req := httptest.NewRequest(http.MethodPost, "/api/reseller", bytes.NewReader(b))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	// inject fake jwt token with admin role
	token := &jwt.Token{Claims: jwt.MapClaims{"role": "admin"}}
	ctx.Set("user", token)
	// call handler directly
	if err := resellerController.CreateReseller(ctx); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on create, got %d: %s", rec.Code, rec.Body.String())
	}

	var createResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to parse create response: %v", err)
	}

	data := createResp["data"].(map[string]interface{})
	id := int(data["id"].(float64))

	// List
	req = httptest.NewRequest(http.MethodGet, "/api/reseller", nil)
	rec = httptest.NewRecorder()
	ctx = e.NewContext(req, rec)
	token = &jwt.Token{Claims: jwt.MapClaims{"role": "admin"}}
	ctx.Set("user", token)
	if err := resellerController.ListResellers(ctx); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on list, got %d", rec.Code)
	}

	// Get
	req = httptest.NewRequest(http.MethodGet, "/api/reseller/"+strconv.Itoa(id), nil)
	rec = httptest.NewRecorder()
	ctx = e.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues(strconv.Itoa(id))
	token = &jwt.Token{Claims: jwt.MapClaims{"role": "admin"}}
	ctx.Set("user", token)
	if err := resellerController.GetReseller(ctx); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on get, got %d", rec.Code)
	}

	// Update
	updateBody := map[string]interface{}{"name": "test-updated", "isActive": false}
	b, _ = json.Marshal(updateBody)
	req = httptest.NewRequest(http.MethodPut, "/api/reseller/"+strconv.Itoa(id), bytes.NewReader(b))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	ctx = e.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues(strconv.Itoa(id))
	token = &jwt.Token{Claims: jwt.MapClaims{"role": "admin"}}
	ctx.Set("user", token)
	if err := resellerController.UpdateReseller(ctx); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on update, got %d", rec.Code)
	}

	resellerID := uint(id)
	peer := model.Peer{
		UUID:           "test-peer-uuid",
		PeerID:         "test-peer-id",
		Name:           "test-peer",
		PrivateKey:     "private-key",
		PublicKey:      "public-key",
		Interface:      "wg0",
		AllowedAddress: "10.0.0.2/32",
		ResellerID:     &resellerID,
		Endpoint:       "example.com",
		EndpointPort:   "51820",
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create linked peer: %v", err)
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/api/reseller/"+strconv.Itoa(id), nil)
	rec = httptest.NewRecorder()
	ctx = e.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues(strconv.Itoa(id))
	token = &jwt.Token{Claims: jwt.MapClaims{"role": "admin"}}
	ctx.Set("user", token)
	if err := resellerController.DeleteReseller(ctx); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d", rec.Code)
	}

	var detachedPeer model.Peer
	if err := db.First(&detachedPeer, peer.ID).Error; err != nil {
		t.Fatalf("failed to fetch linked peer after delete: %v", err)
	}
	if detachedPeer.ResellerID != nil {
		t.Fatalf("expected linked peer reseller_id to be cleared, got %v", *detachedPeer.ResellerID)
	}
}
