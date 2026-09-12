package types

import "net/http"

// AppError carries an HTTP status alongside a message so adapters can map
// domain/use-case failures onto transport responses.
type AppError struct {
	Code    int    `json:"-"`
	Message string `json:"error"`
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) HTTPStatus() int {
	return e.Code
}

func NewBadRequestError(msg string) *AppError {
	return &AppError{Code: http.StatusBadRequest, Message: msg}
}

func NewUnauthorizedError(msg string) *AppError {
	return &AppError{Code: http.StatusUnauthorized, Message: msg}
}

func NewForbiddenError(msg string) *AppError {
	return &AppError{Code: http.StatusForbidden, Message: msg}
}

func NewNotFoundError(msg string) *AppError {
	return &AppError{Code: http.StatusNotFound, Message: msg}
}

func NewMethodNotAllowedError(msg string) *AppError {
	return &AppError{Code: http.StatusMethodNotAllowed, Message: msg}
}

func NewInternalError(msg string) *AppError {
	return &AppError{Code: http.StatusInternalServerError, Message: msg}
}

func NewBadGatewayError(msg string) *AppError {
	return &AppError{Code: http.StatusBadGateway, Message: msg}
}

func NewGatewayTimeoutError(msg string) *AppError {
	return &AppError{Code: http.StatusGatewayTimeout, Message: msg}
}
