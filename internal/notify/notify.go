package notify

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"relwatch/internal/config"
)

const (
	defaultPort     = 587
	implicitTLSPort = 465
)

type NewRelease struct {
	ProjectID   int64
	ProjectName string
	Provider    string
	Repo        string
	Tag         string
	Name        string
	URL         string
	PublishedAt time.Time
}

type Message struct {
	Subject string
	Body    string
	To      []string
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}

type tlsMode int

const (
	tlsStartTLS tlsMode = iota
	tlsImplicit
	tlsNone
)

type SMTP struct {
	address string
	host    string
	from    string
	auth    smtp.Auth
	mode    tlsMode
}

// resolveTLSMode maps the settings `smtp.tls` value onto a transport mode.
// Empty/"auto" picks implicit TLS on the implicit-TLS port (465) and
// opportunistic STARTTLS everywhere else. Unknown values are rejected.
func resolveTLSMode(raw string, port int) (tlsMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		if port == implicitTLSPort {
			return tlsImplicit, nil
		}
		return tlsStartTLS, nil
	case "starttls":
		return tlsStartTLS, nil
	case "implicit", "smtps", "tls":
		return tlsImplicit, nil
	case "none", "plain":
		return tlsNone, nil
	default:
		return 0, fmt.Errorf("smtp: unknown tls mode %q (want auto, starttls, implicit, or none)", raw)
	}
}

func NewSMTP(cfg config.SMTPConfig) (*SMTP, error) {
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		return nil, errors.New("smtp: host is required")
	}
	if strings.TrimSpace(cfg.From) == "" {
		return nil, errors.New("smtp: from is required")
	}
	port := cfg.Port
	if port == 0 {
		switch strings.ToLower(strings.TrimSpace(cfg.TLS)) {
		case "implicit", "smtps", "tls":
			port = implicitTLSPort
		default:
			port = defaultPort
		}
	}
	mode, err := resolveTLSMode(cfg.TLS, port)
	if err != nil {
		return nil, err
	}
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, host)
	}
	return &SMTP{
		address: net.JoinHostPort(host, strconv.Itoa(port)),
		host:    host,
		from:    cfg.From,
		auth:    auth,
		mode:    mode,
	}, nil
}

func (s *SMTP) dial(ctx context.Context) (net.Conn, error) {
	d := &net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", s.address)
	if err != nil {
		return nil, err
	}
	if s.mode != tlsImplicit {
		return conn, nil
	}
	tlsConn := tls.Client(conn, &tls.Config{ServerName: s.host})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func (s *SMTP) Send(ctx context.Context, msg Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(msg.To) == 0 {
		return errors.New("smtp: no recipients")
	}
	conn, err := s.dial(ctx)
	if err != nil {
		return fmt.Errorf("smtp: dial %s: %w", s.address, err)
	}
	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp: %w", err)
	}
	defer c.Close()

	if s.mode == tlsStartTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: s.host}); err != nil {
				return fmt.Errorf("smtp: starttls: %w", err)
			}
		}
	}
	if s.auth != nil {
		if err := c.Auth(s.auth); err != nil {
			return fmt.Errorf("smtp: auth: %w", err)
		}
	}
	if err := c.Mail(s.from); err != nil {
		return fmt.Errorf("smtp: mail from: %w", err)
	}
	for _, rcpt := range msg.To {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp: rcpt %s: %w", rcpt, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp: data: %w", err)
	}
	if _, err := w.Write(buildMessage(s.from, msg)); err != nil {
		w.Close()
		return fmt.Errorf("smtp: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	c.Quit()
	return nil
}

func digest(n int) string {
	word := "release"
	if n != 1 {
		word += "s"
	}
	return fmt.Sprintf("relwatch: %d new %s", n, word)
}

func Digest(rels []NewRelease) Message {
	var b strings.Builder
	for i, r := range rels {
		line := fmt.Sprintf("%s - %s/%s: %s", r.ProjectName, r.Provider, r.Repo, r.Tag)
		if r.Name != "" && r.Name != r.Tag {
			line += fmt.Sprintf(" (%s)", r.Name)
		}
		if !r.PublishedAt.IsZero() {
			line += fmt.Sprintf(" on %s", r.PublishedAt.Format("2006-01-02"))
		}
		b.WriteString(line)
		b.WriteString("\n")
		if r.URL != "" {
			b.WriteString(r.URL)
			b.WriteString("\n")
		}
		if i < len(rels)-1 {
			b.WriteString("\n")
		}
	}
	return Message{Subject: digest(len(rels)), Body: b.String()}
}

func sanitizeHeader(v string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(v)
}

func buildMessage(from string, msg Message) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", sanitizeHeader(from))
	fmt.Fprintf(&b, "To: %s\r\n", sanitizeHeader(strings.Join(msg.To, ", ")))
	fmt.Fprintf(&b, "Subject: %s\r\n", sanitizeHeader(msg.Subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(msg.Body)
	if !strings.HasSuffix(msg.Body, "\n") {
		b.WriteString("\n")
	}
	return []byte(b.String())
}
