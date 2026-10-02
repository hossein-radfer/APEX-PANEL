package http

import (
	"errors"
	"io"
	"io/fs"
	gohttp "net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type UIController struct {
	uiAssetsFs             fs.FS
	staticDirectoryHandler echo.HandlerFunc
}

func NewUiController(fs fs.FS) *UIController {
	return &UIController{
		uiAssetsFs:             fs,
		staticDirectoryHandler: echo.StaticDirectoryHandler(fs, false),
	}
}

func (c *UIController) Serve(ctx echo.Context) error {
	// Hashed build assets (Vite content-hashes every JS/CSS filename) get
	// a new URL on every build, so caching them aggressively is safe.
	if strings.HasPrefix(ctx.Request().URL.Path, "/assets/") {
		ctx.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}

	if err := c.staticDirectoryHandler(ctx); err != nil {
		f, err := c.uiAssetsFs.Open("index.html")
		if err != nil {
			return echo.ErrNotFound
		}
		defer f.Close()

		// index.html must always be revalidated -- it's what points the
		// browser at the current build's hashed asset filenames. Without
		// an explicit no-cache directive, a browser's heuristic caching
		// can keep serving a stale copy of this exact file indefinitely,
		// which looks exactly like "I deployed a new build but the UI
		// never changed" even though the server-side files genuinely did.
		ctx.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		// fi's error was previously discarded (`fi, _ := f.Stat()`), which
		// left fi nil on failure -- the two calls below (fi.Name(),
		// fi.ModTime()) would then panic on a nil pointer dereference.
		fi, err := f.Stat()
		if err != nil {
			return echo.ErrNotFound
		}
		ff, ok := f.(io.ReadSeeker)
		if !ok {
			return errors.New("file does not implement io.ReadSeeker")
		}
		gohttp.ServeContent(ctx.Response(), ctx.Request(), fi.Name(), fi.ModTime(), ff)
		return nil
	}

	return nil
}

func SetupMwpUI(app *echo.Echo, uiAssetsFs fs.FS) {
	app.GET("/*", NewUiController(uiAssetsFs).Serve)
}
