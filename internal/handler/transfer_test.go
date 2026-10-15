package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	apperrors "github.com/ventolin13/challenge3/errors"
	"github.com/ventolin13/challenge3/internal/handler"
	"github.com/ventolin13/challenge3/internal/services"
	"github.com/ventolin13/challenge3/internal/services/structures"
	servicemocks "github.com/ventolin13/challenge3/mocks/internal_/services"
)

const transferBody = `{"idempotency_key":"6ba7b810-9dad-11d1-80b4-00c04fd430c8","from_account_id":"acc_1","to_account_id":"acc_2","amount":1}`

func TestPostTransferWritesServiceError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name string
		err  error
		want apperrors.Entry
	}{
		{name: apperrors.ErrIdempotencyConflict.Code, err: services.ErrIdempotencyConflict, want: apperrors.ErrIdempotencyConflict},
		{name: apperrors.ErrTransferTransactionConflict.Code, err: services.ErrTransactionConflict, want: apperrors.ErrTransferTransactionConflict},
		{name: apperrors.ErrTransferIDConflict.Code, err: services.ErrTransferIDExists, want: apperrors.ErrTransferIDConflict},
		{name: apperrors.ErrInsufficientFunds.Code, err: services.ErrInsufficientFunds, want: apperrors.ErrInsufficientFunds},
		{name: apperrors.ErrSelfTransfer.Code, err: services.ErrSelfTransfer, want: apperrors.ErrSelfTransfer},
		{name: apperrors.ErrAccountNotFound.Code, err: services.ErrAccountNotFound, want: apperrors.ErrAccountNotFound},
		{name: apperrors.ErrAccountMissing.Code, err: services.ErrAccountMissing, want: apperrors.ErrAccountMissing},
		{name: apperrors.ErrTransferMissing.Code, err: services.ErrTransferMissing, want: apperrors.ErrTransferMissing},
		{name: apperrors.ErrDatabaseUnavailable.Code, err: services.ErrUnavailable, want: apperrors.ErrDatabaseUnavailable},
		{name: "unknown", err: errors.New("dial tcp"), want: apperrors.ErrDatabaseUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			service := servicemocks.NewMockTransferService(t)
			var noTransfer structures.Transfer
			service.EXPECT().PostTransfer(mock.Anything, mock.Anything).Return(noTransfer, tc.err)
			engine := gin.New()
			api := handler.NewTransfers(service)
			engine.POST("/api/v1/transfers", api.PostTransfer)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/transfers", strings.NewReader(transferBody))
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			require.Equal(t, tc.want.StatusCode, rec.Code)
			var body apperrors.Response
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, tc.want.Body(), body)
		})
	}
}
