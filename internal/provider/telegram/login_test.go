package telegram

import (
	"context"
	"errors"
	"testing"

	"github.com/gotd/td/tg"

	"lazygram/internal/config"
)

type fakePrompter struct{ phone, code, pass string }

func (f fakePrompter) Phone(context.Context) (string, error)    { return f.phone, nil }
func (f fakePrompter) Code(context.Context) (string, error)     { return f.code, nil }
func (f fakePrompter) Password(context.Context) (string, error) { return f.pass, nil }

func TestNormalizePhone(t *testing.T) {
	ok := map[string]string{
		"+1 555 010 0000":      "+15550100000",
		" +44 (20) 7946-0000 ": "+442079460000",
		"+1.555.010.0000":      "+15550100000",
	}
	for in, want := range ok {
		if got, err := NormalizePhone(in); err != nil || got != want {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	for _, in := range []string{"", "5550100000", "+12", "+1234567890123456", "+1 555 abc 0000", "++1555010000"} {
		if _, err := NormalizePhone(in); err == nil {
			t.Errorf("%q should fail", in)
		}
	}
}

func TestAuthenticatorAdapts(t *testing.T) {
	a := authenticator{fakePrompter{phone: "+1 555 010 0000", code: " 12345\n", pass: "pw"}}
	ctx := context.Background()
	if p, err := a.Phone(ctx); err != nil || p != "+15550100000" {
		t.Fatalf("phone %q %v", p, err)
	}
	if c, _ := a.Code(ctx, nil); c != "12345" {
		t.Fatalf("code %q", c)
	}
	if pw, _ := a.Password(ctx); pw != "pw" {
		t.Fatal("password must pass through unchanged")
	}
	if _, err := a.SignUp(ctx); !errors.Is(err, ErrSignUpUnsupported) {
		t.Fatal("sign up must be refused")
	}
	if err := a.AcceptTermsOfService(ctx, tg.HelpTermsOfService{}); !errors.Is(err, ErrSignUpUnsupported) {
		t.Fatal("tos must be refused")
	}
}

func TestLoginRejectsIncompleteConfig(t *testing.T) {
	if _, err := Login(context.Background(), config.Telegram{}, fakePrompter{}); err == nil {
		t.Fatal("want error")
	}
}
