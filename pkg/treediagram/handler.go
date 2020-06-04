package treediagram

import (
	"context"
	"net/http"
	"time"

	"github.com/jukeizu/contract"
	"github.com/jukeizu/management/pkg/management"
	"github.com/rs/zerolog"
)

const AppId = "intent.endpoint.management"

type Handler struct {
	logger     zerolog.Logger
	service    management.Service
	httpServer *http.Server
}

func NewHandler(logger zerolog.Logger, addr string, service management.Service) Handler {
	logger = logger.With().Str("component", AppId).Logger()

	httpServer := http.Server{
		Addr: addr,
	}

	return Handler{logger, service, &httpServer}
}

func (h Handler) Clean(request contract.Request) (*contract.Response, error) {
	err := h.service.ValidatePermissions(request.Author.Id, request.ChannelId)
	if err != nil {
		return FormatError(err)
	}

	go func() {
		err := h.service.Clean(request.Author.Id, request.ChannelId)
		if err != nil {
			h.logger.Error().Err(err).Msg("failed to clean")
		}
	}()

	reaction := &contract.Reaction{
		MessageId: request.Id,
		ChannelId: request.ChannelId,
		EmojiId:   "🧹",
	}

	return &contract.Response{Reactions: []*contract.Reaction{reaction}}, nil
}

func (h Handler) Start() error {
	h.logger.Info().Msg("starting")

	mux := http.NewServeMux()
	mux.HandleFunc("/clean", h.makeLoggingHttpHandlerFunc("clean", h.Clean))

	h.httpServer.Handler = mux

	return h.httpServer.ListenAndServe()
}

func (h Handler) Stop() error {
	h.logger.Info().Msg("stopping")

	return h.httpServer.Shutdown(context.Background())
}

func (h Handler) makeLoggingHttpHandlerFunc(name string, f func(contract.Request) (*contract.Response, error)) http.HandlerFunc {
	contractHandlerFunc := contract.MakeRequestHttpHandlerFunc(f)

	return func(w http.ResponseWriter, r *http.Request) {
		defer func(begin time.Time) {
			h.logger.Info().
				Str("intent", name).
				Str("took", time.Since(begin).String()).
				Msg("called")
		}(time.Now())

		contractHandlerFunc.ServeHTTP(w, r)
	}
}
