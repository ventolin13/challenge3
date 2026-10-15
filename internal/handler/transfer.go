// Package handler serves the REST API.
package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	apperrors "github.com/ventolin13/challenge3/errors"
	"github.com/ventolin13/challenge3/internal/handler/request"
	"github.com/ventolin13/challenge3/internal/handler/response"
	"github.com/ventolin13/challenge3/internal/services"
	"github.com/ventolin13/challenge3/internal/services/structures"
)

var (
	noEntry   apperrors.Entry
	noCommand structures.TransferCommand
)

// Transfers is the HTTP API.
type Transfers struct {
	service services.TransferService
}

// NewTransfers builds handlers.
func NewTransfers(service services.TransferService) *Transfers {
	return &Transfers{service: service}
}

type wireTransfer struct {
	TransferID    *string      `json:"idempotency_key"`
	FromAccountID *string      `json:"from_account_id"`
	ToAccountID   *string      `json:"to_account_id"`
	Amount        *json.Number `json:"amount"`
}

// PostTransfer moves coins from one account to another.
// A repeat with the same idempotency_key and the same body returns the stored outcome and does not move coins again.
// A stored insufficient funds outcome is returned as 422. A different body for the same key is 409.
// self-transfer not allowed and account not found are not stored.
// The balance decision runs in one REPEATABLE READ transaction. The service does not retry it.
// Serialization and deadlock roll the transaction back and answer 409 transfer transaction rolled back, retry the request.
// A taken transfer_id answers 409 transfer_id already exists.
// A failed Postgres connection answers 500 postgres connection failed.
//
// @Summary     Перевести монеты
// @Description Обязательные поля тела: idempotency_key, from_account_id, to_account_id, amount.
// @Description Списывает amount с from_account_id и зачисляет ту же сумму на to_account_id.
// @Description idempotency_key — UUID клиента, он же transfer_id.
// @Description Повтор того же ключа и тела возвращает записанный исход.
// @Description Другое тело — 409 idempotency key reused with different payload.
// @Description Откат транзакции — 409 transfer transaction rolled back, retry the request.
// @Description Занятый transfer_id — 409 transfer_id already exists.
// @Description Нет соединения с Postgres — 500 postgres connection failed.
// @Tags        transfers
// @Accept      json
// @Produce     json
// @Param       data body request.TransferRequest true "idempotency_key, from_account_id, to_account_id, amount"
// @Success     200 {object} response.TransferResponse "перевод исполнен"
// @Failure     400 {object} apperrors.Response "тело не прошло проверку"
// @Failure     409 {object} apperrors.Response "конфликт ключа или откат транзакции"
// @Failure     422 {object} apperrors.Response "перевод отклонён"
// @Failure     500 {object} apperrors.Response "нет соединения с Postgres"
// @Router      /api/v1/transfers [post]
func (h *Transfers) PostTransfer(c *gin.Context) {
	cmd, entry, ok := readCommand(c)
	if !ok {
		c.JSON(entry.StatusCode, entry.Body())
		return
	}
	row, err := h.service.PostTransfer(c.Request.Context(), cmd)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewTransfer(&row, http.StatusOK))
}

// GetAccount returns account_id and balance.
// An unknown id answers 404 account not found. That is the same reason as a rejected transfer, with HTTP 404.
//
// @Summary     Баланс аккаунта
// @Description Возвращает account_id и balance числом. Неизвестный id — 404 account not found.
// @Tags        accounts
// @Produce     json
// @Param       id path string true "идентификатор аккаунта"
// @Success     200 {object} response.AccountResponse "аккаунт найден"
// @Failure     404 {object} apperrors.Response "аккаунта нет"
// @Failure     500 {object} apperrors.Response "нет соединения с Postgres"
// @Router      /api/v1/accounts/{id} [get]
func (h *Transfers) GetAccount(c *gin.Context) { //nolint:revive // gin writes the response; the name matches the route
	row, err := h.service.GetAccount(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewAccount(row))
}

