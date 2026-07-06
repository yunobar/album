package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/itsLeonB/ungerr"
)

type CaptchaService interface {
	Verify(ctx context.Context, token string) error
}

type turnstileService struct {
	secretKey  string
	httpClient *http.Client
	verifyURL  string
}

type noopCaptchaService struct{}

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

func NewTurnstileService(secretKey string) CaptchaService {
	if secretKey == "" {
		return &noopCaptchaService{}
	}
	return &turnstileService{secretKey: secretKey, httpClient: &http.Client{Timeout: 10 * time.Second}, verifyURL: turnstileVerifyURL}
}

func NewTurnstileServiceWithURL(secretKey, verifyURL string) CaptchaService {
	return &turnstileService{secretKey: secretKey, httpClient: &http.Client{Timeout: 10 * time.Second}, verifyURL: verifyURL}
}

func (ts *turnstileService) Verify(ctx context.Context, token string) error {
	form := url.Values{
		"secret":   {ts.secretKey},
		"response": {token},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return ungerr.Wrap(err, "captcha verification failed")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ts.httpClient.Do(req)
	if err != nil {
		return ungerr.Wrap(err, "captcha verification failed")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ungerr.Wrap(fmt.Errorf("unexpected status code: %d", resp.StatusCode), "captcha verification failed")
	}

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ungerr.Wrap(err, "captcha response decode failed")
	}
	if !result.Success {
		return ungerr.BadRequestError("captcha verification failed")
	}
	return nil
}

func (n *noopCaptchaService) Verify(_ context.Context, _ string) error { return nil }
