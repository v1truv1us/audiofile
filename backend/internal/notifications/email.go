package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ResendSender struct {
	apiKey  string
	from    string
	baseURL string
	client  *http.Client
}

func NewResendSender(apiKey, from string) *ResendSender {
	return &ResendSender{
		apiKey:  apiKey,
		from:    from,
		baseURL: "https://api.resend.com",
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

type resendEmailRequest struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

func (s *ResendSender) Send(ctx context.Context, to, subject, htmlBody string) error {
	payload, err := json.Marshal(resendEmailRequest{
		From:    s.from,
		To:      to,
		Subject: subject,
		HTML:    htmlBody,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/emails", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("resend: unexpected status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

type NoOpSender struct{}

func (NoOpSender) Send(ctx context.Context, to, subject, htmlBody string) error {
	return nil
}
