package cli

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"relwatch/internal/config"
	"relwatch/internal/notify"
)

type stubSender struct {
	sent []notify.Message
	err  error
}

func (s *stubSender) Send(ctx context.Context, msg notify.Message) error {
	s.sent = append(s.sent, msg)
	return s.err
}

func stubFactory(s *stubSender) func(config.SMTPConfig) (notify.Sender, error) {
	return func(config.SMTPConfig) (notify.Sender, error) { return s, nil }
}

func testSMTPConfig() config.SMTPConfig {
	return config.SMTPConfig{
		Enabled:  true,
		Host:     "smtp.example.com",
		Port:     587,
		Username: "user",
		Password: "s3cret-pw",
		From:     "relwatch@example.com",
		To:       []string{"a@example.com", "b@example.com"},
	}
}

func TestSendTestEmailUsesConfiguredRecipients(t *testing.T) {
	s := &stubSender{}
	var out, errOut bytes.Buffer
	if err := sendTestEmail(context.Background(), testSMTPConfig(), nil, "/etc/relwatch/settings.conf", &out, &errOut, stubFactory(s)); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(s.sent))
	}
	msg := s.sent[0]
	if !reflect.DeepEqual(msg.To, []string{"a@example.com", "b@example.com"}) {
		t.Fatalf("to = %v", msg.To)
	}
	if msg.Subject != "relwatch: SMTP test" {
		t.Fatalf("subject = %q", msg.Subject)
	}
	if !strings.Contains(msg.Body, "smtp.example.com:587") || !strings.Contains(msg.Body, "/etc/relwatch/settings.conf") {
		t.Fatalf("body missing server/config: %q", msg.Body)
	}
	want := "sent test email to a@example.com, b@example.com via smtp.example.com:587 (config: /etc/relwatch/settings.conf)\n"
	if out.String() != want {
		t.Fatalf("out = %q, want %q", out.String(), want)
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", errOut.String())
	}
}

func TestSendTestEmailToOverridesConfig(t *testing.T) {
	s := &stubSender{}
	var out, errOut bytes.Buffer
	if err := sendTestEmail(context.Background(), testSMTPConfig(), []string{"c@example.com"}, "x", &out, &errOut, stubFactory(s)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.sent[0].To, []string{"c@example.com"}) {
		t.Fatalf("to = %v", s.sent[0].To)
	}
}

func TestSendTestEmailNoRecipients(t *testing.T) {
	cfg := testSMTPConfig()
	cfg.To = nil
	called := false
	factory := func(config.SMTPConfig) (notify.Sender, error) {
		called = true
		return &stubSender{}, nil
	}
	var out, errOut bytes.Buffer
	err := sendTestEmail(context.Background(), cfg, nil, "x", &out, &errOut, factory)
	if err == nil || !strings.Contains(err.Error(), "no recipients") {
		t.Fatalf("err = %v", err)
	}
	if called {
		t.Fatal("sender constructed despite missing recipients")
	}
}

func TestSendTestEmailDisabledStillSends(t *testing.T) {
	cfg := testSMTPConfig()
	cfg.Enabled = false
	s := &stubSender{}
	var out, errOut bytes.Buffer
	if err := sendTestEmail(context.Background(), cfg, nil, "x", &out, &errOut, stubFactory(s)); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(s.sent))
	}
	if !strings.Contains(errOut.String(), "smtp.enabled is false") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestSendTestEmailErrors(t *testing.T) {
	sendErr := errors.New("smtp: auth: 535 bad credentials")
	ctorErr := errors.New("smtp: host is required")
	cases := []struct {
		name    string
		factory func(config.SMTPConfig) (notify.Sender, error)
		want    error
	}{
		{"send", stubFactory(&stubSender{err: sendErr}), sendErr},
		{"constructor", func(config.SMTPConfig) (notify.Sender, error) { return nil, ctorErr }, ctorErr},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			err := sendTestEmail(context.Background(), testSMTPConfig(), nil, "x", &out, &errOut, tc.factory)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want wrapping %v", err, tc.want)
			}
			if out.Len() != 0 {
				t.Fatalf("unexpected success output: %q", out.String())
			}
		})
	}
}

func TestSendTestEmailNeverLeaksPassword(t *testing.T) {
	s := &stubSender{}
	var out, errOut bytes.Buffer
	cfg := testSMTPConfig()
	cfg.Enabled = false
	if err := sendTestEmail(context.Background(), cfg, nil, "x", &out, &errOut, stubFactory(s)); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"stdout": out.String(), "stderr": errOut.String(), "body": s.sent[0].Body} {
		if strings.Contains(text, cfg.Password) {
			t.Fatalf("%s contains password: %q", name, text)
		}
	}
}

func TestSMTPServerLabel(t *testing.T) {
	if got := smtpServerLabel(config.SMTPConfig{Host: "mx.example.com"}); got != "mx.example.com (default port)" {
		t.Fatalf("got %q", got)
	}
	if got := smtpServerLabel(config.SMTPConfig{Host: "mx.example.com", Port: 465}); got != "mx.example.com:465" {
		t.Fatalf("got %q", got)
	}
}
