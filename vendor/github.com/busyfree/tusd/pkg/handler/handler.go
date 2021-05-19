package handler

import (
	"net/http"

	"github.com/bmizerany/pat"
)

// Handler is a ready to use handler with routing (using pat)
type Handler struct {
	*UnroutedHandler
	http.Handler
}

// NewHandler creates a routed tus protocol handler. This is the simplest
// way to use tusd but may not be as configurable as you require. If you are
// integrating this into an existing app you may like to use tusd.NewUnroutedHandler
// instead. Using tusd.NewUnroutedHandler allows the tus handlers to be combined into
// your existing router (aka mux) directly. It also allows the GET and DELETE
// endpoints to be customized. These are not part of the protocol so can be
// changed depending on your needs.
func NewHandler(config Config) (*Handler, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	handler, err := NewUnroutedHandler(config)
	if err != nil {
		return nil, err
	}

	routedHandler := &Handler{
		UnroutedHandler: handler,
	}

	mux := pat.New()

	routedHandler.Handler = handler.Middleware(mux)

	mux.Post("", http.HandlerFunc(handler.PostFile))
	mux.Head(":id", http.HandlerFunc(handler.HeadFile))
	mux.Add("PATCH", ":id", http.HandlerFunc(handler.PatchFile))
	mux.Get(":id", http.HandlerFunc(handler.GetFile))

	// Only attach the DELETE handler if the Terminate() method is provided
	if config.StoreComposer.UsesTerminater {
		mux.Del(":id", http.HandlerFunc(handler.DelFile))
	}

	return routedHandler, nil
}
