package echo

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/ctxerrors"
	httpSwagger "github.com/swaggo/http-swagger"
)

const shutdownTimeout = 10 * time.Second

type Echo struct {
	config      Config
	Echo        *echo.Echo
	RouterGroup *echo.Group
}

func New(
	rootPath string,
	swaggerYAML []byte,
	middlewares []echo.MiddlewareFunc,
) (*Echo, error) {
	cfg, err := parseConfig()
	if err != nil {
		return nil, ctxerrors.Wrap(err, "could not parse config")
	}

	return NewWithConfig(cfg, rootPath, swaggerYAML, middlewares)
}

func NewWithConfig(
	cfg Config,
	rootPath string,
	swaggerYAML []byte,
	middlewares []echo.MiddlewareFunc,
) (*Echo, error) {
	if err := cfg.validate(); err != nil {
		return nil, ctxerrors.Wrap(err, "could not validate config")
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	routerGroup := e.Group(rootPath)

	if cfg.OASPath != "" && len(swaggerYAML) > 0 {
		routerGroup.GET(cfg.OASPath, func(c echo.Context) error {
			return c.Blob(
				http.StatusOK,
				aichteeteapee.ContentTypeYAML,
				swaggerYAML,
			)
		})
	}

	if cfg.SwaggerUIPath != "" && cfg.OASPath != "" {
		routerGroup.GET(
			cfg.SwaggerUIPath,
			echo.WrapHandler(httpSwagger.Handler(
				httpSwagger.URL(
					path.Join(rootPath, cfg.OASPath),
				),
			)),
		)
	}

	for _, middleware := range middlewares {
		routerGroup.Use(middleware)
	}

	return &Echo{
		config:      cfg,
		Echo:        e,
		RouterGroup: routerGroup,
	}, nil
}

func (e *Echo) Start(ctx context.Context) error {
	slog.Debug(
		"starting echo server",
		"address", e.config.ListenAddress,
	)
	defer slog.Debug("echo server stopped")

	errCh := make(chan error, 1)

	go func() {
		if err := e.Echo.Start(e.config.ListenAddress); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			shutdownTimeout,
		)
		defer cancel()

		//nolint:contextcheck // intentional
		if err := e.Echo.Shutdown(shutdownCtx); err != nil {
			return ctxerrors.Wrap(err, "echo server shutdown error")
		}

		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return ctxerrors.Wrap(err, "echo server error")
	}
}
