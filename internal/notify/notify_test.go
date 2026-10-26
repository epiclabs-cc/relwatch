package notify

import (
	"context"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"relwatch/internal/config"
)

func TestDigestSubject(t *testing.T) {
	msg := Digest(nil)
	if msg.Subject != "relwatch: 0 new releases" {
		t.Fatalf("got %q", msg.Subject)
	}
	if msg.Body != "" {
		t.Fatalf("got body %q, want empty", msg.Body)
	}

	one := Digest([]NewRelease{{ProjectName: "gh", Provider: "github", Repo: "cli/cli", Tag: "v2.0.0"}})
	if one.Subject != "relwatch: 1 new release" {
		t.Fatalf("got %q", one.Subject)
	}

	multi := Digest([]NewRelease{
		{ProjectName: "a", Provider: "github", Repo: "x/y", Tag: "v1"},
		{ProjectName: "b", Provider: "gitlab", Repo: "w/z", Tag: "v2"},
	})
	if multi.Subject != "relwatch: 2 new releases" {
		t.Fatalf("got %q", multi.Subject)
	}
}

func TestDigestBody(t *testing.T) {
	pub := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	msg := Digest([]NewRelease{
		{ProjectName: "gh", Provider: "github", Repo: "cli/cli", Tag: "v2.0.0", Name: "v2.0.0", URL: "https://github.com/cli/cli/releases/v2.0.0", PublishedAt: pub},
	})
	for _, want := range []string{"gh - github/cli/cli: v2.0.0", "on 2026-09-13", "https://github.com/cli/cli/releases/v2.0.0"} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("body missing %q:\n%s", want, msg.Body)
		}
	}
}

func TestBuildMessageSanitizesHeaders(t *testing.T) {
	msg := Message{
		To:      []string{"a@b.c\r\nBcc: evil@x"},
		Subject: "hello\r\nBcc: evil@x.x",
	}
	raw := string(buildMessage("from@y.z", msg))
	if strings.Contains(raw, "\nBcc: evil") || strings.Contains(raw, "\r\nBcc: evil") || strings.HasPrefix(raw, "Bcc: evil\r\n") {
		t.Fatalf("header injection present:\n%s", raw)
	}
	if !strings.Contains(raw, "Subject: hello") {
		t.Fatalf("subject mangled:\n%s", raw)
	}
}

func TestSMTPNewValidation(t *testing.T) {
	if _, err := NewSMTP(config.SMTPConfig{Host: "", From: "a@b.c"}); err == nil {
		t.Fatal("want error for empty host")
	}
	if _, err := NewSMTP(config.SMTPConfig{Host: "smtp.x", From: ""}); err == nil {
		t.Fatal("want error for empty from")
	}
	s, err := NewSMTP(config.SMTPConfig{Host: "smtp.x", Port: 0, From: "a@b.c"})
	if err != nil {
		t.Fatal(err)
	}
	if s.address != "smtp.x:587" {
		t.Fatalf("got %q, want default port 587", s.address)
	}
}

