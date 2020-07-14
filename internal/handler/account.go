package handler

import (
	"net/http"

	"github.com/go-chi/jwtauth"
	"github.com/go-chi/render"
	"github.com/plumlab/go-chi-sample/internal/model"
	"github.com/plumlab/go-chi-sample/internal/service"
)

// Account handler all http request related to account
type Account struct {
	account service.AccountService
}

// NewAccount create new account handler
func NewAccount(account service.AccountService) *Account {
	return &Account{
		account: account,
	}
}

func (a *Account) Login(w http.ResponseWriter, r *http.Request) {
	login := &LoginRequest{}
	if err := render.Bind(r, login); err != nil {
		responseError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	user, err := a.account.Login(r.Context(), login.Email, login.Password)
	if err != nil {
		responseError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	responseData(w, r, user)
}

func (a *Account) Register(w http.ResponseWriter, r *http.Request) {
	registar := &RegisterRequest{}
	if err := render.Bind(r, registar); err != nil {
		responseError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	user := &model.User{
		Firstname:       registar.Firstname,
		Lastname:        registar.Lastname,
		Email:           registar.Email,
		Password:        registar.Password,
		ConfirmPassword: registar.ConfirmPassword,
	}

	user, err := a.account.Register(r.Context(), user)
	if err != nil {
		responseError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	responseData(w, r, user)
}

func (a *Account) Logout(w http.ResponseWriter, r *http.Request) {
	_, claims, err := jwtauth.FromContext(r.Context())
	if err != nil {
		responseError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if email, ok := claims["email"].(string); ok {
		err := a.account.Logout(r.Context(), email)
		if err != nil {
			responseError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		responseData(w, r, nil)
	} else {
		responseError(w, r, http.StatusUnauthorized, "authorization token not found or invalid")
	}
}

func (a *Account) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	forgot := &ForgotPasswordRequest{}
	if err := render.Bind(r, forgot); err != nil {
		responseError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	_, err := a.account.PasswordResetToken(r.Context(), forgot.Email)
	if err != nil {
		responseError(w, r, http.StatusNotFound, err.Error())
		return
	}
	responseData(w, r, nil)
}

func (a *Account) ResetPassword(w http.ResponseWriter, r *http.Request) {
	reset := &ResetPasswordRequest{}
	if err := render.Bind(r, reset); err != nil {
		responseError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	user, err := a.account.ResetPassword(r.Context(), reset.Token, reset.Password)
	if err != nil {
		responseError(w, r, http.StatusNotFound, err.Error())
		return
	}
	responseData(w, r, user)
}

func (a *Account) VerifyPasswordResetToken(w http.ResponseWriter, r *http.Request) {
	reset := &PasswordResetTokenRequest{}
	if err := render.Bind(r, reset); err != nil {
		responseError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	email, err := a.account.VerifyToken(r.Context(), reset.Token)
	if err != nil {
		responseError(w, r, http.StatusNotFound, err.Error())
		return
	}
	responseData(w, r, email)
}

func (a *Account) VerifyEmailToken(w http.ResponseWriter, r *http.Request) {
	verify := &VerifyEmailTokenRequest{}
	if err := render.Bind(r, verify); err != nil {
		responseError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	user, err := a.account.VerifyEmailToken(r.Context(), verify.Token)
	if err != nil {
		responseError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	responseData(w, r, user)
}