// GetTransfer reads a stored transfer by idempotency_key.
// A completed row has status completed. A stored insufficient funds row has status rejected, reason insufficient funds, and status_code 200.
// A key that is not a UUID answers 400. A missing row answers 404.
//
// @Summary     Перевод по ключу идемпотентности
// @Description Читает transfers по idempotency_key. 200 для исполненного перевода и для записанной нехватки средств. Путь не UUID — 400. Строки нет — 404.
// @Tags        transfers
// @Produce     json
// @Param       idempotency_key path string true "UUID ключа идемпотентности"
// @Success     200 {object} response.TransferResponse "строка перевода найдена"
// @Failure     400 {object} apperrors.Response "ключ не UUID"
// @Failure     404 {object} apperrors.Response "строки нет"
// @Failure     500 {object} apperrors.Response "нет соединения с Postgres"
// @Router      /api/v1/transfers/{idempotency_key} [get]
func (h *Transfers) GetTransfer(c *gin.Context) { //nolint:revive // gin writes the response; the name matches the route
	transferID, err := uuid.Parse(c.Param("idempotency_key"))
	if err != nil {
		c.JSON(apperrors.ErrInvalidTransferID.StatusCode, apperrors.ErrInvalidTransferID.Body())
		return
	}
	row, err := h.service.GetTransfer(c.Request.Context(), transferID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewTransfer(&row, http.StatusOK))
}

func readCommand(c *gin.Context) (structures.TransferCommand, apperrors.Entry, bool) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return noCommand, apperrors.ErrInvalidJSON, false
	}
	var body wireTransfer
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err = decoder.Decode(&body); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field == "amount" {
			return noCommand, apperrors.ErrInvalidAmount, false
		}
		return noCommand, apperrors.ErrInvalidJSON, false
	}
	transferID, entry, ok := requiredUUID(body.TransferID)
	if !ok {
		return noCommand, entry, false
	}
	from, entry, ok := requiredAccount(body.FromAccountID, apperrors.ErrInvalidFromAccountID)
	if !ok {
		return noCommand, entry, false
	}
	to, entry, ok := requiredAccount(body.ToAccountID, apperrors.ErrInvalidToAccountID)
	if !ok {
		return noCommand, entry, false
	}
	if body.Amount == nil {
		return noCommand, apperrors.ErrInvalidAmount, false
	}
	amount, valid := request.ParseAmount(body.Amount.String())
	if !valid {
		return noCommand, apperrors.ErrInvalidAmount, false
	}
	return structures.TransferCommand{
		TransferID:    transferID,
		FromAccountID: from,
		ToAccountID:   to,
		Amount:        amount,
	}, noEntry, true
}

func requiredUUID(raw *string) (uuid.UUID, apperrors.Entry, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return uuid.UUID{}, apperrors.ErrInvalidTransferID, false
	}
	id, err := uuid.Parse(strings.TrimSpace(*raw))
	if err != nil {
		return uuid.UUID{}, apperrors.ErrInvalidTransferID, false
	}
	return id, noEntry, true
}

func requiredAccount(raw *string, entry apperrors.Entry) (string, apperrors.Entry, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return "", entry, false
	}
	return strings.TrimSpace(*raw), noEntry, true
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrIdempotencyConflict):
		c.JSON(apperrors.ErrIdempotencyConflict.StatusCode, apperrors.ErrIdempotencyConflict.Body())
	case errors.Is(err, services.ErrTransactionConflict):
		c.JSON(apperrors.ErrTransferTransactionConflict.StatusCode, apperrors.ErrTransferTransactionConflict.Body())
	case errors.Is(err, services.ErrTransferIDExists):
		c.JSON(apperrors.ErrTransferIDConflict.StatusCode, apperrors.ErrTransferIDConflict.Body())
	case errors.Is(err, services.ErrInsufficientFunds):
		c.JSON(apperrors.ErrInsufficientFunds.StatusCode, apperrors.ErrInsufficientFunds.Body())
	case errors.Is(err, services.ErrSelfTransfer):
		c.JSON(apperrors.ErrSelfTransfer.StatusCode, apperrors.ErrSelfTransfer.Body())
	case errors.Is(err, services.ErrAccountNotFound):
		c.JSON(apperrors.ErrAccountNotFound.StatusCode, apperrors.ErrAccountNotFound.Body())
	case errors.Is(err, services.ErrAccountMissing):
		c.JSON(apperrors.ErrAccountMissing.StatusCode, apperrors.ErrAccountMissing.Body())
	case errors.Is(err, services.ErrTransferMissing):
		c.JSON(apperrors.ErrTransferMissing.StatusCode, apperrors.ErrTransferMissing.Body())
	default:
		c.JSON(apperrors.ErrDatabaseUnavailable.StatusCode, apperrors.ErrDatabaseUnavailable.Body())
	}
}
