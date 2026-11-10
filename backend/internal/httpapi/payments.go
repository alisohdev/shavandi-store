package httpapi

import (
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/shavandi/store/backend/internal/payments"
	"net/http"
	"time"
)

func WithPayments(service *payments.Service, publicOrigin string) HandlerOption {
	return func(options *handlerOptions) { options.payments = service; options.paymentOrigin = publicOrigin }
}
func writePaymentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, payments.ErrInvalid):
		writeError(w, r, 400, "invalid_request", "Invalid payment request")
	case errors.Is(err, payments.ErrNotFound):
		writeError(w, r, 404, "not_found", "Payment not found")
	case errors.Is(err, payments.ErrConflict), errors.Is(err, payments.ErrNotPaid):
		writeError(w, r, 409, "payment_pending", "Payment is not verified; refresh the order")
	default:
		writeError(w, r, 503, "payment_unavailable", "Payment is unavailable; do not repeat a charge")
	}
}
func registerPaymentRoutes(router chi.Router, authentication Authentication, options AuthOptions, service *payments.Service, publicOrigin string) {
	router.Get("/api/v1/payments/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]bool{"enabled": service != nil && service.Gateway != nil, "sandbox": service != nil && service.Environment == "sandbox"})
	})
	router.Get("/api/v1/payments/zarinpal/callback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if service == nil || publicOrigin == "" {
			writePaymentError(w, r, payments.ErrUnavailable)
			return
		}
		if r.URL.Query().Get("Status") == "OK" {
			ctx, cancel := boundedPaymentContext(r.Context())
			defer cancel()
			_ = service.Callback(ctx, r.URL.Query().Get("Authority"))
		}
		http.Redirect(w, r, publicOrigin+"/orders/?payment=returned", http.StatusSeeOther)
	})
	security := &authAPI{service: authentication, options: options}
	router.With(security.browserSecurity, security.boundedContext, security.RequireAuthentication).Post("/api/v1/orders/{id}/payment", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			DeliveryVersion int64 `json:"delivery_version"`
			Total           int64 `json:"expected_total_toman"`
		}
		if !decodeAuthJSON(w, r, &input) {
			return
		}
		session, _ := SessionFromContext(r.Context())
		redirect, err := service.Start(r.Context(), session.User.ID, chi.URLParam(r, "id"), input.DeliveryVersion, input.Total)
		if err != nil {
			writePaymentError(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]string{"redirect_url": redirect})
	})
	router.With(security.browserSecurity, security.boundedContext, security.RequireAuthentication).Post("/api/v1/orders/{id}/payment/verify", func(w http.ResponseWriter, r *http.Request) {
		var input struct{}
		if !decodeAuthJSON(w, r, &input) {
			return
		}
		session, _ := SessionFromContext(r.Context())
		err := service.CheckOwner(r.Context(), session.User.ID, chi.URLParam(r, "id"))
		if err != nil {
			writePaymentError(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "checked"})
	})
}

func boundedPaymentContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 10*time.Second)
}
