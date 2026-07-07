package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/itsLeonB/ungerr"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/core/logger"
)

type TurnstileClient interface {
	Verify(ctx context.Context, token string) error
}

type turnstileClient struct {
	secretKey  string
	httpClient *http.Client
	verifyURL  string
}

type noopTurnstileClient struct{}

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

func NewTurnstileClient(secretKey string) TurnstileClient {
	if secretKey == "" {
		return &noopTurnstileClient{}
	}
	return &turnstileClient{secretKey: secretKey, httpClient: &http.Client{Timeout: 10 * time.Second}, verifyURL: turnstileVerifyURL}
}

func NewTurnstileClientWithURL(secretKey, verifyURL string) TurnstileClient {
	return &turnstileClient{secretKey: secretKey, httpClient: &http.Client{Timeout: 10 * time.Second}, verifyURL: verifyURL}
}

func (ts *turnstileClient) Verify(ctx context.Context, token string) error {
	form := url.Values{
		"secret":   {ts.secretKey},
		"response": {token},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return ungerr.Wrap(err, appconstant.ErrCaptchaFailed)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ts.httpClient.Do(req)
	if err != nil {
		return ungerr.Wrap(err, appconstant.ErrCaptchaFailed)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ungerr.Wrap(fmt.Errorf("unexpected status code: %d", resp.StatusCode), appconstant.ErrCaptchaFailed)
	}

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ungerr.Wrap(err, "captcha response decode failed")
	}
	if !result.Success {
		return ungerr.BadRequestError(appconstant.ErrCaptchaFailed)
	}
	return nil
}

func (n *noopTurnstileClient) Verify(_ context.Context, _ string) error {
	logger.Warn("captcha verification is disabled, set a valid turnstile key")
	return nil
}
