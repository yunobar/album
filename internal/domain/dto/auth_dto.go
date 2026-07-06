package dto

import "github.com/google/uuid"

type RegisterRequest struct {
	Email                string `json:"email" binding:"required,email,min=3"`
	Password             string `json:"password" binding:"required,eqfield=PasswordConfirmation"`
	PasswordConfirmation string `json:"passwordConfirmation" binding:"required"`
	Slug                 string `json:"slug"`
}

type InternalLoginRequest struct {
	Email    string `json:"email" binding:"required,email,min=3"`
	Password string `json:"password" binding:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type TokenResponse struct {
	Type         string `json:"type"`
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
	Fingerprint  string `json:"-"`
}

type RegisterResponse struct {
	Message string `json:"message"`
}

type SendPasswordResetRequest struct {
	Email        string `json:"email" binding:"required,email,min=3"`
	CaptchaToken string `json:"captchaToken" binding:"required"`
}

type ResetPasswordRequest struct {
	Token                string `json:"token" binding:"required,min=3"`
	Password             string `json:"password" binding:"required,eqfield=PasswordConfirmation"`
	PasswordConfirmation string `json:"passwordConfirmation" binding:"required"`
}

type OAuthCallbackData struct {
	Provider string `validate:"required,min=1"`
	Code     string `validate:"required,min=1"`
	State    string `validate:"required,min=1"`
}

func NewTokenResp(token, refreshToken, fingerprint string) TokenResponse {
	return TokenResponse{
		Type:         "Bearer",
		Token:        token,
		RefreshToken: refreshToken,
		Fingerprint:  fingerprint,
	}
}

type AdminMe struct {
	ID       uuid.UUID `json:"id"`
	FullName string    `json:"fullName"`
}
