package server

import (
	"errors"
	"io"
	"net/http"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/payment"
)

const maximumPaymentCallbackBody = 64 << 10

func (s *Server) bepusdtNotify(w http.ResponseWriter, r *http.Request) {
	if s.payments == nil {
		httpx.Error(
			w, httpx.RequestID(r.Context()),
			http.StatusServiceUnavailable, 503, "支付服务未配置",
		)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maximumPaymentCallbackBody)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.Error(
			w, httpx.RequestID(r.Context()),
			http.StatusBadRequest, 400, "支付回调内容无效",
		)
		return
	}
	_, err = s.payments.HandleBEpusdtCallback(r.Context(), raw)
	if err != nil {
		status, code, message := paymentCallbackError(err)
		if status >= http.StatusInternalServerError && s.logger != nil {
			s.logger.Error(
				"process BEpusdt callback",
				"request_id", httpx.RequestID(r.Context()),
				"error", err,
			)
		} else if s.logger != nil {
			s.logger.Warn(
				"reject BEpusdt callback",
				"request_id", httpx.RequestID(r.Context()),
				"remote_addr", r.RemoteAddr,
				"status", status,
				"reason", err,
			)
		}
		httpx.Error(w, httpx.RequestID(r.Context()), status, code, message)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}

func paymentCallbackError(err error) (int, int, string) {
	switch {
	case errors.Is(err, payment.ErrInvalidSignature):
		return http.StatusUnauthorized, 401, "支付回调签名无效"
	case errors.Is(err, payment.ErrInvalidCallback),
		errors.Is(err, payment.ErrUnsupportedStatus):
		return http.StatusBadRequest, 400, "支付回调内容无效"
	case errors.Is(err, payment.ErrOrderNotFound):
		return http.StatusNotFound, 404, "充值订单不存在"
	case errors.Is(err, payment.ErrCallbackConflict),
		errors.Is(err, payment.ErrIdempotencyReuse):
		return http.StatusConflict, 409, "支付回调与订单不一致"
	case errors.Is(err, payment.ErrChannelDisabled),
		errors.Is(err, payment.ErrChannelNotReady):
		return http.StatusServiceUnavailable, 503, "支付通道未配置"
	default:
		return http.StatusInternalServerError, 500, "支付回调处理失败"
	}
}