func TestSMTPNewSetsAuth(t *testing.T) {
	cfg := config.SMTPConfig{Host: "smtp.x", From: "a@b.c", Username: "u", Password: "p"}
	s, err := NewSMTP(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if s.auth == nil {
		t.Fatal("want auth set when username provided")
	}
}

func TestSMTPNewNoAuth(t *testing.T) {
	s, err := NewSMTP(config.SMTPConfig{Host: "smtp.x", From: "a@b.c"})
	if err != nil {
		t.Fatal(err)
	}
	if s.auth != nil {
		t.Fatal("want nil auth when no username")
	}
}

func TestSMTPNoRecipients(t *testing.T) {
	s, err := NewSMTP(config.SMTPConfig{Host: "smtp.x", From: "a@b.c"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Send(context.Background(), Message{Subject: "s", Body: "b"}); err == nil {
		t.Fatal("want error for missing recipients")
	}
}

func TestResolveTLSMode(t *testing.T) {
	cases := []struct {
		raw  string
		port int
		want tlsMode
	}{
		{"", 587, tlsStartTLS},
		{"auto", 587, tlsStartTLS},
		{"", 465, tlsImplicit},
		{"auto", 465, tlsImplicit},
		{"starttls", 465, tlsStartTLS},
		{"implicit", 587, tlsImplicit},
		{"smtps", 587, tlsImplicit},
		{"none", 587, tlsNone},
	}
	for _, c := range cases {
		got, err := resolveTLSMode(c.raw, c.port)
		if err != nil {
			t.Fatalf("resolveTLSMode(%q, %d): %v", c.raw, c.port, err)
		}
		if got != c.want {
			t.Fatalf("resolveTLSMode(%q, %d) = %v, want %v", c.raw, c.port, got, c.want)
		}
	}
	if _, err := resolveTLSMode("bogus", 587); err == nil {
		t.Fatal("want error for unknown tls mode")
	}
}

func TestSMTPImplicitDefaultPort(t *testing.T) {
	for _, mode := range []string{"implicit", "smtps", "tls"} {
		s, err := NewSMTP(config.SMTPConfig{Host: "smtp.x", From: "a@b.c", TLS: mode})
		if err != nil {
			t.Fatal(err)
		}
		if s.address != "smtp.x:465" {
			t.Fatalf("tls=%q got %q, want implicit-TLS default port 465", mode, s.address)
		}
		if s.mode != tlsImplicit {
			t.Fatalf("tls=%q mode = %v, want implicit", mode, s.mode)
		}
	}
}

func TestSMTPAutoImplicitOn465(t *testing.T) {
	s, err := NewSMTP(config.SMTPConfig{Host: "smtp.x", Port: 465, From: "a@b.c"})
	if err != nil {
		t.Fatal(err)
	}
	if s.mode != tlsImplicit {
		t.Fatalf("mode = %v, want implicit for port 465", s.mode)
	}
}

func TestSMTPUnknownTLS(t *testing.T) {
	if _, err := NewSMTP(config.SMTPConfig{Host: "smtp.x", From: "a@b.c", TLS: "bogus"}); err == nil {
		t.Fatal("want error for unknown tls mode")
	}
}

const banner = "220 fake ESMTP"

type fakeSMTPServer struct {
	ln    net.Listener
	data  []byte
	from  string
	rcpts []string
}

func newFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeSMTPServer{ln: ln}
	go s.acceptLoop()
	t.Cleanup(func() { ln.Close() })
	return s
}

func (s *fakeSMTPServer) port() string {
	_, port, err := net.SplitHostPort(s.ln.Addr().String())
	if err != nil {
		panic(err)
	}
	return port
}

func (s *fakeSMTPServer) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeSMTPServer) handle(conn net.Conn) {
	defer conn.Close()
	tp := textproto.NewConn(conn)
	if err := tp.PrintfLine(banner); err != nil {
		return
	}
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		parts := strings.SplitN(line, " ", 2)
		switch strings.ToUpper(parts[0]) {
		case "EHLO":
			if err := tp.PrintfLine("250-fake at your service"); err != nil {
				return
			}
			if err := tp.PrintfLine("250-AUTH PLAIN"); err != nil {
				return
			}
			if err := tp.PrintfLine("250 OK"); err != nil {
				return
			}
		case "AUTH":
			if err := tp.PrintfLine("235 authenticated"); err != nil {
				return
			}
		case "MAIL":
			s.from = line
			tp.PrintfLine("250 ok")
		case "RCPT":
			s.rcpts = append(s.rcpts, line)
			tp.PrintfLine("250 ok")
		case "DATA":
			tp.PrintfLine("354 go ahead")
			data, err := tp.ReadDotBytes()
			if err != nil {
				return
			}
			s.data = data
			tp.PrintfLine("250 queued")
		case "QUIT":
			tp.PrintfLine("221 bye")
			return
		default:
			tp.PrintfLine("250 ok")
		}
	}
}

func TestSMTP_Send(t *testing.T) {
	srv := newFakeSMTPServer(t)
	cfg := config.SMTPConfig{
		Enabled:  true,
		Host:     "localhost",
		Port:     mustAtoi(t, srv.port()),
		Username: "user",
		Password: "pass",
		From:     "app@budget.local",
	}
	sender, err := NewSMTP(cfg)
	if err != nil {
		t.Fatal(err)
	}

	msg := Message{
		Subject: "relwatch: 1 new release",
		Body:    "gh - github/cli/cli: v2.0.0\nhttps://github.com/cli/cli/releases/v2.0.0\n",
		To:      []string{"me@example.com"},
	}
	if err := sender.Send(context.Background(), msg); err != nil {
		t.Fatal(err)
	}

	want := "From: app@budget.local\n" +
		"To: me@example.com\n" +
		"Subject: relwatch: 1 new release\n" +
		"MIME-Version: 1.0\n" +
		"Content-Type: text/plain; charset=utf-8\n\n" +
		"gh - github/cli/cli: v2.0.0\nhttps://github.com/cli/cli/releases/v2.0.0\n"
	if string(srv.data) != want {
		t.Fatalf("message mismatch:\n%q", string(srv.data))
	}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	var n int
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}
