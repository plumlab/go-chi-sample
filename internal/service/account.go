package service

import (
	"context"
	"time"

	"github.com/dchest/passwordreset"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/plumlab/go-chi-sample/internal/model"
	"github.com/plumlab/go-chi-sample/internal/repo"
)

// AccountService account service interface
type AccountService interface {
	Login(ctx context.Context, email, password string) (*model.User, error)
	Logout(ctx context.Context, email string) error
	Register(ctx context.Context, user *model.User) (*model.User, error)
	PasswordResetToken(ctx context.Context, email string) (string, error)
	VerifyToken(ctx context.Context, token string) (string, error)
	VerifyEmailToken(ctx context.Context, token string) (*model.User, error)
	ResetPassword(ctx context.Context, token, password string) (*model.User, error)
}

// Account service
type Account struct {
	repo repo.UserRepo
}

// NewAccount create new account service
func NewAccount(repo repo.UserRepo) *Account {
	return &Account{
		repo: repo,
	}
}

// Login check account login and generate user token
func (a *Account) Login(ctx context.Context, email, password string) (*model.User, error) {
	var user *model.User
	user, err := a.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if !user.EmailVerification {
		return nil, errors.New("unverified account")
	}
	if !comparePasswords(user.Password, []byte(password)) {
		return nil, errors.New("passwords are not match")
	}
	token, err := generateToken(email, user.Password)
	if err != nil {
		return nil, err
	}
	user.Token = token
	usr, err := a.repo.Update(ctx, user)
	if err != nil {
		return nil, err
	}
	return usr, nil
}

func (a *Account) Logout(ctx context.Context, email string) error {
	user, err := a.repo.GetByEmail(ctx, email)
	if err != nil {
		return err
	}
	user.Token = ""
	_, err = a.repo.Update(ctx, user)
	if err != nil {
		return err
	}
	return nil
}

func (a *Account) getPasswordHash(email string) ([]byte, error) {
	user, err := a.repo.GetByEmail(context.Background(), email)
	if err != nil {
		return nil, err
	}
	return []byte(user.Password), nil
}

func (a *Account) PasswordResetToken(ctx context.Context, email string) (string, error) {
	user, err := a.repo.GetByEmail(context.Background(), email)
	if err != nil {
		return "", err
	}
	token := passwordreset.NewToken(email, 24*time.Hour, []byte(user.Password), signingKey)

	go sendMail("d-1036306a05ee4829a6799879ec19051c", []interface{}{user, token}, createForgotPasswordEmailFromTemplate)
	return token, nil
}

func (a *Account) VerifyToken(ctx context.Context, token string) (string, error) {
	email, err := passwordreset.VerifyToken(token, a.getPasswordHash, signingKey)
	if err != nil {
		return "", err
	}
	return email, nil
}

func (a *Account) VerifyEmailToken(ctx context.Context, token string) (*model.User, error) {
	email, err := passwordreset.VerifyToken(token, a.getPasswordHash, signingKey)
	if err != nil {
		return nil, err
	}
	user, err := a.repo.GetByEmail(context.Background(), email)
	if err != nil {
		return nil, err
	}
	user.EmailVerification = true
	usr, err := a.repo.Update(ctx, user)
	if err != nil {
		return nil, err
	}
	return usr, nil
}

func (a *Account) ResetPassword(ctx context.Context, token, password string) (*model.User, error) {
	email, err := passwordreset.VerifyToken(token, a.getPasswordHash, signingKey)
	if err != nil {
		return nil, errors.Wrap(err, "token is invalid or expired")
	}
	var user *model.User
	user, err = a.repo.GetByEmail(context.Background(), email)
	if err != nil {
		return nil, errors.Wrap(err, "user does not exist")
	}
	user.Password = hashAndSalt([]byte(password))
	usr, err := a.repo.Update(ctx, user)
	if err != nil {
		return nil, errors.Wrap(err, "cannot change user password")
	}
	return usr, nil
}

// Register create new account
func (a *Account) Register(ctx context.Context, user *model.User) (*model.User, error) {
	users, err := a.repo.FindByEmail(ctx, user.Email)
	if err != nil {
		return nil, err
	}
	if len(users) > 0 {
		return nil, errors.New("email is already registered")
	}
	if user.Password != user.ConfirmPassword {
		return nil, errors.New("password and confirm password are not matched")
	}
	usr := &model.User{
		ID:        uuid.New().String(),
		Firstname: user.Firstname,
		Lastname:  user.Lastname,
		Email:     user.Email,
		Password:  hashAndSalt([]byte(user.Password)),
	}

	user, err = a.repo.Create(ctx, usr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot register new user")
	}
	token := passwordreset.NewToken(user.Email, 48*time.Hour, []byte(user.Password), signingKey)
	go sendMail("d-c2d8108d7529490889ec91151b4471db", []interface{}{user, token}, createVerifyEmailFromTemplate)

	return user, nil
}
